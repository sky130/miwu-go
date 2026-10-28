package home

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/miot"
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
	decode := func(raw string) any {
		t.Helper()
		decoder := json.NewDecoder(strings.NewReader(raw))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if !reflect.DeepEqual(decode(string(encoded)), decode(expected)) {
		t.Fatalf("request = %s, want %s", encoded, expected)
	}
}

func TestListHomesOptionsAndResponse(t *testing.T) {
	ctx := t.Context()
	expected := `{"app_ver":7,"fetch_share":true,"fetch_share_dev":true,"fg":false,"limit":300}`
	client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
		if gotCtx != ctx || path != "v2/homeroom/gethome" {
			t.Fatalf("unexpected context or path: %s", path)
		}
		checkRequest(t, body, expected)
		return []byte(`{"code":0,"result":{
			"homelist":[{"id":"9007199254740993","uid":2391366608,"name":"My home","roomlist":[{"id":42,"name":"Living room"}]}],
			"share_home_list":[{"id":"202","uid":555,"name":"Shared home","shareflag":1}],
			"has_more":true,"max_id":"202"
		}}`), nil
	}))
	response, err := client.ListHomes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Homes) != 1 || miot.RefOf(&response.Homes[0]) != (miot.HomeRef{ID: 9007199254740993, OwnerUID: 2391366608}) {
		t.Fatalf("unexpected homes: %+v", response.Homes)
	}
	if response.ShareHomes == nil || len(*response.ShareHomes) != 1 || !(*response.ShareHomes)[0].IsShareHome() ||
		!response.HasMore || response.MaxID != "202" || response.Homes[0].Rooms[0].ID != 42 {
		t.Fatalf("lost shared homes, rooms, or pagination: %+v", response)
	}

	expected = `{"app_ver":8,"fetch_share":false,"fetch_share_dev":false,"fg":false,"limit":10}`
	if _, err := client.ListHomes(ctx, WithFetchShare(false), WithFetchShareDev(false), WithAppVer(8), WithHomeLimit(5), WithHomeLimit(10)); err != nil {
		t.Fatal(err)
	}
	expected = `{"app_ver":7,"fetch_share":true,"fetch_share_dev":true,"fg":false,"limit":300}`
	if _, err := client.ListHomes(ctx); err != nil {
		t.Fatalf("options leaked between calls: %v", err)
	}
}

func TestListDevicesUsesHomeOwnerAndNumericIDs(t *testing.T) {
	ctx := t.Context()
	home := miot.HomeRef{ID: 9007199254740993, OwnerUID: 2391366608}
	expected := `{"home_owner":2391366608,"home_id":9007199254740993,"limit":200}`
	client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
		if gotCtx != ctx || path != "v2/home/home_device_list" {
			t.Fatalf("unexpected context or path: %s", path)
		}
		checkRequest(t, body, expected)
		return []byte(`{"code":0,"result":{"home_info":{"id":9007199254740993,"roomlist":[{"id":42,"dids":["device-1"]}]},
			"device_info":[{"did":"device-1","uid":2391366608,"name":"Light","model":"test.light","isOnline":true}],
			"has_more":true,"max_did":"device-1"}}`), nil
	}))
	response, err := client.ListDevices(ctx, home)
	if err != nil {
		t.Fatal(err)
	}
	result := response
	if result.HomeInfo == nil || result.HomeInfo.ID != home.ID || result.DeviceInfo == nil || len(*result.DeviceInfo) != 1 ||
		(*result.DeviceInfo)[0].ID != "device-1" || !result.HasMore || result.MaxDeviceID != "device-1" {
		t.Fatalf("unexpected devices: %+v", result)
	}
	expected = `{"home_owner":2391366608,"home_id":9007199254740993,"limit":10}`
	if _, err := client.ListDevices(ctx, home, WithDeviceLimit(10)); err != nil {
		t.Fatal(err)
	}
	expected = `{"home_owner":555,"home_id":202,"limit":200}`
	if _, err := client.ListDevices(ctx, miot.HomeRef{ID: 202, OwnerUID: 555}); err != nil {
		t.Fatalf("home or limit leaked between calls: %v", err)
	}
}

func TestListScenesUsesStringIDs(t *testing.T) {
	ctx := t.Context()
	client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
		if gotCtx != ctx || path != "appgateway/miot/appsceneservice/AppSceneService/GetSimpleSceneList" {
			t.Fatalf("unexpected context or path: %s", path)
		}
		checkRequest(t, body, `{"app_version":12,"get_type":2,"home_id":"9007199254740993","owner_uid":"2391366608"}`)
		return []byte(`{"code":0,"result":{"distance_ceiling":5,"manual_scene_info_list":[{"scene_id":"scene-1","name":"Evening","enable":true}]}}`), nil
	}))
	response, err := client.ListScenes(ctx, miot.HomeRef{ID: 9007199254740993, OwnerUID: 2391366608})
	if err != nil {
		t.Fatal(err)
	}
	if response.Scenes == nil || len(*response.Scenes) != 1 || (*response.Scenes)[0].SceneID != "scene-1" ||
		response.DistanceCeiling != 5 {
		t.Fatalf("unexpected scenes: %+v", response)
	}
}

