package app

import (
	"errors"

	"miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
)

// OutcomeUnknownError means a write could have reached the server. Both UIs
// must preserve this distinction instead of suggesting an automatic retry.
type OutcomeUnknownError struct {
	Err error
}

func (e *OutcomeUnknownError) Error() string {
	return "remote outcome is unknown; operation was not retried: " + e.Err.Error()
}

func (e *OutcomeUnknownError) Unwrap() error {
	return e.Err
}

func writeFailure(err error) error {
	if err == nil {
		return nil
	}
	var business *model.BusinessError
	var batch *model.BatchError
	if errors.As(err, &batch) || errors.As(err, &business) || errors.Is(err, miot.ErrInvalidArgument) || errors.Is(err, miot.ErrAuthRequired) {
		return err
	}
	return &OutcomeUnknownError{Err: err}
}
