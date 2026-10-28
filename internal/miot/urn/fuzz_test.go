package urn

import "testing"

func FuzzURNRoundTrip(f *testing.F) {
	f.Add("urn:miot-spec-v2:device:light:0000A001:test-lamp:1")
	f.Add("urn:miot-spec-v2:device:light:0000A001:test-lamp:0")
	f.Fuzz(func(t *testing.T, raw string) {
		parsed, err := Parse(raw)
		if err != nil {
			return
		}
		if err = parsed.Validate(); err != nil {
			t.Fatalf("Parse/Validate disagree: %v", err)
		}
		again, err := Parse(parsed.String())
		if err != nil || !parsed.Equal(again) {
			t.Fatal("URN round trip failed")
		}
	})
}
