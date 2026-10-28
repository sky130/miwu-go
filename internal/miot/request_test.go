package miot

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/user"
)

type transportFunc func(context.Context, string, any) ([]byte, error)

func (f transportFunc) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return f(ctx, path, body)
}

func TestCallRejectsFailedResponses(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		err  error
		code int
	}{
		{"transport failure", "", context.Canceled, 0},
		{"invalid JSON", "{", nil, 0},
		{"null response", "null", nil, 0},
		{"missing code", `{}`, nil, 0},
		{"null code", `{"code":null,"result":null}`, nil, 0},
		{"missing result", `{"code":0}`, nil, 0},
		{"different error result", `{"code":-6,"message":"invalid request","result":"error"}`, nil, -6},
		{"business failure", `{"code":-6,"message":"invalid request","result":null}`, nil, -6},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := transportFunc(func(context.Context, string, any) ([]byte, error) {
				return []byte(test.body), test.err
			})
			response, err := Call[any](t.Context(), transport, "home/profile", nil)
			if response != nil || err == nil {
				t.Fatalf("expected nil response and error, got %+v, %v", response, err)
			}
			if test.err != nil && !errors.Is(err, test.err) {
				t.Fatalf("lost underlying error: %v", err)
			}
			if test.code != 0 {
				var businessErr *model.BusinessError
				if !errors.As(err, &businessErr) || businessErr.Code != test.code || businessErr.Message != "invalid request" {
					t.Fatalf("lost business error: %v", err)
				}
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestHTTPPostPropagatesInFlightCancellation(t *testing.T) {
	started := make(chan struct{})
	client := NewClient(http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		close(started)
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-time.After(5 * time.Second):
			return nil, errors.New("request context was not canceled")
		}
	})}, user.Credentials{Session: user.Session{ServiceSecurity: data}}, MiotBaseURL)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := client.Post(ctx, "home/profile", map[string]string{"id": "123"})
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected cancellation, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("request did not stop after cancellation")
	}
}
