package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
	dst, err := os.CreateTemp("", "scraper_sync")
	if err != nil {
		return nil, fmt.Errorf("create payload file: %w", err)
	}
	defer os.Remove(dst.Name())

	buf, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if _, err = dst.Write(buf); err != nil {
		return nil, err
	}

	cmd := exec.Command("doctl",
		strings.Split(fmt.Sprintf("serverless fn invoke scraper/sync -P %s", dst.Name()), " ")...)

	logrus.WithFields(logrus.Fields{
		"cmd":  cmd.String(),
		"body": string(buf),
	}).Trace("sending cmd")

	out, err := cmd.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			return nil, fmt.Errorf("%w\n%s", err, fmt.Errorf(string(exitError.Stderr)))
		}

		return nil, err
	}
	logrus.Trace(string(out))

	var resp Response
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
