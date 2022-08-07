package assetdelivery

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

type Client struct {
	client *resty.Client
}

func NewClient(r *resty.Client) *Client {
	return &Client{
		client: r,
	}
}

type BatchOptions struct {
	SkipSigningScripts bool
}

func (c *Client) Batch(ctx context.Context, ids []int64, opts *BatchOptions) (descriptions AssetDescriptions, err error) {
	items := AssetRequestItemsFromAssetIDs(ids...)

	resp, err := c.client.
		NewRequest().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"skipSigningScripts": fmt.Sprint(opts.SkipSigningScripts),
		}).
		SetBody(AssetRequestItems(items)).
		AddRetryCondition(func(resp *resty.Response, err error) bool {
			return strings.Contains(resp.String(), "TooManyRequests") ||
				strings.Contains(resp.String(), "504 Gateway Time-out")
		}).
		Post("https://assetdelivery.roblox.com/v2/assets/batch")
	if err != nil {
		return nil, fmt.Errorf("err executing request: %w", err)
	}

	const byteOrderMarkAsString = string('\uFEFF')
	body := bytes.TrimPrefix(resp.Body(), []byte(byteOrderMarkAsString))

	err = descriptions.UnmarshalJSON(body)
	if err != nil {
		// try unmarshal the errors
		var errors ErrorsResponse
		if err2 := errors.UnmarshalJSON(body); err2 != nil {
			fmt.Println(string(body))
			// just return the original error
			return nil, fmt.Errorf("error unmarshaling response body: %w", err)
		}

		errors.StatusCode = resp.StatusCode()
		return nil, fmt.Errorf("error from server: %w", errors)
	}

	// link the items by request id
	for i := range descriptions {
		descriptions[i].AssetID = items[i].AssetID
	}

	return descriptions, nil
}

func (c *Client) AssetFetchByID(ctx context.Context, id uint64, opts *BatchOptions) (description AssetDescription, err error) {
	resp, err := c.client.NewRequest().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetQueryParams(map[string]string{
			"skipSigningScripts": fmt.Sprint(opts.SkipSigningScripts),
		}).
		Get(fmt.Sprintf("https://assetdelivery.roblox.com/v2/assetId/%d", id))
	if err != nil {
		return AssetDescription{}, fmt.Errorf("err executing request: %w", err)
	}

	if err := description.UnmarshalJSON(resp.Body()); err != nil {
		return AssetDescription{}, nil
	}

	return description, nil
}
