package spec

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	m "miwu-go/internal/miot"
)

const (
	specServerURL = "https://miot-spec.org"
	miotIconURL   = "https://home.mi.com/cgi-op/api/v1/baike/v2/product"
)

type Client struct {
	http http.Client
}

func NewClient(client http.Client) *Client {
	return &Client{http: client}
}

func (c *Client) httpGet(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("http get %q: new request: %w", rawURL, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &m.HTTPError{Status: resp.StatusCode}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20+1))
	if err != nil {
		return nil, fmt.Errorf("http get %q: read body failed: %w", rawURL, err)
	}
	if len(body) > 16<<20 {
		return nil, fmt.Errorf("%w: specification response too large", m.ErrInvalidResponse)
	}
	return body, nil
}

func specGet[T any](ctx context.Context, c *Client, path string) (*T, error) {
	body, err := c.httpGet(ctx, specServerURL+path)
	if err != nil {
		return nil, err
	}

	res := new(T)
	switch v := any(res).(type) {
	case *string:
		*v = string(body)
		return res, nil
	case *[]byte:
		*v = body
		return res, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(res); err != nil {
		return nil, fmt.Errorf("%w: specification JSON: %w", m.ErrInvalidResponse, err)
	}
	return res, nil
}
