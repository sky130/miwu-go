// Package auth implements isolated Xiaomi account login sessions.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/crypto"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/user"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrChallengeRequired   = errors.New("additional verification required; use QR login")
	ErrQRCodeExpired       = errors.New("QR login expired; generate a new code")
	ErrServiceTokenMissing = errors.New("service token missing")
)

type Endpoints struct {
	QRCode, ServiceLogin, PasswordLogin string
}

func DefaultEndpoints() Endpoints {
	return Endpoints{
		QRCode:        "https://account.xiaomi.com/longPolling/loginUrl",
		ServiceLogin:  "https://account.xiaomi.com/pass/serviceLogin?sid=xiaomiio&_json=true",
		PasswordLogin: "https://account.xiaomi.com/pass/serviceLoginAuth2",
	}
}

type Client struct {
	http      http.Client
	endpoints Endpoints
}

func NewClient(client http.Client) *Client {
	return &Client{http: client, endpoints: DefaultEndpoints()}
}

// WithEndpoints returns a separate client, useful for protocol tests.
func (c *Client) WithEndpoints(e Endpoints) *Client {
	copy := *c
	copy.endpoints = e
	return &copy
}

type session struct {
	http     http.Client
	deviceID string
}

func (c *Client) newSession() (*session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	id, err := crypto.GetRandomDeviceID()
	if err != nil {
		return nil, err
	}
	hc := c.http
	hc.Jar = jar
	hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("too many authentication redirects")
		}
		if req.URL.Scheme != "https" && via[0].URL.Scheme == "https" {
			return errors.New("authentication redirect downgraded HTTPS")
		}
		return nil
	}
	s := &session{http: hc, deviceID: id}
	s.seed(c.endpoints.ServiceLogin, []*http.Cookie{{Name: "deviceId", Value: id, Path: "/"}})
	return s, nil
}

func (s *session) seed(rawURL string, cookies []*http.Cookie) {
	u, err := url.Parse(rawURL)
	if err == nil {
		s.http.Jar.SetCookies(u, cookies)
	}
}

func (s *session) request(ctx context.Context, rawURL string, form url.Values) ([]byte, *http.Response, error) {
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, nil, errors.New("invalid authentication URL")
	}
	req.Header.Set("User-Agent", "APP/com.xiaomi.mihome APPV/6.0.103")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := s.http.Do(req)
	if err != nil {
		// url.Error includes the authentication URL, which may contain secrets.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, nil, fmt.Errorf("authentication request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, nil, &m.HTTPError{Status: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20+1))
	if err != nil {
		return nil, nil, fmt.Errorf("read authentication response: %w", err)
	}
	if len(b) > 4<<20 {
		return nil, nil, fmt.Errorf("%w: authentication response too large", m.ErrInvalidResponse)
	}
	return bytes.TrimPrefix(bytes.TrimSpace(b), []byte("&&&START&&&")), resp, nil
}

type response struct {
	Code             *int   `json:"code"`
	Description      string `json:"description"`
	Timeout          int    `json:"timeout"`
	LoginURL         string `json:"loginUrl"`
	LP               string `json:"lp"`
	Location         string `json:"location"`
	Security         string `json:"ssecurity"`
	QS               string `json:"qs"`
	SID              string `json:"sid"`
	Sign             string `json:"_sign"`
	Callback         string `json:"callback"`
	UserID           int64  `json:"userId"`
	ClientUserID     string `json:"cUserId"`
	PassToken        string `json:"passToken"`
	Nonce            int64  `json:"nonce"`
	PasswordSecurity string `json:"psecurity"`
	CaptchaURL       string `json:"captchaUrl"`
	NotificationURL  string `json:"notificationUrl"`
}

func decode(b []byte) (response, error) {
	var r response
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("%w: malformed authentication JSON", m.ErrInvalidResponse)
	}
	if r.Code == nil {
		return r, fmt.Errorf("%w: missing authentication code", m.ErrInvalidResponse)
	}
	return r, nil
}

