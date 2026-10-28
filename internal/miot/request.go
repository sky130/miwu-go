package miot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"miwu-go/internal/miot/model"
)

// Transport sends a complete request body using the caller's context.
type Transport interface {
	Post(ctx context.Context, path string, body any) ([]byte, error)
}

// Call decodes the response envelope and checks its top-level business code.
// Each endpoint is responsible for checking individual results.
func Call[T any](ctx context.Context, client Transport, path string, body any) (*model.MiotResponse[T], error) {
	data, err := client.Post(ctx, path, body)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	var envelope struct {
		Code    *int            `json:"code"`
		Message string          `json:"message"`
		Result  json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("%s: %w: %w", path, ErrInvalidResponse, err)
	}
	if envelope.Code == nil {
		return nil, fmt.Errorf("%s: %w: missing code", path, ErrInvalidResponse)
	}
	if *envelope.Code != 0 {
		return nil, fmt.Errorf("%s: %w", path, &model.BusinessError{Code: *envelope.Code, Message: envelope.Message})
	}
	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("%s: %w: missing result", path, ErrInvalidResponse)
	}
	response := &model.MiotResponse[T]{Code: 0, Message: envelope.Message}
	decoder := json.NewDecoder(bytes.NewReader(envelope.Result))
	decoder.UseNumber()
	if err := decoder.Decode(&response.Result); err != nil {
		return nil, fmt.Errorf("%s: %w: decode result: %w", path, ErrInvalidResponse, err)
	}
	return response, nil
}
