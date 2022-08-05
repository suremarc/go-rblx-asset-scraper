package client

import (
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"
)

type Request struct {
	Ranges      ranges.Ranges `json:"groups"`
	Concurrency int           `json:"concurrency,omitempty"`
}

type Response struct {
	StatusCode           int `json:"statusCode,omitempty"`
	Successes            int `json:"successes"`
	Failures             int `json:"failures"`
	Total                int `json:"total"`
	DurationMilliseconds int `json:"duration_ms"`
}

type Client struct{}

// func (c *Client) Sync(req Request) (*Response, error) {
// 	cmd := exec.Command("doctl serverless fn invoke scraper/sync")
// 	ranges, err := req.Ranges.MarshalText()
// }