func check(r response) error {
	if r.CaptchaURL != "" || r.NotificationURL != "" {
		return fmt.Errorf("%w: %w", ErrChallengeRequired, &model.BusinessError{Code: *r.Code, Message: "verification required"})
	}
	switch *r.Code {
	case 0:
		return nil
	case 70002, 70016, 20003:
		return fmt.Errorf("%w: %w", ErrInvalidCredentials, &model.BusinessError{Code: *r.Code, Message: "invalid credentials"})
	case 81003, 87001:
		return fmt.Errorf("%w: %w", ErrChallengeRequired, &model.BusinessError{Code: *r.Code, Message: "verification required"})
	default:
		return &model.BusinessError{Code: *r.Code, Message: "authentication rejected"}
	}
}

// QRCode owns its session; multiple pending codes never share cookies.
type QRCode struct {
	Data      string
	pollURL   string
	session   *session
	expiresAt time.Time
}

func (c *Client) GenerateQRCode(ctx context.Context) (*QRCode, error) {
	s, err := c.newSession()
	if err != nil {
		return nil, err
	}
	// Preserve the legacy query encoding verified by the original implementation.
	callback := fmt.Sprintf("https://sts.api.io.mi.com/sts\nsid=xiaomiio\nserviceParam=\n_locale=zh_CN\n_dc=%d", time.Now().UnixMilli())
	raw := "_qrsize=240\nqs=?sid=xiaomiio\ncallback=" + callback
	b, _, err := s.request(ctx, c.endpoints.QRCode+"?"+url.QueryEscape(raw), nil)
	if err != nil {
		return nil, err
	}
	r, err := decode(b)
	if err != nil {
		return nil, err
	}
	if err = check(r); err != nil {
		return nil, err
	}
	if r.LoginURL == "" || r.LP == "" {
		return nil, fmt.Errorf("%w: missing QR login or polling URL", m.ErrInvalidResponse)
	}
	code := &QRCode{Data: r.LoginURL, pollURL: r.LP, session: s}
	if r.Timeout > 0 {
		code.expiresAt = time.Now().Add(time.Duration(r.Timeout) * time.Second)
	}
	return code, nil
}

func (c *Client) LoginWithQRCode(ctx context.Context, code *QRCode) (user.Credentials, error) {
	var zero user.Credentials
	if code == nil || code.session == nil {
		return zero, fmt.Errorf("%w: QR session required", m.ErrInvalidArgument)
	}
	if !code.expiresAt.IsZero() && time.Now().After(code.expiresAt) {
		return zero, ErrQRCodeExpired
	}
	b, resp, err := code.session.request(ctx, code.pollURL, nil)
	if err != nil {
		return zero, err
	}
	r, err := decode(b)
	if err != nil {
		return zero, err
	}
	if err = check(r); err != nil {
		return zero, err
	}
	// Explicitly transfer only account identity cookies if polling uses a different host.
	var accountCookies []*http.Cookie
	for _, v := range resp.Cookies() {
		switch v.Name {
		case "userId", "cUserId", "passToken":
			accountCookies = append(accountCookies, &http.Cookie{Name: v.Name, Value: v.Value, Path: "/"})
		}
	}
	code.session.seed(c.endpoints.ServiceLogin, accountCookies)
	b, _, err = code.session.request(ctx, c.endpoints.ServiceLogin, nil)
	if err != nil {
		return zero, err
	}
	service, err := decode(b)
	if err != nil {
		return zero, err
	}
	if err = check(service); err != nil {
		return zero, err
	}
	r.Location = service.Location
	r.Security = service.Security
	return code.session.complete(ctx, r)
}

