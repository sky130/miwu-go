package attr

import (
	"bytes"
	"encoding/json"
	"errors"
)

// An omitted item code is not the same as an explicitly successful item.
func requiredCode(data []byte) error {
	var header struct {
		Code *int `json:"code"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.Code == nil {
		return errors.New("missing operation code")
	}
	return nil
}

func (p *PropertyResult) UnmarshalJSON(data []byte) error {
	if err := requiredCode(data); err != nil {
		return err
	}
	type plain PropertyResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode((*plain)(p))
}

func (a *ActionResult) UnmarshalJSON(data []byte) error {
	if err := requiredCode(data); err != nil {
		return err
	}
	type plain ActionResult
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode((*plain)(a))
}
