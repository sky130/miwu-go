package app

import (
	"context"
	"fmt"
	"strconv"

	"miwu-go/internal/miot"
	"miwu-go/internal/miot/client/home"
	"miwu-go/internal/miot/model/attr"
	miotmodel "miwu-go/internal/miot/model/miot"
)

const DefaultDeviceLimit = 200

// Device exposes the information needed by either UI, without device tokens.
type Device struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Model    string  `json:"model"`
	Online   bool    `json:"online"`
	OwnerUID string  `json:"owner_uid"`
	SpecURN  *string `json:"spec_urn,omitempty"`
}

type DeviceList struct {
	Devices []Device `json:"devices"`
	HasMore bool     `json:"has_more"`
	Cursor  string   `json:"cursor"`
}

func (list DeviceList) Find(id string) (Device, error) {
	for _, item := range list.Devices {
		if item.ID == id {
			return item, nil
		}
	}
	return Device{}, fmt.Errorf("device %q is not present in the returned devices", id)
}

func deviceFromMIoT(device miotmodel.Device) Device {
	return Device{
		ID:       device.ID,
		Name:     device.Name,
		Model:    device.Model,
		Online:   device.IsOnline,
		OwnerUID: strconv.FormatInt(device.UID, 10),
		SpecURN:  device.SpecType,
	}
}

func (a *App) ListDevices(ctx context.Context, accountID string, ref HomeRef, limit int) (DeviceList, error) {
	if limit <= 0 {
		return DeviceList{}, fmt.Errorf("%w: device limit must be positive", miot.ErrInvalidArgument)
	}
	homeRef, err := ref.miotRef()
	if err != nil {
		return DeviceList{}, err
	}
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return DeviceList{}, err
	}
	response, err := clients.home.ListDevices(ctx, homeRef, home.WithDeviceLimit(limit))
	if err != nil {
		return DeviceList{}, err
	}

	result := DeviceList{
		Devices: []Device{},
		HasMore: response.HasMore,
		Cursor:  response.MaxDeviceID,
	}
	if response.DeviceInfo != nil {
		for _, item := range *response.DeviceInfo {
			result.Devices = append(result.Devices, deviceFromMIoT(item))
		}
	}
	return result, nil
}

func (a *App) GetProperties(ctx context.Context, accountID, deviceID string, refs ...attr.PropertyRef) ([]attr.PropertyResult, error) {
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return nil, err
	}
	return clients.device.GetProperties(ctx, deviceID, refs...)
}

// SetProperties accepts explicit IIDs even when a specification is unavailable.
// UIs with specification metadata can first call ValidatePropertyValue.
func (a *App) SetProperties(ctx context.Context, accountID, deviceID string, values ...attr.PropertyValue) ([]attr.PropertyResult, error) {
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return nil, err
	}
	results, err := clients.device.SetProperties(ctx, deviceID, values...)
	return results, writeFailure(err)
}

func (a *App) ExecuteAction(ctx context.Context, accountID, deviceID string, action attr.ActionRequest) (*attr.ActionResult, error) {
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result, err := clients.device.ExecuteAction(ctx, deviceID, action)
	return result, writeFailure(err)
}
