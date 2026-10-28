package cli

import (
	"context"
	"fmt"
	"strconv"

	"miwu-go/internal/app"

	"github.com/spf13/cobra"
)

func truncated(more bool) string {
	if more {
		return "The server reports more items. This result is truncated; pagination is not yet verified."
	}
	return ""
}

func (s *state) addHomes(root *cobra.Command) {
	parent := &cobra.Command{Use: "home", Short: "Browse homes", Args: cobra.NoArgs}
	root.AddCommand(parent)
	var limit int
	cmd := s.add(parent, "list", "List owned and shared homes", func(ctx context.Context) (result, error) {
		if limit <= 0 {
			return result{}, usage("--limit must be positive")
		}
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		list, err := application.ListHomes(ctx, accountID, limit)
		if err != nil {
			return result{}, err
		}
		rows := make([][]string, 0, len(list.Homes))
		for _, item := range list.Homes {
			rows = append(rows, []string{item.ID, item.Name, item.OwnerUID, strconv.FormatBool(item.Shared)})
		}
		return result{
			Data:    list,
			Columns: []string{"ID", "NAME", "OWNER", "SHARED"},
			Rows:    rows,
			Note:    truncated(list.HasMore),
		}, nil
	})
	cmd.Flags().IntVar(&limit, "limit", app.DefaultHomeLimit, "Maximum returned homes")
}

func (s *state) home(ctx context.Context, application *app.App, accountID string) (app.HomeRef, error) {
	if s.settings.Home == "" && !s.interactive() {
		return app.HomeRef{}, usage("--home is required")
	}
	list, err := application.ListHomes(ctx, accountID, app.DefaultHomeLimit)
	if err != nil {
		return app.HomeRef{}, err
	}
	if note := truncated(list.HasMore); note != "" {
		fmt.Fprintln(s.options.Err, note)
	}
	if s.settings.Home != "" {
		return list.Find(s.settings.Home)
	}
	labels := make([]string, len(list.Homes))
	for index, item := range list.Homes {
		labels[index] = fmt.Sprintf("%s (id=%s, owner=%s)", item.Name, item.ID, item.OwnerUID)
	}
	index, err := s.choose(ctx, "Select home", labels)
	if err != nil {
		return app.HomeRef{}, err
	}
	return list.Homes[index].Ref(), nil
}

func (s *state) target(ctx context.Context, application *app.App, accountID, deviceID string, metadata bool) (app.Device, error) {
	if deviceID == "" && !s.interactive() {
		return app.Device{}, usage("--did is required")
	}
	if deviceID != "" && !metadata {
		return app.Device{ID: deviceID}, nil
	}
	ref, err := s.home(ctx, application, accountID)
	if err != nil {
		return app.Device{}, err
	}
	list, err := application.ListDevices(ctx, accountID, ref, app.DefaultDeviceLimit)
	if err != nil {
		return app.Device{}, err
	}
	if note := truncated(list.HasMore); note != "" {
		fmt.Fprintln(s.options.Err, note)
	}
	if deviceID != "" {
		return list.Find(deviceID)
	}
	labels := make([]string, len(list.Devices))
	for index, item := range list.Devices {
		labels[index] = fmt.Sprintf("%s (%s, online=%t)", item.Name, item.ID, item.Online)
	}
	index, err := s.choose(ctx, "Select device", labels)
	if err != nil {
		return app.Device{}, err
	}
	return list.Devices[index], nil
}
