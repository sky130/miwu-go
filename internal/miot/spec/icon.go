package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
)

type deviceInfo struct {
	Code *int     `json:"code"`
	Data iconData `json:"data"`
}

type iconData struct {
	RealIcon string `json:"realIcon"`
}

func (c *Client) IconURL(ctx context.Context, deviceModel string) (*string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(deviceModel) == "" {
		return nil, fmt.Errorf("%w: device model required", m.ErrInvalidArgument)
	}
	q := url.Values{}
	q.Set("model", deviceModel)

	body, err := c.httpGet(ctx, miotIconURL+"?"+q.Encode())
	if err != nil {
		return nil, err
	}

	info := new(deviceInfo)
	if err := json.Unmarshal(body, info); err != nil {
		return nil, fmt.Errorf("%w: icon JSON: %w", m.ErrInvalidResponse, err)
	}
	if info.Code == nil {
		return nil, fmt.Errorf("%w: missing icon code", m.ErrInvalidResponse)
	}
	if *info.Code != 0 {
		return nil, &model.BusinessError{Code: *info.Code, Message: "icon request rejected"}
	}
	if info.Data.RealIcon == "" {
		return nil, fmt.Errorf("%w: empty icon URL", m.ErrInvalidResponse)
	}

	return &info.Data.RealIcon, nil
}
