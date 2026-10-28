package user

type Identity struct {
	UserID       string `json:"userID"`
	ClientUserID string `json:"clientUserID"`
	DeviceID     string `json:"deviceID"`
}

type Session struct {
	Nonce            int64  `json:"nonce"`
	ServiceSecurity  string `json:"serviceSecurity"`
	PasswordSecurity string `json:"passwordSecurity"`
	PassToken        string `json:"passToken"`
	ServiceToken     string `json:"serviceToken"`
}

type Credentials struct {
	Identity
	Session
}

func (Session) String() string {
	return "Session(credentials=redacted)"
}

type Info struct {
	UserID   int64  `json:"userid"`
	Avatar   string `json:"icon"`
	Nickname string `json:"nickname"`
}
