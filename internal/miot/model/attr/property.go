package attr

type PropertyRef struct {
	ServiceIID  int
	PropertyIID int
}

type PropertyValue struct {
	ServiceIID  int
	PropertyIID int
	Value       any
}

type ActionRequest struct {
	ServiceIID int
	ActionIID  int
	Input      []any
}

type PropertyResult struct {
	Code        int    `json:"code"`
	DeviceID    string `json:"did"`
	IID         string `json:"iid"`
	ServiceIID  int    `json:"siid"`
	PropertyIID int    `json:"piid"`
	Value       any    `json:"value"`
	UpdateTime  int64  `json:"updateTime"`
	ExeTime     int    `json:"exe_time"`
}

type ActionResult struct {
	Code       int    `json:"code"`
	DeviceID   string `json:"did"`
	IID        string `json:"iid"`
	ServiceIID int    `json:"siid"`
	ActionIID  int    `json:"aiid"`
	Output     []any  `json:"out"`
	ExeTime    int    `json:"exe_time"`
}
