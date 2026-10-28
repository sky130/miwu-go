package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/user"
)

func TestPasswordSessionsAreIsolated(t *testing.T) {
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/service":
			if _, err := r.Cookie("userId"); err == nil {
				t.Error("new session inherited another account")
			}
			fmt.Fprint(w, `&&&START&&&{"code":70016,"_sign":"sign","sid":"xiaomiio"}`)
		case "/password":
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Error("password MIME mismatch")
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			id := r.Form.Get("user")
			if r.Form.Get("hash") == "" || r.Form.Get("_sign") != "sign" {
				t.Error("missing login form fields")
			}
			http.SetCookie(w, &http.Cookie{Name: "userId", Value: id, Path: "/"})
			fmt.Fprintf(w, `{"code":0,"userId":%s,"passToken":"pass-%s","ssecurity":"c2VjcmV0","location":%q}`, id, id, base+"/token?uid="+id)
		case "/token":
			cookie, err := r.Cookie("userId")
			if err != nil || cookie.Value != r.URL.Query().Get("uid") {
				t.Errorf("account cookie isolation failed: %v", err)
			}
			http.SetCookie(w, &http.Cookie{Name: "serviceToken", Value: "token-" + r.URL.Query().Get("uid"), Path: "/"})
			fmt.Fprint(w, "ok")
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base = server.URL
	client := NewClient(*server.Client()).WithEndpoints(Endpoints{ServiceLogin: base + "/service", PasswordLogin: base + "/password"})
	var wg sync.WaitGroup
	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := strconv.Itoa(i)
			c, err := client.LoginWithPassword(t.Context(), id, "password")
			if err != nil {
				t.Error(err)
				return
			}
			if c.UserID != id || c.ServiceToken != "token-"+id || c.DeviceID == "" {
				t.Error("wrong account credentials")
			}
		}()
	}
	wg.Wait()
}

func TestQRFlowAndMissingPollURL(t *testing.T) {
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/qr":
			json.NewEncoder(w).Encode(map[string]any{"code": 0, "loginUrl": "https://example.com/scan", "lp": base + "/poll"})
		case "/bad":
			fmt.Fprint(w, `{"code":0,"loginUrl":"https://example.com/scan"}`)
		case "/poll":
			http.SetCookie(w, &http.Cookie{Name: "passToken", Value: "pass", Path: "/"})
			fmt.Fprint(w, `&&&START&&&{"code":0,"userId":9007199254740993,"passToken":"pass","cUserId":"client"}`)
		case "/service":
			if cookie, err := r.Cookie("passToken"); err != nil || cookie.Value != "pass" {
				t.Error("poll identity was not retained")
			}
			fmt.Fprintf(w, `{"code":0,"ssecurity":"c2VjcmV0","location":%q}`, base+"/token")
		case "/token":
			http.SetCookie(w, &http.Cookie{Name: "serviceToken", Value: "service", Path: "/"})
			fmt.Fprint(w, "ok")
		}
	}))
	defer server.Close()
	base = server.URL
	client := NewClient(*server.Client()).WithEndpoints(Endpoints{QRCode: base + "/qr", ServiceLogin: base + "/service"})
	qr, err := client.GenerateQRCode(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	credentials, err := client.LoginWithQRCode(t.Context(), qr)
	if err != nil {
		t.Fatal(err)
	}
	if credentials.UserID != "9007199254740993" || credentials.ServiceToken != "service" {
		t.Fatal("identity precision or token lost")
	}
	bad := client.WithEndpoints(Endpoints{QRCode: base + "/bad"})
	if _, err := bad.GenerateQRCode(t.Context()); !errors.Is(err, m.ErrInvalidResponse) {
		t.Fatalf("missing poll URL: %v", err)
	}
}

func TestAuthenticationErrorsAndCancellation(t *testing.T) {
	for _, tt := range []struct {
		body string
		want error
		code int
	}{
		{`{}`, m.ErrInvalidResponse, 0}, {`{`, m.ErrInvalidResponse, 0},
		{`{"code":81003}`, ErrChallengeRequired, 0}, {`{"code":70016}`, ErrInvalidCredentials, 0},
		{`{"code":0,"notificationUrl":"verification"}`, ErrChallengeRequired, 0},
		{`{"code":12345}`, nil, 12345},
	} {
		r, err := decode([]byte(tt.body))
		if err == nil {
			err = check(r)
		}
		if tt.code != 0 {
			var business *model.BusinessError
			if !errors.As(err, &business) || business.Code != tt.code {
				t.Fatal(err)
			}
		} else if !errors.Is(err, tt.want) {
			t.Fatalf("%s: %v", tt.body, err)
		}
	}
	client := NewClient(http.Client{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := client.GenerateQRCode(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
}

func TestRefreshUsesSnapshotAndRedirectCookies(t *testing.T) {
	var base string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/service":
			cookie, err := r.Cookie("passToken")
			if err != nil || cookie.Value != "old-pass" {
				t.Error("refresh did not seed account cookies")
			}
			fmt.Fprintf(w, `{"code":0,"ssecurity":"new-security","location":%q}`, base+"/token")
		case "/token":
			http.SetCookie(w, &http.Cookie{Name: "serviceToken", Value: "redirect-token", Path: "/"})
			http.Redirect(w, r, "/done", 302)
		case "/done":
			fmt.Fprint(w, "ok")
		}
	}))
	defer server.Close()
	base = server.URL
	original := user.Credentials{Identity: user.Identity{UserID: "1", DeviceID: "device"}, Session: user.Session{PassToken: "old-pass", ServiceToken: "old-token"}}
	client := NewClient(*server.Client()).WithEndpoints(Endpoints{ServiceLogin: base + "/service"})
	refreshed, err := client.Refresh(t.Context(), original)
	if err != nil {
		t.Fatal(err)
	}
	if original.ServiceToken != "old-token" || refreshed.ServiceToken != "redirect-token" || refreshed.DeviceID != "device" {
		t.Fatal("refresh mutated or lost identity")
	}
}

func TestDecodeDoesNotExposeAuthenticationBody(t *testing.T) {
	_, err := decode([]byte(`{"passToken":"secret",`))
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatal("invalid response leaked body")
	}
}

func TestInitialPasswordChallengeDoesNotSubmitCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/service" {
			t.Error("password submitted despite verification challenge")
		}
		fmt.Fprint(w, `{"code":70016,"_sign":"sign","captchaUrl":"verification"}`)
	}))
	defer server.Close()
	client := NewClient(*server.Client()).WithEndpoints(Endpoints{ServiceLogin: server.URL + "/service", PasswordLogin: server.URL + "/password"})
	if _, err := client.LoginWithPassword(t.Context(), "user", "password"); !errors.Is(err, ErrChallengeRequired) {
		t.Fatal(err)
	}
}

func TestExpiredQRCodeDoesNotPoll(t *testing.T) {
	client := NewClient(http.Client{})
	session, err := client.newSession()
	if err != nil {
		t.Fatal(err)
	}
	code := &QRCode{session: session, expiresAt: time.Now().Add(-time.Second)}
	if _, err := client.LoginWithQRCode(t.Context(), code); !errors.Is(err, ErrQRCodeExpired) {
		t.Fatal(err)
	}
}

func FuzzAuthenticationResponse(f *testing.F) {
	f.Add([]byte(`{"code":0}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, b []byte) {
		r, err := decode(b)
		if err == nil {
			_ = check(r)
		}
	})
}
