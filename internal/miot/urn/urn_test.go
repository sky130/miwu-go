package urn

import (
	"encoding/json"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	cases := []string{
		"urn:miot-spec-v2:device:light:0000A001:xiaomi-bed3:1:0000C818",
		"urn:miot-spec-v2:device:switch:0000A003:boade-sw1a01:1:0000C808",
		"urn:miot-spec-v2:device:light:0000A001:ylzc-yldd:1:0000C802",
		"urn:miot-spec-v2:device:switch:0000A003:xiaomi-2pro:1:0000C808",
	}
	for _, s := range cases {
		u, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", s, err)
		}
		if got := u.String(); got != s {
			t.Errorf("round trip: %q -> %q", s, got)
		}
		if err := u.Validate(); err != nil {
			t.Errorf("Validate(%q): %v", s, err)
		}
	}
}

func TestParseRealMIoTFields(t *testing.T) {
	const s = "urn:miot-spec-v2:device:light:0000A001:xiaomi-bed3:1:0000C818"
	u, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	if u.Namespace != "miot-spec-v2" {
		t.Errorf("Namespace = %q", u.Namespace)
	}
	if u.Type != "device" {
		t.Errorf("Type = %q", u.Type)
	}
	if u.Name != "light" {
		t.Errorf("Name = %q", u.Name)
	}
	if u.Value != "0000A001" {
		t.Errorf("Value = %q", u.Value)
	}
	if u.VendorProduct == nil || *u.VendorProduct != "xiaomi-bed3" {
		t.Errorf("VendorProduct = %v", u.VendorProduct)
	}
	if u.Version == nil || *u.Version != 1 {
		t.Errorf("Version = %v", u.Version)
	}
	if u.Revision == nil || *u.Revision != "0000C818" {
		t.Errorf("Revision = %v", u.Revision)
	}
}

func TestEqual(t *testing.T) {
	a, _ := Parse("urn:miot-spec-v2:device:light:0000A001:xiaomi-bed3:1:0000C818")
	b, _ := Parse("urn:miot-spec-v2:device:light:0000A001:xiaomi-bed3:1:0000C818")
	c, _ := Parse("urn:miot-spec-v2:device:light:0000A001:xiaomi-bed3:1")

	if !a.Equal(b) {
		t.Error("a should equal b")
	}
	if a.Equal(c) {
		t.Error("a should not equal c")
	}
	if !(*URN)(nil).Equal(nil) {
		t.Error("nil should equal nil")
	}
}

func TestJSONText(t *testing.T) {
	const s = "urn:miot-spec-v2:device:light:0000A001:xiaomi-bed3:1:0000C818"
	u, err := Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	want := `"` + s + `"`
	if string(data) != want {
		t.Errorf("json.Marshal = %s, want %s", data, want)
	}

	var back URN
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !u.Equal(&back) {
		t.Errorf("round trip via JSON: %+v vs %+v", u, back)
	}
}
