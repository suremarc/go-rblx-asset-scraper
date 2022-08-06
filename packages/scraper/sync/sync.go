package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/assetdelivery"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/client"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
	"go.uber.org/atomic"
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

	items := make(chan assetdelivery.AssetDescription, 10_000)
	eg, eCtx := errgroup.WithContext(context.Background())

	s3Client := s3.NewFromConfig(aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""),
		EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: fmt.Sprintf("https://s3.%s.wasabisys.com", region),
			}, nil
		}),
		Region: region,
	})

	uploader := manager.NewUploader(s3Client)

	eg.Go(func() error { return indexLoop(eCtx, in.Ranges, items) })
	if in.Concurrency == 0 {
		in.Concurrency = 4
	}

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

func indexLoop(ctx context.Context, rngs ranges.Ranges, items chan<- assetdelivery.AssetDescription) error {
	defer close(items)

	eg, eCtx := errgroup.WithContext(ctx)

	client := assetdelivery.NewClient()
	limiter := rate.NewLimiter(rate.Every(time.Second/100), 100)

	for {
		ids := rngs.Pop(256).AsIntSlice()
		if len(ids) == 0 {
			break
		}

		if err := limiter.Wait(eCtx); err != nil {
			return eCtx.Err()
		}

		eg.Go(func() error {
			resp, err := client.Batch(eCtx, ids, &assetdelivery.BatchOptions{SkipSigningScripts: true})
			if err != nil {
				logrus.WithError(err).Error("skipping")
				return nil
			}

			for _, item := range resp.DiscardErrored().FilterByAssetType(10) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case items <- item:
				}
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}
