package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/suremarc/go-rblx-asset-scraper/packages/scraper/sync/ranges"
)

type Request struct {
	Ranges      ranges.Ranges `json:"ranges"`
	Concurrency int           `json:"concurrency,omitempty"`
}

type Response struct {
	StatusCode           int `json:"status_code"`
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

	cmd := exec.Command("doctl", strings.Split("serverless fn invoke scraper/sync", " ")...)
	cmd.Args = append(cmd.Args, "-p", fmt.Sprintf("ranges:%s", rngsText))
	if req.Concurrency > 0 {
		cmd.Args = append(cmd.Args, "-p", fmt.Sprintf("concurrency:%d", req.Concurrency))
	}

	logrus.WithField("cmd", cmd.String()).Trace("sending cmd")

	out, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("%w\n%s", err, fmt.Errorf(string(exitError.Stderr)))
		}

		return nil, err
	}
	fmt.Println(string(out))

	var resp Response
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
