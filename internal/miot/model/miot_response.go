package model

import "fmt"

// BusinessError reports a MIoT response or individual operation failure.
type BusinessError struct {
	Code    int
	Message string
}

func (e *BusinessError) Error() string {
	return fmt.Sprintf("MIoT error %d: %s", e.Code, e.Message)
}

type MiotResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  T      `json:"result"`
}

// BatchError preserves every failure while callers retain successful results.
type BatchError struct {
	Failures []error
}

func (e *BatchError) Error() string {
	return fmt.Sprintf("%d operation(s) failed", len(e.Failures))
}

func (e *BatchError) Unwrap() []error {
	return e.Failures
}
