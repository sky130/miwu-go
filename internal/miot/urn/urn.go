package urn

import (
	"fmt"
	"strconv"
	"strings"
)

var validTypes = map[string]struct{}{
	"template": {},
	"property": {},
	"action":   {},
	"event":    {},
	"service":  {},
	"device":   {},
}

type URN struct {
	Namespace     string
	Type          string
	Name          string
	Value         string
	VendorProduct *string
	Version       *int
	Revision      *string
}

func Parse(s string) (*URN, error) {
	parts := strings.Split(s, ":")

	if len(parts) < 5 || len(parts) > 8 {
		return nil, fmt.Errorf("urn: expected 5-8 segments, got %d: %q", len(parts), s)
	}

	if parts[0] != "urn" {
		return nil, fmt.Errorf("urn: missing 'urn' prefix: %q", s)
	}

	required := [4]string{"namespace", "type", "name", "value"}
	for i := 0; i < 4; i++ {
		if parts[i+1] == "" {
			return nil, fmt.Errorf("urn: %s must not be empty: %q", required[i], s)
		}
	}

	if !IsValidType(parts[2]) {
		return nil, fmt.Errorf("urn: invalid type %q: %q", parts[2], s)
	}

	u := &URN{
		Namespace: parts[1],
		Type:      parts[2],
		Name:      parts[3],
		Value:     parts[4],
	}

	if len(parts) > 5 {
		if parts[5] == "" {
			return nil, fmt.Errorf("urn: vendor-product must not be empty: %q", s)
		}
		u.VendorProduct = &parts[5]
	}

	if len(parts) > 6 {
		v, err := parseVersion(parts[6])
		if err != nil {
			return nil, fmt.Errorf("urn: %w: %q", err, s)
		}
		u.Version = &v
	}

	if len(parts) > 7 {
		if parts[7] == "" {
			return nil, fmt.Errorf("urn: revision must not be empty: %q", s)
		}
		u.Revision = &parts[7]
	}

	return u, nil
}

func IsValidType(t string) bool {
	_, ok := validTypes[t]
	return ok
}

func (u *URN) String() string {
	if u == nil {
		return "<nil>"
	}

	var sb strings.Builder
	sb.WriteString("urn:")
	sb.WriteString(u.Namespace)
	sb.WriteByte(':')
	sb.WriteString(u.Type)
	sb.WriteByte(':')
	sb.WriteString(u.Name)
	sb.WriteByte(':')
	sb.WriteString(u.Value)

	if u.VendorProduct != nil {
		sb.WriteByte(':')
		sb.WriteString(*u.VendorProduct)
	}
	if u.Version != nil {
		sb.WriteByte(':')
		sb.WriteString(strconv.Itoa(*u.Version))
	}
	if u.Revision != nil {
		sb.WriteByte(':')
		sb.WriteString(*u.Revision)
	}

	return sb.String()
}

func (u *URN) Validate() error {
	if u == nil {
		return fmt.Errorf("urn: nil URN")
	}
	if u.Namespace == "" {
		return fmt.Errorf("urn: namespace must not be empty")
	}
	if !IsValidType(u.Type) {
		return fmt.Errorf("urn: invalid type %q", u.Type)
	}
	if u.Name == "" {
		return fmt.Errorf("urn: name must not be empty")
	}
	if u.Value == "" {
		return fmt.Errorf("urn: value must not be empty")
	}
	if u.VendorProduct != nil && *u.VendorProduct == "" {
		return fmt.Errorf("urn: vendor-product must not be empty")
	}
	if u.Version != nil && *u.Version < 1 {
		return fmt.Errorf("urn: version must be >= 1, got %d", *u.Version)
	}
	if u.Revision != nil && *u.Revision == "" {
		return fmt.Errorf("urn: revision must not be empty")
	}
	return nil
}

func (u *URN) MarshalText() ([]byte, error) {
	if u == nil {
		return []byte("<nil>"), nil
	}
	return []byte(u.String()), nil
}

func (u *URN) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*u = *parsed
	return nil
}

func (u *URN) Equal(other *URN) bool {
	if u == nil || other == nil {
		return u == other
	}
	return u.Namespace == other.Namespace &&
		u.Type == other.Type &&
		u.Name == other.Name &&
		u.Value == other.Value &&
		equalStringPtr(u.VendorProduct, other.VendorProduct) &&
		equalIntPtr(u.Version, other.Version) &&
		equalStringPtr(u.Revision, other.Revision)
}

func parseVersion(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("version must be a positive integer, got %q", s)
	}
	if v < 1 {
		return 0, fmt.Errorf("version must be positive, got %q", s)
	}
	if strconv.Itoa(v) != s {
		return 0, fmt.Errorf("version has non-canonical form %q", s)
	}
	return v, nil
}

func equalStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
