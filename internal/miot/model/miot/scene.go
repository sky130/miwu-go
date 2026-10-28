package miot

type SceneList struct {
	DistanceCeiling int      `json:"distance_ceiling"`
	Scenes          *[]Scene `json:"manual_scene_info_list"`
}

type Scene struct {
	Authed          []string    `json:"authed"`
	CreateTime      string      `json:"create_time"`
	Enable          bool        `json:"enable"`
	FirstTrigger    *string     `json:"first_trigger"`
	IconURL         string      `json:"icon_url"`
	LastEditTime    string      `json:"last_edit_time"`
	Name            string      `json:"name"`
	SceneAction     SceneAction `json:"scene_action"`
	SceneElseAction *string     `json:"scene_else_action"`
	SceneID         string      `json:"scene_id"`
	Tags            Tags        `json:"tags"`
	TemplateID      string      `json:"template_id"`
	Type            int         `json:"type"`
	Version         int         `json:"version"`
}

type Tags struct {
	Source *string `json:"source"`
	StID   *string `json:"st_id"`
}

type SceneAction struct {
	Actions []ActionEntry `json:"actions"`
	Mode    int           `json:"mode"`
}

type ActionEntry struct {
	DeviceGroupID int    `json:"device_group_id"`
	From          int    `json:"from"`
	GroupID       int    `json:"group_id"`
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Order         int    `json:"order"`
	Payload       string `json:"payload"`
	ProtocolType  int    `json:"protocol_type"`
	SaID          int    `json:"sa_id"`
	StdSaID       string `json:"std_sa_id"`
	Type          int    `json:"type"`
}
