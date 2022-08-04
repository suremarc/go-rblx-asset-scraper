package main

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/sirupsen/logrus"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/assetdelivery"
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
	Groups  string `json:"groups"`
	Workers int    `json:"workers,omitempty"`
}

type Response struct {
	StatusCode int               `json:"statusCode,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
}

type assetWithRetries struct {
	assetdelivery.AssetDescription
	retries int
}

func Main(in Request) (*Response, error) {
	items := make(chan assetWithRetries, 10_000)
	eg, eCtx := errgroup.WithContext(context.Background())

	var grps groups
	if err := grps.parse(in.Groups); err != nil {
		return nil, err
	}

	config := &aws.Config{
		Credentials: credentials.NewStaticCredentials(key, secret, ""),
		Endpoint:    aws.String(fmt.Sprintf("%s.digitaloceanspaces.com:443", region)),
		Region:      aws.String(region),
	}
	sess := session.New(config)
	s3Client := s3.New(sess)

	eg.Go(func() error { return indexLoop(eCtx, grps, items) })
	if in.Workers == 0 {
		in.Workers = 16
	}

	for i := 0; i < in.Workers; i++ {
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
					req, err := http.NewRequest(http.MethodGet, item.Locations[0].Location, nil)
					if err != nil {
						return fmt.Errorf("error creating request: %w", err)
					}

					req = req.WithContext(eCtx)
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						handleRetryFailure(eCtx, logger, items, item, err)
						continue
					}

					pr, pw := io.Pipe()
					go func() {
						gz.Reset(pw)
						if _, err := io.Copy(gz, resp.Body); err != nil {
							pw.CloseWithError(err)
						}
						gz.Flush()
						pw.Close()
					}()

					s3Req, _ := s3Client.PutObjectRequest(&s3.PutObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(item.Etag() + ".gz"),
					})
					s3Req.SetStreamingBody(pr)
					s3Req.SetContext(eCtx)

					if err := s3Req.Send(); err != nil {
						handleRetryFailure(eCtx, logger, items, item, err)
						continue
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

func handleRetryFailure(ctx context.Context, logger logrus.FieldLogger, items chan<- assetWithRetries, item assetWithRetries, err error) {
	if item.retries > 3 {
		logger.WithError(err).Error("retry limit exceeded, skipping")
	} else {
		item.retries++
		// do this to avoid deadlocking
		select {
		case items <- item:
		default:
			logger.WithError(err).Error("input queue is full, dropping")
		}
	}

	return
}

func indexLoop(ctx context.Context, grps groups, items chan<- assetWithRetries) error {
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
				case items <- assetWithRetries{AssetDescription: item}:
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
