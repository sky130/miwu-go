package spec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	m "miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
)

func (c *Client) Get(ctx context.Context, urn string) (attr *model.Specification, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(urn) == "" {
		return nil, fmt.Errorf("%w: URN required", m.ErrInvalidArgument)
	}
	q := url.Values{}
	q.Set("type", urn)

	attr, err = specGet[model.Specification](ctx, c, "/miot-spec-v2/instance?"+q.Encode())
	if err == nil {
		if attr == nil || attr.Services == nil {
			return nil, fmt.Errorf("%w: missing services", m.ErrInvalidResponse)
		}
		initTranslations(attr)
	}
	return attr, err
}

func (c *Client) Languages(ctx context.Context, urn string) (*string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(urn) == "" {
		return nil, fmt.Errorf("%w: URN required", m.ErrInvalidArgument)
	}
	q := url.Values{}
	q.Set("urn", urn)

	body, err := specGet[string](ctx, c, "/instance/v2/multiLanguage?"+q.Encode())
	if err != nil {
		return nil, err
	}

	switch res := *body; res {
	case "model not found", "urn is not iot namespace":
		return nil, fmt.Errorf("%w: language document", ErrNotFound)
	}
	return body, err
}

func LanguageMap(language string, languageCode LanguageCode) (*model.LanguageMap, error) {
	mp := new(languageMap)
	err := json.Unmarshal([]byte(language), mp)
	if err != nil {
		return nil, fmt.Errorf("%w: language JSON: %w", m.ErrInvalidResponse, err)
	}
	if m, ok := mp.Data[languageCode]; ok {
		return &m, nil
	}
	return &model.LanguageMap{}, nil
}

func (c *Client) GetWithLanguage(ctx context.Context, urn string, languageCode LanguageCode) (attr *model.Specification, err error) {
	attr, err = c.Get(ctx, urn)
	if err != nil {
		return nil, err
	}

	language, err := c.Languages(ctx, urn)
	if err != nil {
		return nil, err
	}

	lm, err := LanguageMap(*language, languageCode)
	if err != nil {
		return nil, err
	}

	applyTranslations(attr, *lm)
	return attr, nil
}

func applyTranslations(attr *model.Specification, lm model.LanguageMap) {
	for id, desc := range lm {
		s := strings.Split(id, ":")
		t := languageType(len(s) / 2)

		iids, err := parseIID(s, t)
		if err != nil {
			continue
		}

		service, err := findService(attr, iids.siid)
		if err != nil {
			continue
		}

		switch t {
		case languageService:
			service.Translation = desc
		case languagePropertyOrAction:
			switch s[2] {
			case "property":
				property, err := findProperty(service, iids.iid)
				if err == nil {
					property.Translation = desc
				}
			case "action":
				action, err := findAction(service, iids.iid)
				if err == nil {
					action.Translation = desc
				}
			}
		case languageValueList:
			property, err := findProperty(service, iids.iid)
			if err != nil {
				continue
			}
			valueList := property.ValueList
			if valueList == nil || iids.index < 0 || iids.index >= len(*valueList) {
				continue
			}
			(*valueList)[iids.index].Translation = desc
		}
	}
}
