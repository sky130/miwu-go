package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"miwu-go/internal/app"
	"miwu-go/internal/miot/model"
	"miwu-go/internal/miot/model/attr"

	"github.com/spf13/cobra"
)

func parseJSON(raw string) (any, error) {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.UseNumber()
	var v any
	if err := decoder.Decode(&v); err != nil {
		return nil, usage("invalid JSON value: %v", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, usage("expected exactly one JSON value")
	}
	return v, nil
}

func propertyRef(raw string) (attr.PropertyRef, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return attr.PropertyRef{}, usage("property must be SIID.PIID")
	}
	siid, e1 := strconv.Atoi(parts[0])
	piid, e2 := strconv.Atoi(parts[1])
	if e1 != nil || e2 != nil || siid <= 0 || piid <= 0 {
		return attr.PropertyRef{}, usage("property IDs must be positive integers")
	}
	return attr.PropertyRef{ServiceIID: siid, PropertyIID: piid}, nil
}

func (s *state) addDevices(root *cobra.Command) {
	parent := &cobra.Command{Use: "device", Short: "Browse and control devices", Args: cobra.NoArgs}
	root.AddCommand(parent)
	var limit int
	cmd := s.add(parent, "list", "List devices in a home", func(ctx context.Context) (result, error) {
		if limit <= 0 {
			return result{}, usage("--limit must be positive")
		}
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		ref, err := s.home(ctx, application, accountID)
		if err != nil {
			return result{}, err
		}
		list, err := application.ListDevices(ctx, accountID, ref, limit)
		if err != nil {
			return result{}, err
		}
		rows := make([][]string, 0, len(list.Devices))
		for _, device := range list.Devices {
			rows = append(rows, []string{device.ID, device.Name, device.Model, strconv.FormatBool(device.Online)})
		}
		return result{
			Data:    list,
			Columns: []string{"ID", "NAME", "MODEL", "ONLINE"},
			Rows:    rows,
			Note:    truncated(list.HasMore),
		}, nil
	})
	cmd.Flags().IntVar(&limit, "limit", app.DefaultDeviceLimit, "Maximum returned devices")
	var showID string
	cmd = s.add(parent, "show", "Show device details", func(ctx context.Context) (result, error) {
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		d, err := s.target(ctx, application, accountID, showID, true)
		if err != nil {
			return result{}, err
		}
		return result{Data: d}, nil
	})
	cmd.Flags().StringVar(&showID, "did", "", "Device ID")
	s.addProperties(parent, false)
	s.addProperties(parent, true)
	s.addAction(parent)
}

type propertyChoice struct {
	ref      attr.PropertyRef
	property model.Property
	label    string
}

func (s *state) specification(ctx context.Context, d app.Device) (*model.Specification, error) {
	if d.SpecURN == nil || *d.SpecURN == "" {
		return nil, nil
	}
	a, err := s.application()
	if err != nil {
		return nil, err
	}
	v, err := a.DeviceSpecification(ctx, d, "zh_cn")
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fmt.Fprintln(s.options.Err, "Specification unavailable; enter numeric IDs and JSON values instead.")
		return nil, nil
	}
	return v, nil
}

func (s *state) selectProperty(ctx context.Context, d app.Device, write bool) (attr.PropertyRef, *model.Property, error) {
	v, err := s.specification(ctx, d)
	if err != nil {
		return attr.PropertyRef{}, nil, err
	}
	if v == nil {
		raw, e := s.line(ctx, "Property (SIID.PIID)")
		if e != nil {
			return attr.PropertyRef{}, nil, e
		}
		ref, e := propertyRef(raw)
		return ref, nil, e
	}
	permission := "read"
	if write {
		permission = "write"
	}
	choices := []propertyChoice{}
	labels := []string{}
	for _, service := range v.Services {
		if service.Properties == nil {
			continue
		}
		for _, p := range *service.Properties {
			if !slices.Contains(p.Access, permission) {
				continue
			}
			ref := attr.PropertyRef{ServiceIID: service.IID, PropertyIID: p.IID}
			label := fmt.Sprintf("%s / %s (%d.%d, %s)", service.Translation, p.Translation, ref.ServiceIID, ref.PropertyIID, p.Format)
			choices = append(choices, propertyChoice{ref, p, label})
			labels = append(labels, label)
		}
	}
	index, err := s.choose(ctx, "Select "+permission+" property", labels)
	if err != nil {
		return attr.PropertyRef{}, nil, err
	}
	return choices[index].ref, &choices[index].property, nil
}

