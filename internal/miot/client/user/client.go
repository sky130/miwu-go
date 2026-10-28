package user

import (
	"context"
	"fmt"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/model/user"
)

type Client struct {
	transport m.Transport
	userID    string
}

func NewClient(transport m.Transport, credentials user.Credentials) *Client {
	return &Client{transport: transport, userID: credentials.UserID}
}

func (c *Client) GetInfo(ctx context.Context) (*user.Info, error) {
	if c.userID == "" {
		return nil, fmt.Errorf("%w: user ID required", m.ErrInvalidArgument)
	}
	response, err := m.Call[*user.Info](ctx, c.transport, "home/profile", struct {
		ID string `json:"id"`
	}{c.userID})
	if err != nil {
		return nil, err
	}
	if response.Result == nil || response.Result.UserID <= 0 {
		return nil, fmt.Errorf("%w: missing profile", m.ErrInvalidResponse)
	}
	return response.Result, nil
}

func (c *Client) CheckSession(ctx context.Context) error {
	_, err := c.GetInfo(ctx)
	return err
}
