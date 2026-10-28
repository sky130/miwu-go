package miot

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidResponse = errors.New("invalid API response")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrAuthRequired    = errors.New("session unavailable; run account refresh or login")
)

// HTTPError deliberately excludes response bodies, which may contain credentials.
type HTTPError struct {
	Status int
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP status %d", e.Status)
}

func (e *HTTPError) Is(target error) bool {
	return target == ErrAuthRequired && e.Status == 401
}
