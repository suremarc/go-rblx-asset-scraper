package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"
)

type Request struct {
	Ranges      ranges.Ranges `json:"ranges"`
	Concurrency int           `json:"concurrency,omitempty"`
}

type Response struct {
	StatusCode           int `json:"status_code,omitempty"`
	Successes            int `json:"successes"`
	Failures             int `json:"failures"`
	Total                int `json:"total"`
	DurationMilliseconds int `json:"duration_ms"`
}

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) Sync(ctx context.Context, req Request) (*Response, error) {
	rngsText, err := req.Ranges.MarshalText()
	if err != nil {
		return nil, fmt.Errorf("couldn't send sync request: %w", err)
	}

	cmd := exec.Command("doctl", "serverless fn invoke scraper/sync",
		fmt.Sprintf("-p ranges:%s", rngsText))
	if req.Concurrency > 0 {
		cmd.Args = append(cmd.Args, fmt.Sprintf("-p concurrency:%d", req.Concurrency))
	}

	fmt.Println(cmd.String())

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var resp Response
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
