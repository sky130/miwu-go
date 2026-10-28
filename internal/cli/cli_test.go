package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"miwu-go/internal/app"
	"miwu-go/internal/config"
	"miwu-go/internal/miot/auth"
	"miwu-go/internal/miot/model/user"
	"miwu-go/internal/userstore"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

type testAuth struct {
	passwordErr error
}

func (a testAuth) GenerateQRCode(context.Context) (*auth.QRCode, error) {
	return &auth.QRCode{Data: "https://example.com/qr"}, nil
}

func (a testAuth) LoginWithQRCode(context.Context, *auth.QRCode) (user.Credentials, error) {
	return testCredentials(), nil
}

func (a testAuth) LoginWithPassword(context.Context, string, string) (user.Credentials, error) {
	return testCredentials(), a.passwordErr
}

func (a testAuth) Refresh(_ context.Context, c user.Credentials) (user.Credentials, error) {
	c.ServiceToken = "refreshed"
	return c, nil
}

func testCredentials() user.Credentials {
	return user.Credentials{Identity: user.Identity{UserID: "123", DeviceID: "device"}, Session: user.Session{ServiceToken: "TOP-SECRET", PassToken: "PASS-SECRET", ServiceSecurity: "c2VjcmV0"}}
}

type fixture struct {
	cfg      *config.File
	store    *userstore.FileStore
	requests atomic.Int32
	custom   roundTripFunc
	auth     testAuth
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	cipher, err := userstore.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store, err := userstore.NewFileStore(t.TempDir(), cipher)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Save(t.Context(), testCredentials()); err != nil {
		t.Fatal(err)
	}
	return &fixture{cfg: config.NewFile(t.TempDir()), store: store}
}

func (f *fixture) open(t *testing.T) func(Settings) (*app.App, error) {
	return func(settings Settings) (*app.App, error) {
		hc := http.Client{Timeout: settings.Timeout, Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			f.requests.Add(1)
			if f.custom != nil {
				return f.custom(r)
			}
			switch r.URL.Path {
			case "/app/home/profile":
				return response(`{"code":0,"result":{"userid":123,"nickname":"User"}}`), nil
			case "/app/v2/homeroom/gethome":
				return response(`{"code":0,"result":{"homelist":[],"share_home_list":[{"id":"202","uid":555,"name":"Shared","shareflag":1}],"has_more":true,"max_id":"202"}}`), nil
			case "/app/v2/home/home_device_list":
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				var body map[string]json.Number
				if err := json.Unmarshal([]byte(r.Form.Get("data")), &body); err != nil {
					t.Error(err)
				}
				if body["home_owner"] != "555" || body["home_id"] != "202" {
					t.Error("shared home used the logged-in user as owner")
				}
				return response(`{"code":0,"result":{"device_info":[{"did":"lamp","name":"Light","model":"test.lamp","isOnline":true,"spec_type":"urn:miot-spec-v2:device:light:0000A001:test-lamp:1","token":"DEVICE-SECRET"}],"has_more":true,"max_did":"lamp"}}`), nil
			case "/app/miotspec/prop/get":
				return response(`{"code":0,"result":[{"did":"lamp","siid":2,"piid":1,"code":0,"value":9007199254740993},{"did":"lamp","siid":2,"piid":2,"code":-1}]}`), nil
			case "/app/miotspec/prop/set":
				return response(`{"code":0,"result":[{"did":"lamp","siid":2,"piid":1,"code":0}]}`), nil
			case "/app/miotspec/action":
				return response(`{"code":0,"result":{"did":"lamp","siid":2,"aiid":1,"code":0,"out":[true]}}`), nil
			case "/app/appgateway/miot/appsceneservice/AppSceneService/GetSimpleSceneList":
				return response(`{"code":0,"result":{"manual_scene_info_list":[{"scene_id":"scene","name":"Evening","enable":true}]}}`), nil
			case "/app/appgateway/miot/appsceneservice/AppSceneService/NewRunScene":
				return response(`{"code":0,"result":null}`), nil
			case "/miot-spec-v2/instance":
				return response(`{"services":[{"iid":2,"description":"Switch","properties":[{"iid":1,"description":"Power","format":"bool","access":["read","write"]}],"actions":[{"iid":1,"description":"Toggle","in":[]}]}]}`), nil
			case "/instance/v2/multiLanguage":
				return response(`{"data":{"zh_cn":{"service:2":"Switch","service:2:property:1":"Power"}}}`), nil
			case "/cgi-op/api/v1/baike/v2/product":
				return response(`{"code":0,"data":{"realIcon":"https://example.com/icon"}}`), nil
			default:
				t.Errorf("unexpected request: %s", r.URL)
				return nil, errors.New("unexpected request")
			}
		})}
		return app.New(f.cfg, func() (app.AccountStore, error) { return f.store, nil }, f.auth, hc), nil
	}
}

