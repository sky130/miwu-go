package app

import (
	"context"
	"fmt"

	"miwu-go/internal/miot"
	miotmodel "miwu-go/internal/miot/model/miot"
)

type Scene struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func (a *App) ListScenes(ctx context.Context, accountID string, ref HomeRef) ([]Scene, error) {
	homeRef, err := ref.miotRef()
	if err != nil {
		return nil, err
	}
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return nil, err
	}
	response, err := clients.home.ListScenes(ctx, homeRef)
	if err != nil {
		return nil, err
	}
	result := []Scene{}
	if response.Scenes != nil {
		for _, scene := range *response.Scenes {
			result = append(result, Scene{ID: scene.SceneID, Name: scene.Name, Enabled: scene.Enable})
		}
	}
	return result, nil
}

// RunScene reports request acceptance; it does not wait for devices to complete.
func (a *App) RunScene(ctx context.Context, accountID string, ref HomeRef, sceneID string) error {
	if sceneID == "" {
		return fmt.Errorf("%w: scene ID is required", miot.ErrInvalidArgument)
	}
	homeRef, err := ref.miotRef()
	if err != nil {
		return err
	}
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return err
	}
	return writeFailure(clients.home.RunScene(ctx, homeRef, miotmodel.Scene{SceneID: sceneID}))
}
