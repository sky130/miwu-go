package device

type paramsContainer[P any] struct {
	Params P `json:"params"`
}

type getPropertyParam struct {
	DeviceID    string `json:"did"`
	ServiceIID  int    `json:"siid"`
	PropertyIID int    `json:"piid"`
}

type setPropertyParam struct {
	DeviceID    string `json:"did"`
	ServiceIID  int    `json:"siid"`
	PropertyIID int    `json:"piid"`
	Value       any    `json:"value"`
}

type actionParam struct {
	DeviceID   string `json:"did"`
	ServiceIID int    `json:"siid"`
	ActionIID  int    `json:"aiid"`
	Input      []any  `json:"in"`
}
