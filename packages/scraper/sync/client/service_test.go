//go:build service_test

package client

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	c := NewClient()

	rng, err := ranges.NewRange(100_000, 101_000)
	require.NoError(t, err)

	resp, err := c.Sync(context.TODO(), Request{Ranges: ranges.Ranges{rng}})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	buf, err := json.MarshalIndent(resp, "\t", "")
	require.NoError(t, err)
	t.Log(string(buf))
}
