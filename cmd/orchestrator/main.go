package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/client"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

func main() {
	store, err := NewSQL(os.Getenv("POSTGRES_CONN"))
	if err != nil {
		logrus.WithError(err).Fatal("create store")
	}

	cl := client.NewClient()

	rngsStr := os.Args[1]
	var rng ranges.Range
	if err := rng.UnmarshalText([]byte(rngsStr)); err != nil {
		logrus.WithError(err).Fatal("parse arg")
	}

	logrus.WithField("range", rngsStr).Info("starting job")

	eg, eCtx := errgroup.WithContext(context.Background())
	var mu sync.Mutex

	limiter := rate.NewLimiter(rate.Every(time.Minute/360), 120)

	for i := 0; i < 120; i++ {
		i := i
		eg.Go(func() error {
			for {
				select {
				case <-eCtx.Done():
					return nil
				default:
					mu.Lock()
					subRng, more := pop(&rng, 10_000)
					mu.Unlock()
					if !more {
						return nil
					}

					logger := logrus.WithFields(logrus.Fields{
						"range": subRng,
						"index": i,
					})

					status, err := store.Query(eCtx, subRng)
					if err != nil && !errors.Is(err, sql.ErrNoRows) {
						logger.WithError(err).Error("couldn't query store")
						continue
					} else if status == http.StatusOK {
						continue
					}
					// either ErrNoRows (no record) or the last one failed

					if err := limiter.Wait(eCtx); err != nil {
						return eCtx.Err()
					}
					logger.Info("kicking off job")
					resp, err := cl.Sync(eCtx, client.Request{
						Ranges: ranges.Ranges{subRng},
					})
					if err != nil {
						logger.WithError(err).Error("couldn't request sync")
						continue
					}

					if err := store.Log(eCtx, subRng, resp); err != nil {
						logger.WithError(err).Error("couldn't log response")
						continue
					}
				}
			}
		})
	}

	if err := eg.Wait(); err != nil {
		logrus.WithError(err).Fatal("run job")
	}
}

func pop(rng *ranges.Range, n int) (ranges.Range, bool) {
	if rng.Len() == 0 {
		return ranges.Range{}, false
	}

	return rng.Pop(n), true
}
