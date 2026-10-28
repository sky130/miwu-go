package miot

import "strconv"

type DeviceList struct {
	HomeInfo    *DeviceHome `json:"home_info"`
	DeviceInfo  *[]Device   `json:"device_info"`
	HasMore     bool        `json:"has_more"`
	MaxDeviceID string      `json:"max_did"`
}

type DeviceHome struct {
	ID    int64        `json:"id"`
	Rooms []DeviceRoom `json:"roomlist"`
}

type DeviceRoom struct {
	ID        int64    `json:"id"`
	DeviceIDs []string `json:"dids"`
}

type Device struct {
	ID          string  `json:"did"`
	Name        string  `json:"name"`
	MAC         string  `json:"mac"`
	Model       string  `json:"model"`
	UID         int64   `json:"uid"`
	IsOnline    bool    `json:"isOnline"`
	SpecType    *string `json:"spec_type"`
	BSSID       string  `json:"bssid"`
	Cnt         *int    `json:"cnt"`
	ComFlag     int     `json:"comFlag"`
	Extra       Extra   `json:"extra"`
	FreqFlag    bool    `json:"freqFlag"`
	HideMode    *int    `json:"hide_mode"`
	LastOnline  *int64  `json:"last_online"`
	Latitude    string  `json:"latitude"`
	LocalIP     *string `json:"localip"`
	Longitude   string  `json:"longitude"`
	OrderTime   int     `json:"orderTime"`
	ParentID    *string `json:"parent_id"`
	PermitLevel int     `json:"permitLevel"`
	PID         int     `json:"pid"`
	RSSI        *int    `json:"rssi"`
	ShowMode    *int    `json:"show_mode"`
	SSID        *string `json:"ssid"`
	Token       string  `json:"token"`
}

type Extra struct {
	FWVersion       *string `json:"fw_version"`
	IsSetPinCode    *int    `json:"isSetPincode"`
	IsSubGroup      *bool   `json:"isSubGroup"`
	MCUVersion      *string `json:"mcu_version"`
	PinCodeType     *int    `json:"pincodeType"`
	Platform        *string `json:"platform"`
	ShowGroupMember *bool   `json:"showGroupMember"`
}

func (d *Device) Equals(other *Device) bool {
	if d == other {
		return true
	}
	if other == nil {
		return false
	}
	return d.ID == other.ID &&
		d.UID == other.UID &&
		d.Model == other.Model &&
		d.IsOnline == other.IsOnline
}

func (d *Device) Key() string {
	return d.ID + "|" + strconv.FormatInt(d.UID, 10) + "|" + d.Model
}
