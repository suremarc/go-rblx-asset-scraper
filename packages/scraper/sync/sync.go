package main

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	tls "github.com/refraction-networking/utls"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/assetdelivery"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/client"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/go-resty/resty/v2"
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

	eg.Go(func() error { return indexLoop(eCtx, eg, in.Ranges, items, time.Second) })
	if in.Concurrency == 0 {
		in.Concurrency = 4
	}

	logrus.WithField("request", in).Debug("got request")

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

					ctx, cancel := context.WithTimeout(eCtx, time.Second*5)

					logger := logrus.WithField("item", item)
					req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.Locations[0].Location, nil)
					if err != nil {
						cancel()
						return fmt.Errorf("error creating request: %w", err)
					}

					logger.Trace("initializing download")
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						cancel()
						logger.WithError(err).Error("failed to get asset, skipping")
						continue
					}
					logger.Trace("initialized download")

					pr, pw := io.Pipe()
					go func() {
						gz.Reset(pw)
						defer gz.Reset(nil)
						if _, err := io.Copy(gz, resp.Body); err != nil {
							logger.WithError(err).Error("couldn't stream response body")
							pw.CloseWithError(err)
							return
						}
						if err := gz.Close(); err != nil {
							logger.WithError(err).Error("couldn't close/flush gzip writer")
						}

						pw.Close()
					}()

					logger.Trace("initializing s3 upload")
					_, err = uploader.Upload(ctx, &s3.PutObjectInput{
						Bucket: aws.String(bucket),
						Key:    aws.String(item.Etag() + ".gz"),
						Body:   pr,
					})
					logger.Trace("finished s3 upload")
					if err != nil {
						cancel()
						logger.WithError(err).Error("couldn't upload to s3")
						continue
					}
					pr.Close()

					cancel()
					numSuccess.Inc()
				}
			}
		})
	}

	if err := eg.Wait(); err != nil {
		logrus.WithError(err).Debug("died with error")
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

func newClientWithOptions(proxy string) (*resty.Client, error) {
	return resty.New().
		SetRetryCount(3).
		SetTransport(&http.Transport{
			DialTLS: func(network, addr string) (net.Conn, error) {
				dialConn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}

				config := tls.Config{
					ServerName: strings.Split(addr, ":")[0],
				}

				uTLSConn := tls.UClient(dialConn, &config, tls.HelloRandomizedALPN)
				if err := uTLSConn.Handshake(); err != nil {
					return nil, err
				}
				return uTLSConn, nil
			},
		}).
		SetProxy(proxy).
		SetHeaders(map[string]string{
			"Accept-Encoding":           "gzip, deflate, br",
			"Pragma":                    "No-Cache",
			"Accept-Language":           "en-US,en;q=0.8",
			"Upgrade-Insecure-Requests": "1",
			// "User-Agent":                browser.UserAgent,
			"Accept":        "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp, image/apng,*/*;q=0.8",
			"Cache-Control": "No-Cache",
		}), nil
}

func indexLoop(eCtx context.Context, eg *errgroup.Group, rngs ranges.Ranges, items chan<- assetdelivery.AssetDescription, rt time.Duration) error {
	defer close(items)
	proxy := os.Getenv("INDEXER_PROXY")

	restyClient, err := newClientWithOptions(proxy)
	if err != nil {
		return err
	}

	client := assetdelivery.NewClient(restyClient)
	limiter := rate.NewLimiter(rate.Every(rt), 1)

	var wg sync.WaitGroup
	var count atomic.Int64
	var failedCount atomic.Int64

	for {
		rng := rngs.Pop(256)
		ids := rng.AsIntSlice()
		if len(ids) == 0 {
			break
		}
		wg.Add(1)
		count.Inc()

		if err := limiter.Wait(eCtx); err != nil {
			return eCtx.Err()
		}

		eg.Go(func() error {
			defer wg.Done()
			logrus.WithField("range", rng).Trace("making batch request")
			resp, err := client.Batch(eCtx, ids, &assetdelivery.BatchOptions{SkipSigningScripts: true})
			logrus.WithField("range", rng).Trace("got batch request")
			if err != nil {
				var rErr assetdelivery.ErrorsResponse
				if errors.As(err, &rErr) {
					if rErr.StatusCode == http.StatusForbidden || rErr.StatusCode == http.StatusTooManyRequests {
						if failedCount.Inc() > count.Load()/3 {
							return errors.New("exceeded 30% of batch requests getting 403/429's")
						}
					}
				}
				logrus.WithError(err).Error("skipping")
				return nil
			}

			for _, item := range resp.DiscardErrored().FilterByAssetType(10) {
				select {
				case <-eCtx.Done():
					return eCtx.Err()
				case items <- item:
				}
			}
			return nil
		})
	}
	logrus.Trace("initialized all batch fetches")
	wg.Wait()

	return nil
}
