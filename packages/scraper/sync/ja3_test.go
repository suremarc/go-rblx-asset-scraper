//go:build integration_test

package main

import (
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/assetdelivery"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"
	"golang.org/x/sync/errgroup"
)

func TestJa3(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	items := make(chan assetdelivery.AssetDescription, 1_000_000)
	rng, err := ranges.NewRange(100_000, 1_000_000)
	require.NoError(t, err)

	rngs := ranges.Ranges{rng}
	eg, eCtx := errgroup.WithContext(context.TODO())
	eg.Go(func() error { return indexLoop(eCtx, eg, rngs, items, time.Second/256) })
	require.NoError(t, eg.Wait())
}
