package home

type listHomesRequest struct {
	AppVersion    int  `json:"app_ver"`
	FetchShare    bool `json:"fetch_share"`
	FetchShareDev bool `json:"fetch_share_dev"`
	FG            bool `json:"fg"`
	Limit         int  `json:"limit"`
}

type listDevicesRequest struct {
	HomeOwner int64 `json:"home_owner"`
	HomeID    int64 `json:"home_id"`
	Limit     int   `json:"limit"`
}

type listScenesRequest struct {
	AppVersion int    `json:"app_version"`
	GetType    int    `json:"get_type"`
	HomeID     string `json:"home_id"`
	OwnerUID   string `json:"owner_uid"`
}

type runSceneRequest struct {
	HomeID    string `json:"home_id"`
	OwnerUID  string `json:"owner_uid"`
	SceneID   string `json:"scene_id"`
	PhoneID   string `json:"phone_id"`
	SceneType int    `json:"scene_type"`
}
