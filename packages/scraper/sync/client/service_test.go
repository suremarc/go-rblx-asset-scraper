//go:build service_test

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	logrus.SetLevel(logrus.TraceLevel)

	c := NewClient()

	rng, err := ranges.NewRange(101_000, 101_001)
	require.NoError(t, err)

	resp, err := c.Sync(context.TODO(), Request{
		Ranges:      ranges.Ranges{rng},
		Concurrency: 8,
	})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Greater(t, resp.Successes, 290) // Ideally there should be 300-305 successes.

	buf, err := json.MarshalIndent(resp, "\t", "")
	require.NoError(t, err)
	t.Log(string(buf))
}
