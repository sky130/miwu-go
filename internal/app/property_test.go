package app_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"miwu-go/internal/app"
	"miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
)

func decodePropertyValue(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var value any
	err := decoder.Decode(&value)
	return value, err
}

func TestPropertyValuePrecisionAndBounds(t *testing.T) {
	for _, tt := range []struct {
		format, value string
		valid         bool
	}{
		{"uint64", "18446744073709551615", true}, {"uint64", "18446744073709551616", false},
		{"int8", "-128", true}, {"int8", "-129", false}, {"uint8", "-1", false},
		{"bool", "false", true}, {"bool", "0", false}, {"string", `"ok"`, true}, {"string", "1", false},
	} {
		v, err := decodePropertyValue(tt.value)
		if err != nil {
			t.Fatal(err)
		}
		err = app.ValidatePropertyValue(&model.Property{Format: tt.format}, v)
		if (err == nil) != tt.valid {
			t.Fatalf("%s %s: %v", tt.format, tt.value, err)
		}
		if !tt.valid && !errors.Is(err, miot.ErrInvalidArgument) {
			t.Fatalf("validation lost its error category: %v", err)
		}
	}
	rangeValues := []any{json.Number("0"), json.Number("1"), json.Number("0.1")}
	p := &model.Property{Format: "float", ValueRange: &rangeValues}
	if err := app.ValidatePropertyValue(p, json.Number("0.3")); err != nil {
		t.Fatal("decimal step rejected")
	}
	if err := app.ValidatePropertyValue(p, json.Number("0.35")); err == nil {
		t.Fatal("invalid decimal step accepted")
	}
}

func TestActionInputsPreserveSpecificationOrder(t *testing.T) {
	properties := []model.Property{{IID: 1, Format: "bool"}, {IID: 2, Format: "int8"}}
	service := model.Service{IID: 2, Properties: &properties}
	inputs, err := app.ActionInputs(service, model.Action{In: []any{json.Number("2"), json.Number("1"), json.Number("3")}})
	if err != nil || len(inputs) != 3 || inputs[0].IID != 2 || inputs[1].IID != 1 || inputs[2] != nil {
		t.Fatalf("input order or metadata fallback lost: %+v, %v", inputs, err)
	}
	for _, raw := range []any{json.Number("1.5"), json.Number("-1"), json.Number("0"), json.Number("18446744073709551616"), "1"} {
		if _, err := app.ActionInputs(service, model.Action{In: []any{raw}}); !errors.Is(err, miot.ErrInvalidResponse) {
			t.Fatalf("invalid input IID accepted: %v, %v", raw, err)
		}
	}
}
