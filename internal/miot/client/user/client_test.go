package user

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"miwu-go/internal/miot/model/user"
)

type transportFunc func(context.Context, string, any) ([]byte, error)

func (f transportFunc) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return f(ctx, path, body)
}

func TestUserClientRequestAndTokenValidation(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		err      error
		valid    bool
	}{
		{"valid", `{"code":0,"result":{"userid":2391366608,"nickname":"name","icon":"avatar"}}`, nil, true},
		{"business failure", `{"code":-6,"message":"invalid request","result":null}`, nil, false},
		{"missing profile", `{"code":0,"result":null}`, nil, false},
		{"canceled", "", context.Canceled, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
				if gotCtx != ctx || path != "home/profile" {
					t.Fatalf("unexpected context or path: %s", path)
				}
				encoded, err := json.Marshal(body)
				if err != nil || string(encoded) != `{"id":"2391366608"}` {
					t.Fatalf("unexpected profile request: %s, %v", encoded, err)
				}
				return []byte(test.response), test.err
			}), user.Credentials{Identity: user.Identity{UserID: "2391366608"}})
			err := client.CheckSession(ctx)
			valid := err == nil
			if valid != test.valid || (err == nil) != test.valid {
				t.Fatalf("valid=%v, error=%v", valid, err)
			}
			if test.err != nil && !errors.Is(err, test.err) {
				t.Fatalf("lost underlying error: %v", err)
			}
		})
	}
}
