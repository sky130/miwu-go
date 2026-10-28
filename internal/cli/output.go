package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"miwu-go/internal/app"
	"miwu-go/internal/miot"
	"miwu-go/internal/miot/auth"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/userstore"
)

type result struct {
	Data    any
	Columns []string
	Rows    [][]string
	Note    string
}

type errorInfo struct {
	Kind         string `json:"kind"`
	Message      string `json:"message"`
	BusinessCode *int   `json:"business_code,omitempty"`
}

type envelope struct {
	Data  any        `json:"data"`
	Error *errorInfo `json:"error"`
}

type reportedError struct {
	error
}

func (e *reportedError) Unwrap() error {
	return e.error
}

func describe(err error) *errorInfo {
	if err == nil {
		return nil
	}
	info := &errorInfo{Kind: "operation", Message: err.Error()}
	var batch *model.BatchError
	var business *model.BusinessError
	var argument *usageError
	var httpErr *miot.HTTPError
	var outcome *app.OutcomeUnknownError
	switch {
	case errors.Is(err, context.Canceled) || errors.Is(err, io.EOF):
		info.Kind = "canceled"
		if !errors.As(err, &outcome) {
			info.Message = "operation canceled"
		}
	case errors.As(err, &outcome):
		info.Kind = "outcome_unknown"
	case errors.Is(err, context.DeadlineExceeded):
		info.Kind = "timeout"
	case errors.As(err, &argument) || errors.Is(err, miot.ErrInvalidArgument):
		info.Kind = "argument"
	case errors.As(err, &batch):
		info.Kind = "partial_failure"
	case errors.Is(err, auth.ErrChallengeRequired):
		info.Kind = "verification_required"
	case errors.Is(err, auth.ErrInvalidCredentials) || errors.Is(err, miot.ErrAuthRequired):
		info.Kind = "authentication"
	case errors.Is(err, auth.ErrQRCodeExpired):
		info.Kind = "qr_expired"
	case errors.Is(err, userstore.ErrNotFound):
		info.Kind = "not_found"
	case errors.Is(err, userstore.ErrCorrupt):
		info.Kind = "corrupt_store"
	case errors.Is(err, userstore.ErrUnsupportedVersion):
		info.Kind = "unsupported_version"
	case errors.Is(err, miot.ErrInvalidResponse):
		info.Kind = "protocol"
	case errors.As(err, &business):
		info.Kind = "business"
	case errors.As(err, &httpErr):
		info.Kind = "http"
	}
	if errors.As(err, &business) {
		info.BusinessCode = &business.Code
	}
	return info
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	kind := describe(err).Kind
	switch kind {
	case "argument":
		return 2
	case "partial_failure":
		return 3
	case "canceled":
		return 130
	default:
		return 1
	}
}

func (s *state) emit(value result, err error) error {
	if s.settings.Output == "json" {
		return json.NewEncoder(s.options.Out).Encode(envelope{Data: value.Data, Error: describe(err)})
	}
	if len(value.Columns) > 0 {
		writer := tabwriter.NewWriter(s.options.Out, 0, 4, 2, ' ', 0)
		if _, e := fmt.Fprintln(writer, joinColumns(value.Columns)); e != nil {
			return e
		}
		for _, row := range value.Rows {
			if _, e := fmt.Fprintln(writer, joinColumns(row)); e != nil {
				return e
			}
		}
		if e := writer.Flush(); e != nil {
			return e
		}
		if len(value.Rows) == 0 {
			fmt.Fprintln(s.options.Out, "No items.")
		}
	} else if value.Data != nil {
		encoder := json.NewEncoder(s.options.Out)
		encoder.SetIndent("", "  ")
		if e := encoder.Encode(value.Data); e != nil {
			return e
		}
	}
	if value.Note != "" {
		fmt.Fprintln(s.options.Err, value.Note)
	}
	if err != nil {
		fmt.Fprintln(s.options.Err, joinColumns([]string{describe(err).Message}))
	}
	return nil
}

func joinColumns(values []string) string {
	// Remote names cannot inject extra rows, columns, or terminal escape sequences.
	out := ""
	for i, v := range values {
		if i > 0 {
			out += "\t"
		}
		for _, r := range v {
			if r < 32 || r == 127 {
				out += " "
			} else {
				out += string(r)
			}
		}
	}
	return out
}

func (s *state) report(err error) {
	var reported *reportedError
	if errors.As(err, &reported) {
		return
	}
	if s.settings.Output == "json" {
		_ = s.emit(result{}, err)
		return
	}
	fmt.Fprintln(s.options.Err, joinColumns([]string{describe(err).Message}))
}
