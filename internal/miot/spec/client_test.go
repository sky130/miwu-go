package spec

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

func TestGetWithLanguage(t *testing.T) {
	urn := "urn:miot-spec-v2:device:air-condition-outlet:0000A045:lumi-mcn04:1"
	ctx := t.Context()
	client := NewClient(http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Context() != ctx {
			t.Fatal("caller context was not propagated")
		}
		switch req.URL.Path {
		case "/miot-spec-v2/instance":
			if req.URL.Query().Get("type") != urn {
				t.Fatalf("unexpected spec query: %s", req.URL)
			}
			return jsonResponse(`{"services":[{"iid":2,"description":"switch",
				"properties":[{"iid":1,"description":"power","value-list":[{"value":0,"description":"off"}]}],
				"actions":[{"iid":1,"description":"toggle"}]}]}`), nil
		case "/instance/v2/multiLanguage":
			if req.URL.Query().Get("urn") != urn {
				t.Fatalf("unexpected language query: %s", req.URL)
			}
			return jsonResponse(`{"data":{"zh_cn":{"service:2":"开关","service:2:property:1":"电源",
				"service:2:property:1:valuelist:0":"关闭","service:2:action:1":"切换"}}}`), nil
		default:
			t.Fatalf("unexpected request: %s", req.URL)
			return nil, errors.New("unexpected request")
		}
	})})

	attr, err := client.GetWithLanguage(ctx, urn, LanguageChinese)
	if err != nil {
		t.Fatalf("get attr failure: %v", err)
	}
	service := attr.Services[0]
	if service.Translation != "开关" || (*service.Properties)[0].Translation != "电源" ||
		(*(*service.Properties)[0].ValueList)[0].Translation != "关闭" || (*service.Actions)[0].Translation != "切换" {
		t.Fatalf("translations were not applied: %+v", service)
	}

	for _, service := range attr.Services {
		if service.Translation == "" {
			t.Errorf("service translation is empty, siid: %d, desc: %s", service.IID, service.Description)
		}

		if service.Properties != nil {
			for _, prop := range *service.Properties {
				if prop.Translation == "" {
					t.Errorf("property translation is empty, siid: %d, piid: %d, desc: %s", service.IID, prop.IID, prop.Description)
				}

				if prop.ValueList != nil {
					for _, val := range *prop.ValueList {
						if val.Translation == "" {
							t.Errorf("valuelist translation is empty, siid: %d, piid: %d, value: %d", service.IID, prop.IID, val.Value)
						}
					}
				}
			}
		}

		if service.Actions != nil {
			for _, act := range *service.Actions {
				if act.Translation == "" {
					t.Errorf("action translation is empty, siid: %d, aiid: %d, desc: %s", service.IID, act.IID, act.Description)
				}
			}
		}
	}
}

func TestSpecEndpointsPropagateCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		call func(*Client, context.Context) error
	}{
		{"spec", func(client *Client, ctx context.Context) error { _, err := client.Get(ctx, "urn:test"); return err }},
		{"language", func(client *Client, ctx context.Context) error {
			_, err := client.Languages(ctx, "urn:test")
			return err
		}},
		{"translated spec", func(client *Client, ctx context.Context) error {
			_, err := client.GetWithLanguage(ctx, "urn:test", LanguageChinese)
			return err
		}},
		{"icon", func(client *Client, ctx context.Context) error {
			_, err := client.IconURL(ctx, "lumi.test")
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			client := NewClient(http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Context().Err() == nil {
					return nil, errors.New("request lost its canceled context")
				}
				return nil, req.Context().Err()
			})})
			if err := test.call(client, ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("expected cancellation, got %v", err)
			}
		})
	}
}

func TestGetIconURLPreservesModelQuery(t *testing.T) {
	ctx := t.Context()
	client := NewClient(http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Context() != ctx || req.URL.Query().Get("model") != "lumi.test" {
			t.Fatalf("unexpected icon request: %s", req.URL)
		}
		return jsonResponse(`{"code":0,"data":{"realIcon":"https://example.com/icon.png"}}`), nil
	})})
	icon, err := client.IconURL(ctx, "lumi.test")
	if err != nil || icon == nil || *icon != "https://example.com/icon.png" {
		t.Fatalf("unexpected icon: %v, %v", icon, err)
	}
}
