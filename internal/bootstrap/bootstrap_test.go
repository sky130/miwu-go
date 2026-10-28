package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"miwu-go/internal/config"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (transport transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestOpenKeepsAccountInitializationLazy(t *testing.T) {
	configDir := t.TempDir()
	dataDir := filepath.Join(t.TempDir(), "accounts")
	if err := config.NewFile(configDir).Save(config.Config{DefaultAccount: "123"}); err != nil {
		t.Fatal(err)
	}
	application, err := Open(Options{ConfigDir: configDir, DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	if id, err := application.DefaultAccount(); err != nil || id != "123" {
		t.Fatalf("configuration override ignored: %q, %v", id, err)
	}
	if _, err := os.Stat(dataDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("account directory accessed during application construction: %v", err)
	}
	if _, err := Open(Options{RequestTimeout: -time.Second}); err == nil {
		t.Fatal("negative timeout accepted")
	}
}

func TestRequestDiagnosticsExcludeCredentials(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var diagnostics bytes.Buffer
		transport := &traceTransport{
			out: &diagnostics,
			base: transportFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.RawQuery != "token=QUERY-SECRET" {
					t.Error("request was modified")
				}
				if fail {
					return nil, context.DeadlineExceeded
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("RESPONSE-SECRET"))}, nil
			}),
		}
		request, err := http.NewRequest(http.MethodPost, "https://example.com/PATH-SECRET?token=QUERY-SECRET", strings.NewReader("BODY-SECRET"))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "HEADER-SECRET")
		response, err := transport.RoundTrip(request)
		if fail && !errors.Is(err, context.DeadlineExceeded) || !fail && err != nil {
			t.Fatal(err)
		}
		if response != nil {
			response.Body.Close()
		}
		output := diagnostics.String()
		if strings.Contains(output, "SECRET") || !strings.Contains(output, "method=POST host=example.com") {
			t.Fatalf("unsafe diagnostics: %q", output)
		}
		if fail && !strings.Contains(output, "failed=true") {
			t.Fatal("failure missing from diagnostics")
		}
	}
}