func (s *state) propertyValue(ctx context.Context, p *model.Property) (any, error) {
	if p != nil {
		fmt.Fprintf(s.options.Err, "Format: %s", p.Format)
		if p.Unit != nil {
			fmt.Fprintf(s.options.Err, "; unit: %s", *p.Unit)
		}
		fmt.Fprintln(s.options.Err)
		if p.ValueList != nil {
			for _, v := range *p.ValueList {
				fmt.Fprintf(s.options.Err, "  %d = %s\n", v.Value, joinColumns([]string{v.Translation}))
			}
		}
		if p.ValueRange != nil {
			fmt.Fprintf(s.options.Err, "Range [min, max, step]: %v\n", *p.ValueRange)
		}
	}
	for {
		raw, err := s.line(ctx, "Value as JSON (quote strings)")
		if err != nil {
			return nil, err
		}
		v, err := parseJSON(raw)
		if err == nil {
			err = app.ValidatePropertyValue(p, v)
		}
		if err == nil {
			return v, nil
		}
		fmt.Fprintln(s.options.Err, err)
	}
}

func propertyResults(values []attr.PropertyResult) result {
	rows := [][]string{}
	for _, v := range values {
		raw, _ := json.Marshal(v.Value)
		rows = append(rows, []string{v.DeviceID, fmt.Sprintf("%d.%d", v.ServiceIID, v.PropertyIID), strconv.Itoa(v.Code), string(raw)})
	}
	return result{Data: values, Columns: []string{"DEVICE", "PROPERTY", "CODE", "VALUE"}, Rows: rows}
}

func (s *state) addProperties(parent *cobra.Command, write bool) {
	var did string
	var raw []string
	name := "get"
	flag := "property"
	description := "Read device properties"
	if write {
		name = "set"
		flag = "set"
		description = "Write device properties"
	}
	cmd := s.add(parent, name, description, func(ctx context.Context) (result, error) {
		refs := []attr.PropertyRef{}
		values := []attr.PropertyValue{}
		for _, item := range raw {
			key := item
			var value any
			if write {
				var text string
				var ok bool
				key, text, ok = strings.Cut(item, "=")
				if !ok {
					return result{}, usage("--set must be SIID.PIID=JSON")
				}
				var err error
				value, err = parseJSON(text)
				if err != nil {
					return result{}, err
				}
			}
			ref, err := propertyRef(key)
			if err != nil {
				return result{}, err
			}
			refs = append(refs, ref)
			if write {
				values = append(values, attr.PropertyValue{ServiceIID: ref.ServiceIID, PropertyIID: ref.PropertyIID, Value: value})
			}
		}
		if len(refs) == 0 && !s.interactive() {
			return result{}, usage("--%s is required", flag)
		}
		if did == "" && !s.interactive() {
			return result{}, usage("--did is required")
		}
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		d, err := s.target(ctx, application, accountID, did, len(refs) == 0)
		if err != nil {
			return result{}, err
		}
		if len(refs) == 0 {
			ref, p, e := s.selectProperty(ctx, d, write)
			if e != nil {
				return result{}, e
			}
			refs = append(refs, ref)
			if write {
				value, e := s.propertyValue(ctx, p)
				if e != nil {
					return result{}, e
				}
				values = append(values, attr.PropertyValue{ServiceIID: ref.ServiceIID, PropertyIID: ref.PropertyIID, Value: value})
			}
		}
		if write {
			results, err := application.SetProperties(ctx, accountID, d.ID, values...)
			return propertyResults(results), err
		}
		results, err := application.GetProperties(ctx, accountID, d.ID, refs...)
		return propertyResults(results), err
	})
	cmd.Flags().StringVar(&did, "did", "", "Device ID")
	hint := "Property SIID.PIID (repeatable)"
	if write {
		hint = "Property SIID.PIID=JSON (repeatable)"
	}
	cmd.Flags().StringArrayVar(&raw, flag, nil, hint)
}

