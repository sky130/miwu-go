package cli

import (
	"context"

	"miwu-go/internal/app"

	"github.com/spf13/cobra"
)

func (s *state) required(ctx context.Context, value, prompt string) (string, error) {
	if value != "" {
		return value, nil
	}
	return s.line(ctx, prompt)
}

func (s *state) addSpecs(root *cobra.Command) {
	parent := &cobra.Command{Use: "spec", Short: "Inspect device specifications", Args: cobra.NoArgs}
	root.AddCommand(parent)
	for _, name := range []string{"show", "languages", "parse-urn"} {
		var raw, language string
		cmd := s.add(parent, name, "Inspect "+name, func(ctx context.Context) (result, error) {
			value, err := s.required(ctx, raw, "URN")
			if err != nil {
				return result{}, err
			}
			parsed, err := app.ParseURN(value)
			if err != nil {
				return result{}, err
			}
			if name == "parse-urn" {
				return result{Data: parsed}, nil
			}
			application, err := s.application()
			if err != nil {
				return result{}, err
			}
			if name == "languages" {
				body, err := application.SpecificationLanguages(ctx, value)
				if err != nil {
					return result{}, err
				}
				return result{Data: body}, nil
			}
			specification, err := application.GetSpecification(ctx, value, language)
			return result{Data: specification}, err
		})
		cmd.Flags().StringVar(&raw, "urn", "", "MIoT URN")
		if name == "show" {
			cmd.Flags().StringVar(&language, "language", "zh_cn", "Translation language, empty for original descriptions")
		}
	}
	var model string
	cmd := s.add(parent, "icon", "Get the device icon URL", func(ctx context.Context) (result, error) {
		name, err := s.required(ctx, model, "Device model")
		if err != nil {
			return result{}, err
		}
		application, err := s.application()
		if err != nil {
			return result{}, err
		}
		icon, err := application.IconURL(ctx, name)
		if err != nil {
			return result{}, err
		}
		return result{Data: map[string]string{"url": icon}}, nil
	})
	cmd.Flags().StringVar(&model, "model", "", "Device model")
}