func runTest(t *testing.T, f *fixture, input string, terminal bool, args ...string) (int, string, string) {
	t.Helper()
	var out, diagnostics bytes.Buffer
	code := Run(t.Context(), args, Options{In: strings.NewReader(input), Out: &out, Err: &diagnostics, Terminal: terminal, ReadPassword: func() ([]byte, error) { return []byte("hidden-password"), nil }, Open: f.open(t)})
	return code, out.String(), diagnostics.String()
}

func assertJSON(t *testing.T, out string) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(out))
	var v any
	if err := decoder.Decode(&v); err != nil {
		t.Fatalf("invalid JSON: %q: %v", out, err)
	}
	if err := decoder.Decode(&v); err != io.EOF {
		t.Fatalf("extra output: %q", out)
	}
}

func TestLocalCommandsNeverInitializeCredentials(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"device", "--help"}, {"spec", "parse-urn", "--urn", "urn:miot-spec-v2:device:light:0000A001:test-lamp:1", "--output", "json"}} {
		var out bytes.Buffer
		code := Run(t.Context(), args, Options{Out: &out, Open: func(Settings) (*app.App, error) { t.Fatal("local command initialized credentials"); return nil, nil }})
		if code != 0 {
			t.Fatal(code)
		}
	}
}

func TestArgumentsAndCancellation(t *testing.T) {
	f := newFixture(t)
	for _, args := range [][]string{{"unknown"}, {"device", "get"}, {"device", "action", "--siid", "2"}, {"--unknown"}, {"--timeout", "0"}, {"--home", "-1", "home", "list"}, {"login", "--password-stdin"}} {
		args = append(args, "--output=json")
		code, out, _ := runTest(t, f, "", false, args...)
		if code != 2 {
			t.Fatalf("%v: exit=%d, %s", args, code, out)
		}
		assertJSON(t, out)
	}
	code, _, _ := runTest(t, f, "", true)
	if code != 130 {
		t.Fatalf("EOF exit=%d", code)
	}
	code, _, _ = runTest(t, f, "20\n", true)
	if code != 0 {
		t.Fatalf("menu exit=%d", code)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if code = Run(ctx, nil, Options{Terminal: true}); code != 130 {
		t.Fatal(code)
	}
}

func TestAccountPersistenceAndPasswordLogin(t *testing.T) {
	f := newFixture(t)
	code, out, _ := runTest(t, f, "password\n", false, "login", "--username", "user", "--password-stdin", "--output=json")
	if code != 0 {
		t.Fatal(out)
	}
	assertJSON(t, out)
	for _, args := range [][]string{{"account", "use", "--account", "123"}, {"account", "show"}, {"account", "check"}, {"account", "refresh"}} {
		code, out, errOut := runTest(t, f, "", false, args...)
		if code != 0 {
			t.Fatalf("%v: %s %s", args, out, errOut)
		}
	}
	cfg, err := f.cfg.Load()
	if err != nil || cfg.DefaultAccount != "123" {
		t.Fatal("default account lost")
	}
	code, _, _ = runTest(t, f, "", false, "account", "delete", "--yes")
	if code != 0 {
		t.Fatal(code)
	}
	cfg, err = f.cfg.Load()
	if err != nil || cfg.DefaultAccount != "" {
		t.Fatal("deleted default account remained configured")
	}
}

func TestSDKCommandsAndRedactedJSON(t *testing.T) {
	f := newFixture(t)
	for _, args := range [][]string{
		{"account", "list"}, {"account", "show"}, {"home", "list"}, {"device", "list"}, {"device", "show", "--did", "lamp"},
		{"device", "set", "--did", "lamp", "--set", "2.1=false"}, {"device", "action", "--did", "lamp", "--siid", "2", "--aiid", "1"},
		{"scene", "list"}, {"scene", "run", "--scene", "scene"},
		{"spec", "show", "--urn", "urn:miot-spec-v2:device:light:0000A001:test-lamp:1"},
		{"spec", "languages", "--urn", "urn:miot-spec-v2:device:light:0000A001:test-lamp:1"}, {"spec", "icon", "--model", "test.lamp"},
	} {
		args = append(args, "--account", "123", "--home", "202", "--output=json")
		code, out, errOut := runTest(t, f, "", false, args...)
		if code != 0 {
			t.Fatalf("%v: exit=%d %s %s", args, code, out, errOut)
		}
		assertJSON(t, out)
		if strings.Contains(out+errOut, "SECRET") {
			t.Fatalf("credentials leaked: %s %s", out, errOut)
		}
	}
	code, out, _ := runTest(t, f, "", false, "device", "get", "--did", "lamp", "--property", "2.1", "--property", "2.2", "--account", "123", "--output=json")
	if code != 3 || !strings.Contains(out, "9007199254740993") || !strings.Contains(out, "partial_failure") {
		t.Fatalf("partial results lost: exit=%d %s", code, out)
	}
	assertJSON(t, out)
}

func TestCLIAndApplicationReturnTheSameData(t *testing.T) {
	fixture := newFixture(t)
	application, err := fixture.open(t)(Settings{})
	if err != nil {
		t.Fatal(err)
	}
	ref := app.HomeRef{ID: "202", OwnerUID: "555"}
	homes, err := application.ListHomes(t.Context(), "123", app.DefaultHomeLimit)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := application.ListDevices(t.Context(), "123", ref, app.DefaultDeviceLimit)
	if err != nil {
		t.Fatal(err)
	}
	scenes, err := application.ListScenes(t.Context(), "123", ref)
	if err != nil {
		t.Fatal(err)
	}
	accounts, err := application.ListAccounts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		args []string
		data any
	}{
		{[]string{"home", "list"}, homes},
		{[]string{"device", "list"}, devices},
		{[]string{"scene", "list"}, scenes},
		{[]string{"account", "list"}, accounts},
	} {
		args := append(test.args, "--account", "123", "--home", "202", "--output=json")
		code, output, diagnostics := runTest(t, fixture, "", false, args...)
		if code != 0 {
			t.Fatalf("%v: %s %s", args, output, diagnostics)
		}
		var envelope struct {
			Data  json.RawMessage `json:"data"`
			Error json.RawMessage `json:"error"`
		}
		if err := json.Unmarshal([]byte(output), &envelope); err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(test.data)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(envelope.Data, want) || string(envelope.Error) != "null" {
			t.Fatalf("%v: CLI data %s differs from application data %s", args, envelope.Data, want)
		}
	}
}

