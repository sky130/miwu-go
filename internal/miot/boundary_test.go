package miot

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"miwu-go/internal/miot/crypto"
	"miwu-go/internal/miot/model/user"
)

func TestConfiguredBaseSigningAndNoRedirectReplay(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/custom/home/profile" {
			t.Error("authenticated POST was replayed")
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Error(err)
		}
		signed, err := crypto.GenerateSignedNonce("c2VjcmV0", values.Get("_nonce"))
		if err != nil {
			t.Error(err)
		}
		expected, err := crypto.GenerateSignature("/home/profile", signed, values.Get("_nonce"), values.Get("data"))
		if err != nil {
			t.Error(err)
		}
		if values.Get("signature") != expected || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Error("wrong base-relative signature or MIME")
		}
		w.Header().Set("Location", "/redirect")
		w.WriteHeader(307)
	}))
	defer server.Close()
	client := NewClient(*server.Client(), user.Credentials{Session: user.Session{ServiceSecurity: "c2VjcmV0"}}, server.URL+"/custom/")
	_, err := client.Post(t.Context(), "home/profile", map[string]string{"id": "1"})
	var status *HTTPError
	if !errors.As(err, &status) || status.Status != 307 || requests != 1 {
		t.Fatalf("redirect policy failed: requests=%d err=%v", requests, err)
	}
}

func FuzzResponseEnvelope(f *testing.F) {
	f.Add([]byte(`{"code":0,"result":null}`))
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Call[any](t.Context(), transportFunc(func(_ context.Context, _ string, _ any) ([]byte, error) { return data, nil }), "test", nil)
	})
}
