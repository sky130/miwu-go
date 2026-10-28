package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"miwu-go/internal/app"
	"miwu-go/internal/config"
	"miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/attr"
	"miwu-go/internal/miot/model/user"
	"miwu-go/internal/userstore"
)

const (
	sharedHomeID = "9007199254740993"
	sharedOwner  = "9007199254740995"
	testURN      = "urn:miot-spec-v2:device:light:0000A001:test-lamp:1"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (transport transportFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func credentials(id string) user.Credentials {
	return user.Credentials{
		Identity: user.Identity{UserID: id, DeviceID: "device"},
		Session: user.Session{
			ServiceToken:    "TOKEN-SECRET-" + id,
			ServiceSecurity: "c2VjcmV0",
			PassToken:       "PASS-SECRET",
		},
	}
}

func newApplication(t *testing.T, transport http.RoundTripper) (*app.App, *userstore.FileStore) {
	t.Helper()
	cipher, err := userstore.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	store, err := userstore.NewFileStore(t.TempDir(), cipher)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"123", "456"} {
		if err := store.Save(t.Context(), credentials(id)); err != nil {
			t.Fatal(err)
		}
	}
	return app.New(config.NewFile(t.TempDir()), func() (app.AccountStore, error) {
		return store, nil
	}, nil, http.Client{Transport: transport}), store
}

func TestSharedHomeWorkflowWithoutCLI(t *testing.T) {
	var writes atomic.Int32
	application, _ := newApplication(t, transportFunc(func(request *http.Request) (*http.Response, error) {
		if err := request.ParseForm(); err != nil {
			t.Error(err)
		}
		switch request.URL.Path {
		case "/app/home/profile":
			return jsonResponse(`{"code":0,"result":{"userid":123,"nickname":"User","icon":"avatar"}}`), nil
		case "/app/v2/homeroom/gethome":
			return jsonResponse(`{"code":0,"result":{"homelist":[{"id":"101","uid":123,"name":"Owned"}],"share_home_list":[{"id":"101","uid":123,"name":"Duplicate"},{"id":"9007199254740993","uid":9007199254740995,"name":"Shared","shareflag":1}],"has_more":true,"max_id":"next"}}`), nil
		case "/app/v2/home/home_device_list":
			var body map[string]json.Number
			if err := json.Unmarshal([]byte(request.Form.Get("data")), &body); err != nil {
				t.Error(err)
			}
			if body["home_owner"] != sharedOwner || body["home_id"] != sharedHomeID {
				t.Error("shared owner or home ID lost precision")
			}
			return jsonResponse(`{"code":0,"result":{"device_info":[{"did":"lamp","uid":9007199254740995,"name":"Light","model":"test.lamp","isOnline":true,"spec_type":"urn:miot-spec-v2:device:light:0000A001:test-lamp:1","token":"DEVICE-SECRET"}],"has_more":true,"max_did":"next-device"}}`), nil
		case "/app/miotspec/prop/get", "/app/miotspec/prop/set":
			return jsonResponse(`{"code":0,"result":[{"did":"lamp","siid":2,"piid":1,"code":0,"value":9007199254740993},{"did":"lamp","siid":2,"piid":2,"code":-1}]}`), nil
		case "/app/miotspec/action":
			return jsonResponse(`{"code":0,"result":{"did":"lamp","siid":2,"aiid":1,"code":0,"out":[true]}}`), nil
		case "/app/appgateway/miot/appsceneservice/AppSceneService/GetSimpleSceneList":
			return jsonResponse(`{"code":0,"result":{"manual_scene_info_list":[{"scene_id":"evening","name":"Evening","enable":true}]}}`), nil
		case "/app/appgateway/miot/appsceneservice/AppSceneService/NewRunScene":
			var body struct {
				HomeID   string `json:"home_id"`
				OwnerUID string `json:"owner_uid"`
			}
			if err := json.Unmarshal([]byte(request.Form.Get("data")), &body); err != nil {
				t.Error(err)
			}
			if body.HomeID != sharedHomeID || body.OwnerUID != sharedOwner {
				t.Error("scene used the account instead of the shared owner")
			}
			writes.Add(1)
			return jsonResponse(`{"code":0,"result":null}`), nil
		default:
			return nil, fmt.Errorf("unexpected path: %s", request.URL.Path)
		}
	}))

	if err := application.UseAccount(t.Context(), "123"); err != nil {
		t.Fatal(err)
	}
	accounts, err := application.ListAccounts(t.Context())
	if err != nil || len(accounts) != 2 || !accounts[0].Default || accounts[1].Default {
		t.Fatalf("accounts: %+v, %v", accounts, err)
	}
	info, err := application.AccountInfo(t.Context(), "123")
	if err != nil || info.ID != "123" || info.Nickname != "User" {
		t.Fatalf("profile: %+v, %v", info, err)
	}
	if err := application.CheckSession(t.Context(), "123"); err != nil {
		t.Fatal(err)
	}
	homes, err := application.ListHomes(t.Context(), "123", app.DefaultHomeLimit)
	if err != nil || len(homes.Homes) != 2 || !homes.HasMore || homes.Cursor != "next" {
		t.Fatalf("homes: %+v, %v", homes, err)
	}
	ref, err := homes.Find(sharedHomeID)
	if err != nil || ref.OwnerUID != sharedOwner || !homes.Homes[1].Shared {
		t.Fatalf("shared home: %+v, %v", ref, err)
	}
	devices, err := application.ListDevices(t.Context(), "123", ref, app.DefaultDeviceLimit)
	if err != nil || !devices.HasMore || devices.Cursor != "next-device" {
		t.Fatalf("devices: %+v, %v", devices, err)
	}
	device, err := devices.Find("lamp")
	if err != nil || !device.Online || device.OwnerUID != sharedOwner {
		t.Fatalf("device: %+v, %v", device, err)
	}
	for _, value := range []any{accounts, info, homes, devices, device} {
		data, err := json.Marshal(value)
		if err != nil || strings.Contains(string(data), "SECRET") {
			t.Fatalf("unsafe result: %s, %v", data, err)
		}
	}
	refs := []attr.PropertyRef{{ServiceIID: 2, PropertyIID: 1}, {ServiceIID: 2, PropertyIID: 2}}
	values, err := application.GetProperties(t.Context(), "123", "lamp", refs...)
	var batch *model.BatchError
	if !errors.As(err, &batch) || len(values) != 2 || values[0].Value != json.Number(sharedHomeID) {
		t.Fatalf("partial read: %+v, %v", values, err)
	}
	values, err = application.SetProperties(t.Context(), "123", "lamp",
		attr.PropertyValue{ServiceIID: 2, PropertyIID: 1, Value: false},
		attr.PropertyValue{ServiceIID: 2, PropertyIID: 2, Value: true},
	)
	var unknown *app.OutcomeUnknownError
	if !errors.As(err, &batch) || errors.As(err, &unknown) || len(values) != 2 {
		t.Fatalf("partial write: %+v, %v", values, err)
	}
	if _, err := application.ExecuteAction(t.Context(), "123", "lamp", attr.ActionRequest{ServiceIID: 2, ActionIID: 1}); err != nil {
		t.Fatal(err)
	}
	scenes, err := application.ListScenes(t.Context(), "123", ref)
	if err != nil || len(scenes) != 1 || scenes[0].ID != "evening" || !scenes[0].Enabled {
		t.Fatalf("scenes: %+v, %v", scenes, err)
	}
	if err := application.RunScene(t.Context(), "123", ref, scenes[0].ID); err != nil || writes.Load() != 1 {
		t.Fatalf("scene execution: %v, writes=%d", err, writes.Load())
	}
	if _, err := homes.Find("missing"); err == nil {
		t.Fatal("missing home accepted")
	}
	if _, err := devices.Find("missing"); err == nil {
		t.Fatal("missing device accepted")
	}
}

func TestApplicationWriteFailuresAreNotRetried(t *testing.T) {
	for _, operation := range []string{"property", "action", "scene"} {
		t.Run(operation, func(t *testing.T) {
			var requests atomic.Int32
			application, _ := newApplication(t, transportFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return nil, context.DeadlineExceeded
			}))
			var err error
			switch operation {
			case "property":
				_, err = application.SetProperties(t.Context(), "123", "lamp", attr.PropertyValue{ServiceIID: 2, PropertyIID: 1, Value: false})
			case "action":
				_, err = application.ExecuteAction(t.Context(), "123", "lamp", attr.ActionRequest{ServiceIID: 2, ActionIID: 1})
			case "scene":
				err = application.RunScene(t.Context(), "123", app.HomeRef{ID: sharedHomeID, OwnerUID: sharedOwner}, "evening")
			}
			var unknown *app.OutcomeUnknownError
			if !errors.As(err, &unknown) || !errors.Is(err, context.DeadlineExceeded) || requests.Load() != 1 {
				t.Fatalf("error=%v, requests=%d", err, requests.Load())
			}
		})
	}
}

