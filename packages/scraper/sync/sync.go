package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/assetdelivery"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/client"
	"go.uber.org/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

var (
	key, secret, bucket, region string
)

func Main(in client.Request) (*client.Response, error) {
	l, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err != nil {
		logrus.WithField("LOG_LEVEL", os.Getenv("LOG_LEVEL")).Warn("invalid or missing log level")
	} else {
		logrus.SetLevel(l)
	}

	if in.Concurrency == 0 {
		in.Concurrency = 4
	}

	items := make(chan assetdelivery.AssetDescription, 10_000)
	eg, eCtx := errgroup.WithContext(context.Background())

	s3Client := s3.NewFromConfig(aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""),
		EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: fmt.Sprintf("https://%s.digitaloceanspaces.com:443", region),
			}, nil
		}),
		Region: region,
	})

	uploader := manager.NewUploader(s3Client)

	eg.Go(func() error { return indexLoop(eCtx, in, items) })

	logrus.WithField("request", in).Trace("got request")

	var numItems atomic.Int64
	var numSuccess atomic.Int64
	t0 := time.Now()

	for i := 0; i < in.Concurrency; i++ {
		eg.Go(func() error {
			gz := gzip.NewWriter(nil)
			for {
				select {
				case <-eCtx.Done():
					return eCtx.Err()
				case item, ok := <-items:
					if !ok {
						return nil
					}
					numItems.Inc()

					logger := logrus.WithField("item", item)
					req, err := http.NewRequestWithContext(eCtx, http.MethodGet, item.Locations[0].Location, nil)
					if err != nil {
						return fmt.Errorf("error creating request: %w", err)
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						logger.WithError(err).Error("failed to get asset, skipping")
						continue
					}

					pr, pw := io.Pipe()
					defer pr.Close()
					go func() {
						gz.Reset(pw)
						if _, err := io.Copy(gz, resp.Body); err != nil {
							logger.WithError(err).Error("couldn't stream response body")
							pw.CloseWithError(err)
						}
						gz.Close()
						pw.Close()
					}()

					_, err = uploader.Upload(eCtx, &s3.PutObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(item.Etag() + ".gz"),
						Body:   pr,
					})
					if err != nil {
						logger.WithError(err).Error("couldn't upload to s3")
						continue
					}

					numSuccess.Inc()
				}

			}
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &client.Response{
		StatusCode:           http.StatusOK,
		Successes:            int(numSuccess.Load()),
		Failures:             int(numItems.Load() - numSuccess.Load()),
		Total:                int(numItems.Load()),
		DurationMilliseconds: int(time.Since(t0).Milliseconds()),
	}, nil
}

func indexLoop(ctx context.Context, in client.Request, items chan<- assetdelivery.AssetDescription) error {
	defer close(items)

	client := assetdelivery.NewClient()
	limiter := rate.NewLimiter(rate.Every(time.Second/100), 100)

	const maxBatchSize = 256
	numChunks := (in.Ranges.Len()-1)/maxBatchSize + 1
	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(int(numChunks))

	for i := 0; i < in.Concurrency; i++ {
		go func() {
			defer wg.Done()

			if err := limiter.Wait(ctx); err != nil {
				return
			}

			mu.Lock()
			ids := in.Ranges.Pop(maxBatchSize).AsIntSlice()
			mu.Unlock()

			if len(ids) == 0 {
				return
			}

			resp, err := client.Batch(ctx, ids, &assetdelivery.BatchOptions{SkipSigningScripts: true})
			if err != nil {
				logrus.WithError(err).Error("skipping")
				return
			}

			for _, item := range resp.DiscardErrored().FilterByAssetType(10) {
				select {
				case <-ctx.Done():
					return
				case items <- item:
				}
			}
		}()
	}

	wg.Wait()

	return nil
}
