package spec

import (
	"errors"
	"fmt"

	"miwu-go/internal/miot/model"
)

var ErrNotFound = errors.New("not found")

func findByIID[T any](items []T, iid int, getIID func(*T) int) (*T, bool) {
	for i := range items {
		if getIID(&items[i]) == iid {
			return &items[i], true
		}
	}
	return nil, false
}

func findService(attr *model.Specification, siid int) (*model.Service, error) {
	if s, ok := findByIID(attr.Services, siid, func(x *model.Service) int { return x.IID }); ok {
		return s, nil
	}
	return nil, fmt.Errorf("service not found, siid: %d: %w", siid, ErrNotFound)
}

func findProperty(service *model.Service, piid int) (*model.Property, error) {
	if service.Properties == nil {
		return nil, fmt.Errorf("properties of service(%d) is nil: %w", service.IID, ErrNotFound)
	}
	if p, ok := findByIID(*service.Properties, piid, func(x *model.Property) int { return x.IID }); ok {
		return p, nil
	}
	return nil, fmt.Errorf("property not found, siid: %d, piid: %d: %w", service.IID, piid, ErrNotFound)
}

func findAction(service *model.Service, aiid int) (*model.Action, error) {
	if service.Actions == nil {
		return nil, fmt.Errorf("actions of service(%d) is nil: %w", service.IID, ErrNotFound)
	}
	if a, ok := findByIID(*service.Actions, aiid, func(x *model.Action) int { return x.IID }); ok {
		return a, nil
	}
	return nil, fmt.Errorf("action not found, siid: %d, aiid: %d: %w", service.IID, aiid, ErrNotFound)
}
