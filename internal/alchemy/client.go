package alchemy

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	apiKey  string
	network string
	http    *http.Client
}

type ownerNFTsResponse struct {
	PageKey string `json:"pageKey"`
}

func NewClient(apiKey, network string) *Client {
	return &Client{
		apiKey:  apiKey,
		network: network,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) FetchOwnerNFTsRaw(ctx context.Context, owner string, pageSize int, page int) ([]byte, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	var pageKey string
	var raw []byte
	for i := 1; i <= page; i++ {
		respBody, nextPageKey, err := c.fetchOwnerNFTsPage(ctx, owner, pageSize, pageKey)
		if err != nil {
			return nil, err
		}
		raw = respBody
		if i == page {
			return raw, nil
		}
		if nextPageKey == "" {
			return raw, nil
		}
		pageKey = nextPageKey
	}
	return raw, nil
}

func (c *Client) fetchOwnerNFTsPage(ctx context.Context, owner string, pageSize int, pageKey string) ([]byte, string, error) {
	base := fmt.Sprintf("https://%s.g.alchemy.com/nft/v3/%s/getNFTsForOwner", c.network, c.apiKey)
	query := url.Values{}
	query.Set("owner", strings.ToLower(owner))
	query.Set("pageSize", strconv.Itoa(pageSize))
	if pageKey != "" {
		query.Set("pageKey", pageKey)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+query.Encode(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("create alchemy request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("call alchemy: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("read alchemy response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("alchemy status %d: %s", resp.StatusCode, string(body))
	}

	var parsed ownerNFTsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, "", fmt.Errorf("decode alchemy page key: %w", err)
	}

	return body, parsed.PageKey, nil
}
