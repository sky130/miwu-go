package app

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"miwu-go/internal/miot"
	"miwu-go/internal/miot/model"
)

func invalidValue(format string, args ...any) error {
	return fmt.Errorf("%w: %s", miot.ErrInvalidArgument, fmt.Sprintf(format, args...))
}

func numeric(v any) (*big.Rat, bool) {
	switch x := v.(type) {
	case json.Number:
		r, ok := new(big.Rat).SetString(string(x))
		return r, ok
	case float64:
		r, ok := new(big.Rat).SetString(strconv.FormatFloat(x, 'g', -1, 64))
		return r, ok
	case int:
		return new(big.Rat).SetInt64(int64(x)), true
	}
	return nil, false
}

// ValidatePropertyValue checks format, bounds, decimal step and enum values.
func ValidatePropertyValue(p *model.Property, v any) error {
	if p == nil {
		return nil
	}
	switch p.Format {
	case "bool":
		if _, ok := v.(bool); !ok {
			return invalidValue("expected a JSON boolean")
		}
	case "string":
		if _, ok := v.(string); !ok {
			return invalidValue("expected a JSON string")
		}
	default:
		n, ok := numeric(v)
		if !ok {
			return invalidValue("expected a JSON number")
		}
		if p.Format != "float" && p.Format != "double" && !n.IsInt() {
			return invalidValue("expected an integer")
		}
		bits := 0
		unsigned := false
		if strings.HasPrefix(p.Format, "uint") {
			unsigned = true
			bits, _ = strconv.Atoi(strings.TrimPrefix(p.Format, "uint"))
		} else if strings.HasPrefix(p.Format, "int") {
			bits, _ = strconv.Atoi(strings.TrimPrefix(p.Format, "int"))
		}
		if bits > 0 && bits <= 64 {
			min := new(big.Int)
			max := new(big.Int)
			if unsigned {
				max.Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
			} else {
				max.Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)), big.NewInt(1))
				min.Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
			}
			if n.Cmp(new(big.Rat).SetInt(min)) < 0 || n.Cmp(new(big.Rat).SetInt(max)) > 0 {
				return invalidValue("value is outside %s bounds", p.Format)
			}
		}
		if p.ValueRange != nil && len(*p.ValueRange) >= 2 {
			values := *p.ValueRange
			lo, okLo := numeric(values[0])
			hi, okHi := numeric(values[1])
			if okLo && okHi {
				if n.Cmp(lo) < 0 || n.Cmp(hi) > 0 {
					return invalidValue("value is outside the specification range")
				}
				if len(values) > 2 {
					step, ok := numeric(values[2])
					if ok && step.Sign() > 0 {
						q := new(big.Rat).Quo(new(big.Rat).Sub(n, lo), step)
						if !q.IsInt() {
							return invalidValue("value does not match the specification step")
						}
					}
				}
			}
		}
	}
	if p.ValueList != nil {
		n, ok := numeric(v)
		if !ok {
			return invalidValue("value must match the specification enum")
		}
		for _, choice := range *p.ValueList {
			if n.Cmp(new(big.Rat).SetInt64(int64(choice.Value))) == 0 {
				return nil
			}
		}
		return invalidValue("value must match the specification enum")
	}
	return nil
}
