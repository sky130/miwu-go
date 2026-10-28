package miot

import "encoding/json"

type HomeRef struct {
	ID       int64
	OwnerUID int64
}

func RefOf(h *Home) HomeRef {
	return HomeRef{ID: h.ID, OwnerUID: h.OwnerUID}
}

type HomeList struct {
	Homes      []Home  `json:"homelist"`
	ShareHomes *[]Home `json:"share_home_list"`
	HasMore    bool    `json:"has_more"`
	MaxID      string  `json:"max_id"`
}

type Home struct {
	ID                  int64           `json:"id,string"`
	Name                string          `json:"name"`
	OwnerUID            int64           `json:"uid"`
	Rooms               []Room          `json:"roomlist"`
	DeviceIDs           []string        `json:"dids"`
	Address             string          `json:"address"`
	Background          string          `json:"background"`
	BSSID               string          `json:"bssid"`
	CarDID              string          `json:"car_did"`
	CityID              int             `json:"city_id"`
	CreateTime          int             `json:"create_time"`
	Icon                string          `json:"icon"`
	Latitude            float64         `json:"latitude"`
	Longitude           float64         `json:"longitude"`
	PermitLevel         int             `json:"permit_level"`
	PopupFlag           int             `json:"popup_flag"`
	PopupTimeStamp      int             `json:"popup_time_stamp"`
	ShareFlag           int             `json:"shareflag"`
	SmartRoomBackground string          `json:"smart_room_background"`
	Status              int             `json:"status"`
	TempDeviceIDs       json.RawMessage `json:"temp_dids"`
}

type Room struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	DeviceIDs  []string `json:"dids"`
	Background string   `json:"background"`
	BSSID      string   `json:"bssid"`
	CreateTime int      `json:"create_time"`
	Icon       string   `json:"icon"`
	ParentID   string   `json:"parentid"`
	ShareFlag  int64    `json:"shareflag"`
}

func (h *Home) IsShareHome() bool {
	return h.ShareFlag != 0
}
