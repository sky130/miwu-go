package device

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/attr"
)

type transportFunc func(context.Context, string, any) ([]byte, error)

func (f transportFunc) Post(ctx context.Context, path string, body any) ([]byte, error) {
	return f(ctx, path, body)
}

func checkRequest(t *testing.T, body any, expected string) {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request = %s, want %s", encoded, expected)
	}
}

func TestGetPropertiesPreservesPartialResults(t *testing.T) {
	ctx := t.Context()
	client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
		if gotCtx != ctx || path != "miotspec/prop/get" {
			t.Fatalf("unexpected context or path: %s", path)
		}
		checkRequest(t, body, `{"params":[{"did":"123","siid":2,"piid":1},{"did":"123","siid":2,"piid":2}]}`)
		return []byte(`{"code":0,"result":[
			{"did":"123","siid":2,"piid":1,"code":0,"value":true},
			{"did":"123","siid":2,"piid":2,"code":-704042011}
		]}`), nil
	}))
	response, err := client.GetProperties(ctx, "123", attr.PropertyRef{ServiceIID: 2, PropertyIID: 1}, attr.PropertyRef{ServiceIID: 2, PropertyIID: 2})
	var batchErr *model.BatchError
	if !errors.As(err, &batchErr) {
		t.Fatalf("expected partial failure: %v", err)
	}
	if len(response) != 2 || response[0].Value != true || response[1].Code != -704042011 {
		t.Fatalf("unexpected results: %+v", response)
	}
}

func TestSetPropertiesChecksAllResults(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		failures int
	}{
		{"success", `{"code":0,"result":[{"did":"123","siid":2,"piid":1,"code":0},{"did":"123","siid":2,"piid":2,"code":0},{"did":"123","siid":2,"piid":3,"code":0}]}`, 0},
		{"partial failure", `{"code":0,"result":[
			{"did":"123","siid":2,"piid":1,"code":0},
			{"did":"123","siid":2,"piid":2,"code":-1},
			{"did":"123","siid":2,"piid":3,"code":-2}
		]}`, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
				if gotCtx != ctx || path != "miotspec/prop/set" {
					t.Fatalf("unexpected context or path: %s", path)
				}
				checkRequest(t, body, `{"params":[{"did":"123","siid":2,"piid":1,"value":false},
					{"did":"123","siid":2,"piid":2,"value":0},{"did":"123","siid":2,"piid":3,"value":""}]}`)
				return []byte(test.response), nil
			}))
			response, err := client.SetProperties(ctx, "123",
				attr.PropertyValue{ServiceIID: 2, PropertyIID: 1, Value: false},
				attr.PropertyValue{ServiceIID: 2, PropertyIID: 2, Value: 0},
				attr.PropertyValue{ServiceIID: 2, PropertyIID: 3, Value: ""})
			if response == nil || response[0].Code != 0 {
				t.Fatalf("successful result was lost: %+v, %v", response, err)
			}
			if test.failures == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var businessErr *model.BusinessError
			if !errors.As(err, &businessErr) || businessErr.Code != -1 {
				t.Fatalf("expected typed property error, got %v", err)
			}
			joined, ok := err.(interface{ Unwrap() []error })
			if !ok || len(joined.Unwrap()) != test.failures {
				t.Fatalf("expected %d failures, got %v", test.failures, err)
			}
		})
	}
}

func TestExecuteActionUsesObjectParams(t *testing.T) {
	for _, test := range []struct {
		name  string
		input []any
		json  string
	}{
		{"no input", nil, `{"params":{"did":"123","siid":2,"aiid":1,"in":[]}}`},
		{"with input", []any{false, 42}, `{"params":{"did":"123","siid":2,"aiid":1,"in":[false,42]}}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx := t.Context()
			client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
				if gotCtx != ctx || path != "miotspec/action" {
					t.Fatalf("unexpected context or path: %s", path)
				}
				checkRequest(t, body, test.json)
				return []byte(`{"code":0,"result":{"did":"123","siid":2,"aiid":1,"code":0,"out":[42]}}`), nil
			}))
			response, err := client.ExecuteAction(ctx, "123", attr.ActionRequest{ServiceIID: 2, ActionIID: 1, Input: test.input})
			if err != nil {
				t.Fatal(err)
			}
			if response == nil || len(response.Output) != 1 || response.Output[0] != json.Number("42") {
				t.Fatalf("unexpected action result: %+v", response)
			}
		})
	}
}

func TestExecuteActionChecksResultCode(t *testing.T) {
	client := NewClient(transportFunc(func(context.Context, string, any) ([]byte, error) {
		return []byte(`{"code":0,"result":{"did":"123","siid":2,"aiid":1,"code":-704042011,"out":[]}}`), nil
	}))
	response, err := client.ExecuteAction(t.Context(), "123", attr.ActionRequest{ServiceIID: 2, ActionIID: 1})
	var businessErr *model.BusinessError
	if response != nil || !errors.As(err, &businessErr) || businessErr.Code != -704042011 {
		t.Fatalf("expected action failure, got %+v, %v", response, err)
	}
}

func TestClientCanBeReusedAfterCallCancellation(t *testing.T) {
	client := NewClient(transportFunc(func(ctx context.Context, _ string, _ any) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []byte(`{"code":0,"result":[{"did":"123","siid":2,"piid":1,"code":0,"value":true}]}`), nil
	}))
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	response, err := client.GetProperties(ctx, "123", attr.PropertyRef{ServiceIID: 2, PropertyIID: 1})
	if response != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled call, got %+v, %v", response, err)
	}
	if _, err := client.GetProperties(t.Context(), "123", attr.PropertyRef{ServiceIID: 2, PropertyIID: 1}); err != nil {
		t.Fatalf("client retained the canceled context: %v", err)
	}
}

func TestPropertiesRejectIncompleteAndMissingCodes(t *testing.T) {
	for _, body := range []string{`{"code":0,"result":null}`, `{"code":0,"result":[]}`, `{"code":0,"result":[{"did":"123","siid":2,"piid":1}]}`, `{"code":0,"result":[{"did":"other","siid":2,"piid":1,"code":0}]}`} {
		c := NewClient(transportFunc(func(context.Context, string, any) ([]byte, error) { return []byte(body), nil }))
		if _, err := c.GetProperties(t.Context(), "123", attr.PropertyRef{ServiceIID: 2, PropertyIID: 1}); err == nil {
			t.Fatalf("invalid result accepted: %s", body)
		}
	}
}