func TestCanceledOperationDoesNotOpenStorage(t *testing.T) {
	application := app.New(nil, func() (app.AccountStore, error) {
		t.Fatal("canceled operation opened storage")
		return nil, nil
	}, nil, http.Client{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := application.SetProperties(ctx, "123", "lamp", attr.PropertyValue{ServiceIID: 2, PropertyIID: 1, Value: false})
	var unknown *app.OutcomeUnknownError
	if !errors.Is(err, context.Canceled) || errors.As(err, &unknown) {
		t.Fatal(err)
	}
}

func TestPreflightFailuresAndBusinessRejectionsRemainDistinct(t *testing.T) {
	var requests atomic.Int32
	application, _ := newApplication(t, transportFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return jsonResponse(`{"code":-6,"message":"invalid request","result":null}`), nil
	}))
	var unknown *app.OutcomeUnknownError
	if _, err := application.SetProperties(t.Context(), "123", "lamp"); !errors.Is(err, miot.ErrInvalidArgument) || errors.As(err, &unknown) {
		t.Fatalf("validation became an unknown outcome: %v", err)
	}
	if _, err := application.SetProperties(t.Context(), "missing", "lamp", attr.PropertyValue{ServiceIID: 2, PropertyIID: 1, Value: false}); !errors.Is(err, userstore.ErrNotFound) || errors.As(err, &unknown) {
		t.Fatalf("storage failure became an unknown outcome: %v", err)
	}
	if _, err := application.ListDevices(t.Context(), "123", app.HomeRef{ID: "1", OwnerUID: "invalid"}, app.DefaultDeviceLimit); !errors.Is(err, miot.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if requests.Load() != 0 {
		t.Fatal("invalid operations reached the remote service")
	}
	_, err := application.SetProperties(t.Context(), "123", "lamp", attr.PropertyValue{ServiceIID: 2, PropertyIID: 1, Value: false})
	var business *model.BusinessError
	if !errors.As(err, &business) || business.Code != -6 || errors.As(err, &unknown) || requests.Load() != 1 {
		t.Fatalf("business rejection classification changed: %v", err)
	}
}

func TestInFlightOperationUsesCallerCancellation(t *testing.T) {
	started := make(chan struct{})
	application, _ := newApplication(t, transportFunc(func(request *http.Request) (*http.Response, error) {
		close(started)
		<-request.Context().Done()
		return nil, request.Context().Err()
	}))
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go func() {
		<-started
		cancel()
	}()
	if _, err := application.ListHomes(ctx, "123", app.DefaultHomeLimit); !errors.Is(err, context.Canceled) {
		t.Fatalf("request ignored caller cancellation: %v", err)
	}
}

func TestConcurrentAccountRequestsKeepTheirCredentials(t *testing.T) {
	application, _ := newApplication(t, transportFunc(func(request *http.Request) (*http.Response, error) {
		id, err := request.Cookie("userId")
		if err != nil {
			return nil, err
		}
		token, err := request.Cookie("serviceToken")
		if err != nil {
			return nil, err
		}
		if token.Value != "TOKEN-SECRET-"+id.Value {
			return nil, errors.New("credentials crossed account boundaries")
		}
		return jsonResponse(fmt.Sprintf(`{"code":0,"result":{"homelist":[{"id":"101","uid":%s,"name":"Home"}]}}`, id.Value)), nil
	}))
	var tasks sync.WaitGroup
	for index := range 24 {
		tasks.Go(func() {
			id := "123"
			if index%2 == 1 {
				id = "456"
			}
			if err := application.UseAccount(t.Context(), id); err != nil {
				t.Error(err)
			}
			homes, err := application.ListHomes(t.Context(), id, app.DefaultHomeLimit)
			if err != nil || len(homes.Homes) != 1 || homes.Homes[0].OwnerUID != id {
				t.Errorf("account %s received another account's result: %+v, %v", id, homes, err)
			}
			if _, err := application.DefaultAccount(); err != nil {
				t.Error(err)
			}
		})
	}
	tasks.Wait()
}

func TestSpecificationOperationsDoNotOpenAccounts(t *testing.T) {
	application := app.New(nil, func() (app.AccountStore, error) {
		t.Fatal("public specification operation opened accounts")
		return nil, nil
	}, nil, http.Client{Transport: transportFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/miot-spec-v2/instance":
			return jsonResponse(`{"services":[{"iid":2,"description":"Switch"}]}`), nil
		case "/instance/v2/multiLanguage":
			return jsonResponse(`{"data":{"zh_cn":{"service:2":"Translated"}}}`), nil
		case "/cgi-op/api/v1/baike/v2/product":
			return jsonResponse(`{"code":0,"data":{"realIcon":"https://example.com/icon"}}`), nil
		default:
			return nil, errors.New("unexpected request")
		}
	})})
	parsed, err := app.ParseURN(testURN)
	if err != nil || parsed.URN != testURN {
		t.Fatalf("URN: %+v, %v", parsed, err)
	}
	for _, language := range []string{"", "zh_cn"} {
		specification, err := application.GetSpecification(t.Context(), testURN, language)
		if err != nil || len(specification.Services) != 1 {
			t.Fatalf("specification: %+v, %v", specification, err)
		}
		if language != "" && specification.Services[0].Translation != "Translated" {
			t.Fatal("translation lost")
		}
	}
	if _, err := application.SpecificationLanguages(t.Context(), testURN); err != nil {
		t.Fatal(err)
	}
	if icon, err := application.IconURL(t.Context(), "test.lamp"); err != nil || icon != "https://example.com/icon" {
		t.Fatalf("icon: %q, %v", icon, err)
	}
	if specification, err := application.DeviceSpecification(t.Context(), app.Device{ID: "lamp"}, "zh_cn"); err != nil || specification != nil {
		t.Fatal("missing specification did not preserve explicit-IID fallback")
	}
	if _, err := application.GetSpecification(t.Context(), "invalid", "zh_cn"); !errors.Is(err, miot.ErrInvalidArgument) {
		t.Fatal(err)
	}
}