func TestRunSceneUsesNewAPI(t *testing.T) {
	for _, result := range []string{`null`, `{"success":true}`, `"ok"`} {
		t.Run(result, func(t *testing.T) {
			ctx := t.Context()
			client := NewClient(transportFunc(func(gotCtx context.Context, path string, body any) ([]byte, error) {
				if gotCtx != ctx || path != "appgateway/miot/appsceneservice/AppSceneService/NewRunScene" {
					t.Fatalf("unexpected context or path: %s", path)
				}
				checkRequest(t, body, `{"home_id":"9007199254740993","owner_uid":"2391366608","scene_id":"scene-1","phone_id":"null","scene_type":2}`)
				return []byte(`{"code":0,"result":` + result + `}`), nil
			}))
			if err := client.RunScene(ctx, miot.HomeRef{ID: 9007199254740993, OwnerUID: 2391366608}, miot.Scene{SceneID: "scene-1"}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestHomeEndpointsPropagateErrors(t *testing.T) {
	for _, endpoint := range []string{"homes", "devices", "scenes", "run scene"} {
		for _, failure := range []string{"business", "decode", "transport", "cancellation"} {
			t.Run(endpoint+"/"+failure, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				transportErr := errors.New("transport failure")
				if failure == "cancellation" {
					cancel()
				}
				client := NewClient(transportFunc(func(gotCtx context.Context, _ string, _ any) ([]byte, error) {
					if gotCtx != ctx {
						t.Fatal("caller context was not propagated")
					}
					switch failure {
					case "business":
						return []byte(`{"code":-6,"message":"invalid request","result":null}`), nil
					case "decode":
						return []byte(`{`), nil
					case "transport":
						return nil, transportErr
					default:
						return nil, gotCtx.Err()
					}
				}))
				home := miot.HomeRef{ID: 101, OwnerUID: 2391366608}
				var err error
				switch endpoint {
				case "homes":
					_, err = client.ListHomes(ctx)
				case "devices":
					_, err = client.ListDevices(ctx, home)
				case "scenes":
					_, err = client.ListScenes(ctx, home)
				default:
					err = client.RunScene(ctx, home, miot.Scene{SceneID: "scene-1"})
				}
				if err == nil {
					t.Fatal("endpoint swallowed the failure")
				}
				switch failure {
				case "business":
					var businessErr *model.BusinessError
					if !errors.As(err, &businessErr) || businessErr.Code != -6 {
						t.Fatalf("lost business error: %v", err)
					}
				case "transport":
					if !errors.Is(err, transportErr) {
						t.Fatalf("lost transport error: %v", err)
					}
				case "cancellation":
					if !errors.Is(err, context.Canceled) {
						t.Fatalf("lost cancellation: %v", err)
					}
				}
			})
		}
	}
}

func TestHomeClientRejectsMissingParameters(t *testing.T) {
	client := NewClient(transportFunc(func(context.Context, string, any) ([]byte, error) {
		t.Fatal("invalid parameters must not reach the transport")
		return nil, nil
	}))
	ctx := t.Context()
	home := miot.HomeRef{ID: 101, OwnerUID: 2391366608}
	for _, limit := range []int{0, -1} {
		if _, err := client.ListHomes(ctx, WithHomeLimit(limit)); err == nil {
			t.Fatal("accepted invalid home limit")
		}
		if _, err := client.ListDevices(ctx, home, WithDeviceLimit(limit)); err == nil {
			t.Fatal("accepted invalid device limit")
		}
	}
	for _, ref := range []miot.HomeRef{{}, {ID: 101}, {OwnerUID: 2391366608}} {
		if _, err := client.ListDevices(ctx, ref); err == nil {
			t.Fatal("accepted incomplete home for devices")
		}
		if _, err := client.ListScenes(ctx, ref); err == nil {
			t.Fatal("accepted incomplete home for scenes")
		}
		if err := client.RunScene(ctx, ref, miot.Scene{SceneID: "scene-1"}); err == nil {
			t.Fatal("accepted incomplete home for scene execution")
		}
	}
	if err := client.RunScene(ctx, home, miot.Scene{}); err == nil {
		t.Fatal("accepted empty scene ID")
	}
}
