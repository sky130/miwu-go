package home

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/model/miot"
)

const sceneServicePath = "appgateway/miot/appsceneservice/AppSceneService/"

type Client struct {
	client m.Transport
}

func NewClient(client m.Transport) *Client {
	return &Client{client: client}
}

func (c *Client) ListHomes(ctx context.Context, options ...ListHomesOption) (*miot.HomeList, error) {
	params := defaultListHomesParams()
	for _, option := range options {
		option(&params)
	}
	if params.limit <= 0 {
		return nil, fmt.Errorf("%w: list homes: limit must be positive", m.ErrInvalidArgument)
	}
	body := listHomesRequest{
		AppVersion: params.appVer, FetchShare: params.fetchShare,
		FetchShareDev: params.fetchShareDev, Limit: params.limit,
	}
	return required[miot.HomeList](ctx, c.client, "v2/homeroom/gethome", body)
}

func (c *Client) ListDevices(ctx context.Context, home miot.HomeRef, options ...ListDevicesOption) (*miot.DeviceList, error) {
	if err := validateHome(home); err != nil {
		return nil, err
	}
	params := defaultListDevicesParams()
	for _, option := range options {
		option(&params)
	}
	if params.limit <= 0 {
		return nil, fmt.Errorf("%w: list devices: limit must be positive", m.ErrInvalidArgument)
	}
	body := listDevicesRequest{HomeOwner: home.OwnerUID, HomeID: home.ID, Limit: params.limit}
	return required[miot.DeviceList](ctx, c.client, "v2/home/home_device_list", body)
}

func (c *Client) ListScenes(ctx context.Context, home miot.HomeRef) (*miot.SceneList, error) {
	if err := validateHome(home); err != nil {
		return nil, err
	}
	body := listScenesRequest{
		AppVersion: 12, GetType: 2,
		HomeID: strconv.FormatInt(home.ID, 10), OwnerUID: strconv.FormatInt(home.OwnerUID, 10),
	}
	return required[miot.SceneList](ctx, c.client, sceneServicePath+"GetSimpleSceneList", body)
}

func (c *Client) RunScene(ctx context.Context, home miot.HomeRef, scene miot.Scene) error {
	if err := validateHome(home); err != nil {
		return err
	}
	if scene.SceneID == "" {
		return fmt.Errorf("%w: run scene: scene ID is required", m.ErrInvalidArgument)
	}
	body := runSceneRequest{
		HomeID: strconv.FormatInt(home.ID, 10), OwnerUID: strconv.FormatInt(home.OwnerUID, 10),
		SceneID: scene.SceneID, PhoneID: "null", SceneType: 2,
	}
	_, err := m.Call[json.RawMessage](ctx, c.client, sceneServicePath+"NewRunScene", body)
	return err
}

func validateHome(home miot.HomeRef) error {
	if home.ID <= 0 || home.OwnerUID <= 0 {
		return fmt.Errorf("%w: home ID and owner UID are required", m.ErrInvalidArgument)
	}
	return nil
}

func required[T any](ctx context.Context, transport m.Transport, path string, body any) (*T, error) {
	response, err := m.Call[*T](ctx, transport, path, body)
	if err != nil {
		return nil, err
	}
	if response.Result == nil {
		return nil, fmt.Errorf("%w: missing list result", m.ErrInvalidResponse)
	}
	return response.Result, nil
}
