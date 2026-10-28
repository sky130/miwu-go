package app

import (
	"context"
	"encoding/json"
	"fmt"

	"miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/spec"
	"miwu-go/internal/miot/urn"
)

type URNInfo struct {
	URN           string  `json:"urn"`
	Namespace     string  `json:"namespace"`
	Type          string  `json:"type"`
	Name          string  `json:"name"`
	Value         string  `json:"value"`
	VendorProduct *string `json:"vendor_product,omitempty"`
	Version       *int    `json:"version,omitempty"`
	Revision      *string `json:"revision,omitempty"`
}

// ParseURN is local and does not require constructing an App or opening storage.
func ParseURN(value string) (URNInfo, error) {
	parsed, err := urn.Parse(value)
	if err != nil {
		return URNInfo{}, fmt.Errorf("%w: invalid URN: %v", miot.ErrInvalidArgument, err)
	}
	return URNInfo{
		URN:           parsed.String(),
		Namespace:     parsed.Namespace,
		Type:          parsed.Type,
		Name:          parsed.Name,
		Value:         parsed.Value,
		VendorProduct: parsed.VendorProduct,
		Version:       parsed.Version,
		Revision:      parsed.Revision,
	}, nil
}

func (a *App) GetSpecification(ctx context.Context, value, language string) (*model.Specification, error) {
	if _, err := ParseURN(value); err != nil {
		return nil, err
	}
	if language == "" {
		return a.specification.Get(ctx, value)
	}
	return a.specification.GetWithLanguage(ctx, value, spec.LanguageCode(language))
}

func (a *App) DeviceSpecification(ctx context.Context, device Device, language string) (*model.Specification, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if device.SpecURN == nil || *device.SpecURN == "" {
		return nil, nil
	}
	return a.GetSpecification(ctx, *device.SpecURN, language)
}

func (a *App) SpecificationLanguages(ctx context.Context, value string) (json.RawMessage, error) {
	if _, err := ParseURN(value); err != nil {
		return nil, err
	}
	body, err := a.specification.Languages(ctx, value)
	if err != nil {
		return nil, err
	}
	if body == nil || !json.Valid([]byte(*body)) {
		return nil, fmt.Errorf("%w: invalid language JSON", miot.ErrInvalidResponse)
	}
	return json.RawMessage(*body), nil
}

func (a *App) IconURL(ctx context.Context, modelName string) (string, error) {
	icon, err := a.specification.IconURL(ctx, modelName)
	if err != nil {
		return "", err
	}
	if icon == nil {
		return "", fmt.Errorf("%w: missing icon URL", miot.ErrInvalidResponse)
	}
	return *icon, nil
}

// ActionInputs resolves the specification's input order. A nil property keeps
// the existing explicit-JSON fallback when input metadata is incomplete.
func ActionInputs(service model.Service, action model.Action) ([]*model.Property, error) {
	inputs := make([]*model.Property, 0, len(action.In))
	for _, rawIID := range action.In {
		propertyIID, ok := numeric(rawIID)
		if !ok || !propertyIID.IsInt() || !propertyIID.Num().IsInt64() || propertyIID.Sign() <= 0 {
			return nil, fmt.Errorf("%w: invalid action input specification", miot.ErrInvalidResponse)
		}
		var property *model.Property
		if service.Properties != nil {
			for _, candidate := range *service.Properties {
				if int64(candidate.IID) == propertyIID.Num().Int64() {
					copy := candidate
					property = &copy
					break
				}
			}
		}
		inputs = append(inputs, property)
	}
	return inputs, nil
}
