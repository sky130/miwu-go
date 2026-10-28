package miot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"miwu-go/internal/miot/crypto"
	"miwu-go/internal/miot/model/user"
)

const MiotBaseURL = "https://api.io.mi.com/app/"

const miotUserAgent = "APP/com.xiaomi.mihome APPV/6.0.103 iosPassportSDK/3.9.0 iOS/14.4 miHSTS"

// Client owns a credential snapshot. Create a new client after refreshing it.
type Client struct {
	client      http.Client
	baseURL     string
	credentials user.Credentials
}

func NewClient(client http.Client, credentials user.Credentials, baseURL string) *Client {
	// Authenticated POSTs must not be silently replayed on redirects.
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{client: client, baseURL: strings.TrimRight(baseURL, "/") + "/", credentials: credentials}
}

func Decode[T any](data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

func (c *Client) Post(ctx context.Context, path string, body any) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	// Sign the endpoint relative to the configured API base, not a hard-coded host.
	signedPath := "/" + strings.TrimLeft(path, "/")
	nonce, err := crypto.GetNonce()
	if err != nil {
		return nil, err
	}
	sn, err := crypto.GenerateSignedNonce(c.credentials.ServiceSecurity, nonce)
	if err != nil {
		return nil, fmt.Errorf("sign nonce: %w", err)
	}
	signature, err := crypto.GenerateSignature(signedPath, sn, nonce, string(raw))
	if err != nil {
		return nil, err
	}
	encoded := url.Values{"_nonce": {nonce}, "data": {string(raw)}, "signature": {signature}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+strings.TrimLeft(path, "/"), strings.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("new API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", miotUserAgent)
	req.Header.Set("x-xiaomi-protocal-flag-cli", "PROTOCAL-HTTP2")
	for _, cookie := range []*http.Cookie{{Name: "PassportDeviceId", Value: c.credentials.DeviceID}, {Name: "userId", Value: c.credentials.UserID}, {Name: "serviceToken", Value: c.credentials.ServiceToken}} {
		req.AddCookie(cookie)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send API request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{Status: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20+1))
	if err != nil {
		return nil, fmt.Errorf("read API response: %w", err)
	}
	if len(b) > 16<<20 {
		return nil, fmt.Errorf("%w: response too large", ErrInvalidResponse)
	}
	return b, nil
}