func (s *state) addAction(parent *cobra.Command) {
	var did, input string
	var siid, aiid int
	cmd := s.add(parent, "action", "Execute a device action", func(ctx context.Context) (result, error) {
		action := attr.ActionRequest{ServiceIID: siid, ActionIID: aiid}
		var inputSet bool
		if input != "" {
			v, err := parseJSON(input)
			if err != nil {
				return result{}, err
			}
			var ok bool
			action.Input, ok = v.([]any)
			if !ok {
				return result{}, usage("--in must be a JSON array")
			}
			inputSet = true
		}
		if (siid == 0) != (aiid == 0) || siid < 0 || aiid < 0 {
			return result{}, usage("--siid and --aiid must both be positive")
		}
		if (did == "" || siid == 0) && !s.interactive() {
			return result{}, usage("--did, --siid and --aiid are required")
		}
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		d, err := s.target(ctx, application, accountID, did, siid == 0)
		if err != nil {
			return result{}, err
		}
		if siid == 0 {
			v, e := s.specification(ctx, d)
			if e != nil {
				return result{}, e
			}
			if v == nil {
				raw, e := s.line(ctx, "Action (SIID.AIID)")
				if e != nil {
					return result{}, e
				}
				ref, e := propertyRef(raw)
				if e != nil {
					return result{}, e
				}
				action.ServiceIID = ref.ServiceIID
				action.ActionIID = ref.PropertyIID
				if !inputSet {
					raw, e := s.line(ctx, "Action input as JSON array ([] for none)")
					if e != nil {
						return result{}, e
					}
					value, e := parseJSON(raw)
					if e != nil {
						return result{}, e
					}
					var ok bool
					action.Input, ok = value.([]any)
					if !ok {
						return result{}, usage("action input must be an array")
					}
				}
			} else {
				type choice struct {
					service model.Service
					action  model.Action
				}
				choices := []choice{}
				labels := []string{}
				for _, service := range v.Services {
					if service.Actions == nil {
						continue
					}
					for _, a := range *service.Actions {
						choices = append(choices, choice{service, a})
						labels = append(labels, fmt.Sprintf("%s / %s (%d.%d)", service.Translation, a.Translation, service.IID, a.IID))
					}
				}
				index, e := s.choose(ctx, "Select action", labels)
				if e != nil {
					return result{}, e
				}
				chosen := choices[index]
				action.ServiceIID = chosen.service.IID
				action.ActionIID = chosen.action.IID
				if !inputSet {
					properties, err := app.ActionInputs(chosen.service, chosen.action)
					if err != nil {
						return result{}, err
					}
					for _, property := range properties {
						if property != nil {
							fmt.Fprintln(s.options.Err, "Input:", joinColumns([]string{property.Translation}))
						}
						value, e := s.propertyValue(ctx, property)
						if e != nil {
							return result{}, e
						}
						action.Input = append(action.Input, value)
					}
				}
			}
		}
		value, err := application.ExecuteAction(ctx, accountID, d.ID, action)
		if err != nil {
			return result{}, err
		}
		return result{Data: value}, nil
	})
	cmd.Flags().StringVar(&did, "did", "", "Device ID")
	cmd.Flags().IntVar(&siid, "siid", 0, "Service IID")
	cmd.Flags().IntVar(&aiid, "aiid", 0, "Action IID")
	cmd.Flags().StringVar(&input, "in", "", "JSON array of action inputs")
}
