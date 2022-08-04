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

func init() {
	key = os.Getenv("SPACES_KEY")
	if key == "" {
		panic("no key provided")
	}
	secret = os.Getenv("SPACES_SECRET")
	if secret == "" {
		panic("no secret provided")
	}
	bucket = os.Getenv("SPACES_BUCKET")
	if bucket == "" {
		panic("no bucket provided")
	}
	region = os.Getenv("SPACES_REGION")
	if region == "" {
		panic("no region provided")
	}
}

type Request struct {
	Groups      string `json:"groups"`
	Concurrency int    `json:"concurrency,omitempty"`
}

type Response struct {
	StatusCode int               `json:"statusCode,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
}

func Main(in Request) (*Response, error) {
	items := make(chan assetdelivery.AssetDescription, 10_000)
	eg, eCtx := errgroup.WithContext(context.Background())

	var grps groups
	if err := grps.parse(in.Groups); err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(key, secret, ""),
		EndpointResolver: aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL: fmt.Sprintf("https://%s.digitaloceanspaces.com:443", region),
			}, nil
		}),
		Region: region,
	})

	uploader := manager.NewUploader(client)

	eg.Go(func() error { return indexLoop(eCtx, grps, items) })
	if in.Concurrency == 0 {
		in.Concurrency = 8
	}

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

					logger := logrus.WithField("item", item)
					req, err := http.NewRequestWithContext(eCtx, http.MethodGet, item.Locations[0].Location, nil)
					if err != nil {
						return fmt.Errorf("error creating request: %w", err)
					}

					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						logger.WithError(err).Error("failed to get batch, skipping")
						continue
					}

					pr, pw := io.Pipe()
					defer pr.Close()
					go func() {
						gz.Reset(pw)
						if _, err := io.Copy(gz, resp.Body); err != nil {
							logger.WithError(err).Error("couldn't stream response body: %w", err)
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
						return err
					}
				}

			}
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return &Response{
		StatusCode: http.StatusOK,
	}, nil
}

func indexLoop(ctx context.Context, grps groups, items chan<- assetdelivery.AssetDescription) error {
	defer close(items)

	eg, eCtx := errgroup.WithContext(ctx)

	client := assetdelivery.NewClient()
	limiter := rate.NewLimiter(rate.Every(time.Second/100), 100)

	for {
		ids := grps.pop(256).asIntSlice()
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