func TestPropertyWizardAndVerificationFallback(t *testing.T) {
	f := newFixture(t)
	code, out, errOut := runTest(t, f, "1\n1\n1\nfalse\n", true, "device", "set", "--account", "123", "--home", "202")
	if code != 0 || !strings.Contains(out, "lamp") || !strings.Contains(errOut, "expected a JSON boolean") {
		t.Fatalf("wizard failed: %d %s %s", code, out, errOut)
	}
	f.auth.passwordErr = auth.ErrChallengeRequired
	code, out, errOut = runTest(t, f, "y\n", true, "login", "--username", "user")
	if code != 0 || strings.Contains(out+errOut, "hidden-password") {
		t.Fatalf("QR fallback failed: %d %s", code, out)
	}
	code, out, _ = runTest(t, f, "password\n", false, "login", "--username", "user", "--password-stdin", "--output=json")
	if code != 1 || !strings.Contains(out, "verification_required") {
		t.Fatal("script login challenge was hidden")
	}
}

func TestWriteFailureIsNotRetried(t *testing.T) {
	f := newFixture(t)
	f.custom = func(r *http.Request) (*http.Response, error) { return nil, context.DeadlineExceeded }
	code, out, _ := runTest(t, f, "", false, "device", "set", "--did", "lamp", "--set", "2.1=false", "--account", "123", "--output=json")
	if code != 1 || f.requests.Load() != 1 || !strings.Contains(out, "outcome_unknown") {
		t.Fatalf("write outcome handling failed: %d %s requests=%d", code, out, f.requests.Load())
	}
}

func TestJSONValuePrecision(t *testing.T) {
	value, err := parseJSON("9007199254740993")
	if err != nil || value != json.Number("9007199254740993") {
		t.Fatal("number rounded")
	}
	for _, raw := range []string{"1 2", "false true", "{", ""} {
		if _, err := parseJSON(raw); err == nil {
			t.Fatal("invalid value accepted")
		}
	}
}

func TestActionWizard(t *testing.T) {
	f := newFixture(t)
	code, out, diagnostics := runTest(t, f, "1\n1\n", true, "device", "action", "--account", "123", "--home", "202")
	if code != 0 || !strings.Contains(out, `"out"`) {
		t.Fatalf("action wizard failed: %d %s %s", code, out, diagnostics)
	}
}

type announcingReader struct {
	io.Reader
	once    sync.Once
	started chan struct{}
}

func (r *announcingReader) Read(b []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	return r.Reader.Read(b)
}

func TestCancellationReleasesPendingInput(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	defer r.Close()
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { <-started; cancel() }()
	code := Run(ctx, nil, Options{Terminal: true, In: &announcingReader{Reader: r, started: started}, InterruptInput: func() { _ = r.Close() }})
	if code != 130 {
		t.Fatal(code)
	}
}

func TestPasswordCancellation(t *testing.T) {
	f := newFixture(t)
	started := make(chan struct{})
	released := make(chan struct{})
	finished := make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() { <-started; cancel() }()
	code := Run(ctx, []string{"login", "--username", "user"}, Options{Terminal: true, Open: f.open(t), ReadPassword: func() ([]byte, error) { close(started); <-released; close(finished); return nil, io.EOF }, InterruptInput: func() { close(released) }})
	if code != 130 {
		t.Fatal(code)
	}
	<-finished
}
