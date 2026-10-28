package device

import (
	"context"
	"fmt"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/attr"
)

type Client struct {
	transport m.Transport
}

func NewClient(transport m.Transport) *Client {
	return &Client{transport: transport}
}

func validate(did string, refs []attr.PropertyRef) error {
	if did == "" || len(refs) == 0 {
		return fmt.Errorf("%w: device ID and properties required", m.ErrInvalidArgument)
	}
	seen := map[attr.PropertyRef]bool{}
	for _, r := range refs {
		if r.ServiceIID <= 0 || r.PropertyIID <= 0 || seen[r] {
			return fmt.Errorf("%w: invalid or duplicate property", m.ErrInvalidArgument)
		}
		seen[r] = true
	}
	return nil
}

func batch(results []attr.PropertyResult, did string, refs []attr.PropertyRef) error {
	if len(results) != len(refs) {
		return fmt.Errorf("%w: incomplete property results", m.ErrInvalidResponse)
	}
	expected := map[attr.PropertyRef]bool{}
	for _, r := range refs {
		expected[r] = true
	}
	var failures []error
	for _, r := range results {
		ref := attr.PropertyRef{ServiceIID: r.ServiceIID, PropertyIID: r.PropertyIID}
		if r.DeviceID != did || !expected[ref] {
			return fmt.Errorf("%w: unexpected property result", m.ErrInvalidResponse)
		}
		delete(expected, ref)
		if r.Code != 0 {
			failures = append(failures, fmt.Errorf("property %d.%d: %w", r.ServiceIID, r.PropertyIID, &model.BusinessError{Code: r.Code, Message: "property rejected"}))
		}
	}
	if len(failures) > 0 {
		return &model.BatchError{Failures: failures}
	}
	return nil
}

func (c *Client) GetProperties(ctx context.Context, did string, refs ...attr.PropertyRef) ([]attr.PropertyResult, error) {
	if err := validate(did, refs); err != nil {
		return nil, err
	}
	params := make([]getPropertyParam, len(refs))
	for i, r := range refs {
		params[i] = getPropertyParam{DeviceID: did, ServiceIID: r.ServiceIID, PropertyIID: r.PropertyIID}
	}
	response, err := m.Call[[]attr.PropertyResult](ctx, c.transport, "miotspec/prop/get", paramsContainer[[]getPropertyParam]{Params: params})
	if err != nil {
		return nil, err
	}
	return response.Result, batch(response.Result, did, refs)
}

func (c *Client) SetProperties(ctx context.Context, did string, values ...attr.PropertyValue) ([]attr.PropertyResult, error) {
	refs := make([]attr.PropertyRef, len(values))
	params := make([]setPropertyParam, len(values))
	for i, v := range values {
		refs[i] = attr.PropertyRef{ServiceIID: v.ServiceIID, PropertyIID: v.PropertyIID}
		params[i] = setPropertyParam{DeviceID: did, ServiceIID: v.ServiceIID, PropertyIID: v.PropertyIID, Value: v.Value}
	}
	if err := validate(did, refs); err != nil {
		return nil, err
	}
	response, err := m.Call[[]attr.PropertyResult](ctx, c.transport, "miotspec/prop/set", paramsContainer[[]setPropertyParam]{Params: params})
	if err != nil {
		return nil, err
	}
	return response.Result, batch(response.Result, did, refs)
}

func (c *Client) ExecuteAction(ctx context.Context, did string, action attr.ActionRequest) (*attr.ActionResult, error) {
	if did == "" || action.ServiceIID <= 0 || action.ActionIID <= 0 {
		return nil, fmt.Errorf("%w: device, service and action IDs required", m.ErrInvalidArgument)
	}
	input := action.Input
	if input == nil {
		input = []any{}
	}
	params := actionParam{DeviceID: did, ServiceIID: action.ServiceIID, ActionIID: action.ActionIID, Input: input}
	response, err := m.Call[*attr.ActionResult](ctx, c.transport, "miotspec/action", paramsContainer[actionParam]{Params: params})
	if err != nil {
		return nil, err
	}
	r := response.Result
	if r == nil || r.DeviceID != did || r.ServiceIID != action.ServiceIID || r.ActionIID != action.ActionIID {
		return nil, fmt.Errorf("%w: missing or mismatched action result", m.ErrInvalidResponse)
	}
	if r.Code != 0 {
		return nil, &model.BusinessError{Code: r.Code, Message: "action rejected"}
	}
	return r, nil
}