func (c *Client) LoginWithPassword(ctx context.Context, username, password string) (user.Credentials, error) {
	var zero user.Credentials
	if username == "" || password == "" {
		return zero, fmt.Errorf("%w: username and password required", m.ErrInvalidArgument)
	}
	s, err := c.newSession()
	if err != nil {
		return zero, err
	}
	b, _, err := s.request(ctx, c.endpoints.ServiceLogin, nil)
	if err != nil {
		return zero, err
	}
	location, err := decode(b)
	if err != nil {
		return zero, err
	}
	// The initial unauthenticated page can legitimately request a password (70016).
	if location.CaptchaURL != "" || location.NotificationURL != "" {
		return zero, check(location)
	}
	if *location.Code != 70016 {
		if err = check(location); err != nil {
			return zero, err
		}
	}
	if location.Sign == "" {
		return zero, fmt.Errorf("%w: missing password login signature", m.ErrInvalidResponse)
	}
	form := url.Values{"qs": {location.QS}, "sid": {location.SID}, "_sign": {location.Sign}, "callback": {location.Callback}, "user": {username}, "hash": {crypto.MD5(password)}, "_json": {"true"}}
	b, _, err = s.request(ctx, c.endpoints.PasswordLogin, form)
	if err != nil {
		return zero, err
	}
	r, err := decode(b)
	if err != nil {
		return zero, err
	}
	if err = check(r); err != nil {
		return zero, err
	}
	return s.complete(ctx, r)
}

func (s *session) token(ctx context.Context, location string) (string, error) {
	if location == "" {
		return "", fmt.Errorf("%w: missing token exchange URL", m.ErrInvalidResponse)
	}
	_, resp, err := s.request(ctx, location, nil)
	if err != nil {
		return "", err
	}
	for _, v := range resp.Cookies() {
		if v.Name == "serviceToken" && v.Value != "" {
			return v.Value, nil
		}
	}
	locations := []string{location}
	if resp.Request != nil {
		locations = append(locations, resp.Request.URL.String())
	}
	for _, u := range locations {
		parsed, err := url.Parse(u)
		if err != nil {
			continue
		}
		for _, v := range s.http.Jar.Cookies(parsed) {
			if v.Name == "serviceToken" && v.Value != "" {
				return v.Value, nil
			}
		}
	}
	return "", ErrServiceTokenMissing
}

func (s *session) complete(ctx context.Context, r response) (user.Credentials, error) {
	var zero user.Credentials
	if r.UserID <= 0 || r.Security == "" || r.PassToken == "" {
		return zero, fmt.Errorf("%w: incomplete login credentials", m.ErrInvalidResponse)
	}
	token, err := s.token(ctx, r.Location)
	if err != nil {
		return zero, err
	}
	return user.Credentials{Identity: user.Identity{UserID: strconv.FormatInt(r.UserID, 10), ClientUserID: r.ClientUserID, DeviceID: s.deviceID}, Session: user.Session{Nonce: r.Nonce, ServiceSecurity: r.Security, PasswordSecurity: r.PasswordSecurity, PassToken: r.PassToken, ServiceToken: token}}, nil
}

func (c *Client) Refresh(ctx context.Context, credentials user.Credentials) (user.Credentials, error) {
	if credentials.UserID == "" || credentials.PassToken == "" {
		return user.Credentials{}, m.ErrAuthRequired
	}
	s, err := c.newSession()
	if err != nil {
		return user.Credentials{}, err
	}
	s.seed(c.endpoints.ServiceLogin, []*http.Cookie{{Name: "deviceId", Value: credentials.DeviceID, Path: "/"}, {Name: "userId", Value: credentials.UserID, Path: "/"}, {Name: "cUserId", Value: credentials.ClientUserID, Path: "/"}, {Name: "passToken", Value: credentials.PassToken, Path: "/"}})
	b, _, err := s.request(ctx, c.endpoints.ServiceLogin, nil)
	if err != nil {
		return user.Credentials{}, err
	}
	r, err := decode(b)
	if err != nil {
		return user.Credentials{}, err
	}
	if err = check(r); err != nil {
		return user.Credentials{}, err
	}
	if r.Security == "" {
		return user.Credentials{}, fmt.Errorf("%w: missing refreshed security", m.ErrInvalidResponse)
	}
	token, err := s.token(ctx, r.Location)
	if err != nil {
		return user.Credentials{}, err
	}
	credentials.ServiceToken = token
	credentials.ServiceSecurity = r.Security
	return credentials, nil
}
