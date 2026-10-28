package cli

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
)

func (s *state) addScenes(root *cobra.Command) {
	parent := &cobra.Command{Use: "scene", Short: "Browse and run scenes", Args: cobra.NoArgs}
	root.AddCommand(parent)

	s.add(parent, "list", "List manual scenes", func(ctx context.Context) (result, error) {
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		ref, err := s.home(ctx, application, accountID)
		if err != nil {
			return result{}, err
		}
		scenes, err := application.ListScenes(ctx, accountID, ref)
		if err != nil {
			return result{}, err
		}
		rows := make([][]string, 0, len(scenes))
		for _, scene := range scenes {
			rows = append(rows, []string{scene.ID, scene.Name, strconv.FormatBool(scene.Enabled)})
		}
		return result{Data: scenes, Columns: []string{"ID", "NAME", "ENABLED"}, Rows: rows}, nil
	})

	var sceneID string
	cmd := s.add(parent, "run", "Submit a scene for execution", func(ctx context.Context) (result, error) {
		if sceneID == "" && !s.interactive() {
			return result{}, usage("--scene is required")
		}
		application, accountID, err := s.applicationForAccount(ctx)
		if err != nil {
			return result{}, err
		}
		ref, err := s.home(ctx, application, accountID)
		if err != nil {
			return result{}, err
		}
		id := sceneID
		if id == "" {
			scenes, err := application.ListScenes(ctx, accountID, ref)
			if err != nil {
				return result{}, err
			}
			labels := make([]string, len(scenes))
			for index, scene := range scenes {
				labels[index] = scene.Name + " (" + scene.ID + ")"
			}
			index, err := s.choose(ctx, "Select scene", labels)
			if err != nil {
				return result{}, err
			}
			id = scenes[index].ID
		}
		if err := application.RunScene(ctx, accountID, ref, id); err != nil {
			return result{}, err
		}
		return message("Scene request accepted: " + id), nil
	})
	cmd.Flags().StringVar(&sceneID, "scene", "", "Scene ID")
}
