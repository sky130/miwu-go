package app

import (
	"context"
	"fmt"
	"strconv"

	"miwu-go/internal/miot"
	"miwu-go/internal/miot/client/home"
	miotmodel "miwu-go/internal/miot/model/miot"
)

const DefaultHomeLimit = 300

// HomeRef carries the actual owner, including for homes shared with the account.
// String IDs also survive a future JavaScript bridge without loss of precision.
type HomeRef struct {
	ID       string `json:"id"`
	OwnerUID string `json:"owner_uid"`
}

type Home struct {
	ID       string `json:"id"`
	OwnerUID string `json:"owner_uid"`
	Name     string `json:"name"`
	Shared   bool   `json:"shared"`
}

func (h Home) Ref() HomeRef {
	return HomeRef{ID: h.ID, OwnerUID: h.OwnerUID}
}

func (h HomeRef) miotRef() (miotmodel.HomeRef, error) {
	id, idErr := strconv.ParseInt(h.ID, 10, 64)
	owner, ownerErr := strconv.ParseInt(h.OwnerUID, 10, 64)
	if idErr != nil || ownerErr != nil || id <= 0 || owner <= 0 {
		return miotmodel.HomeRef{}, fmt.Errorf("%w: home ID and owner UID must be positive integers", miot.ErrInvalidArgument)
	}
	return miotmodel.HomeRef{ID: id, OwnerUID: owner}, nil
}

type HomeList struct {
	Homes   []Home `json:"homes"`
	HasMore bool   `json:"has_more"`
	Cursor  string `json:"cursor"`
}

func (list HomeList) Find(id string) (HomeRef, error) {
	for _, item := range list.Homes {
		if item.ID == id {
			return item.Ref(), nil
		}
	}
	return HomeRef{}, fmt.Errorf("home %q is not present in the returned homes", id)
}

func (a *App) ListHomes(ctx context.Context, accountID string, limit int) (HomeList, error) {
	if limit <= 0 {
		return HomeList{}, fmt.Errorf("%w: home limit must be positive", miot.ErrInvalidArgument)
	}
	clients, err := a.services(ctx, accountID)
	if err != nil {
		return HomeList{}, err
	}
	response, err := clients.home.ListHomes(ctx, home.WithHomeLimit(limit))
	if err != nil {
		return HomeList{}, err
	}

	result := HomeList{
		Homes:   make([]Home, 0, len(response.Homes)),
		HasMore: response.HasMore,
		Cursor:  response.MaxID,
	}
	seen := make(map[int64]bool)
	appendHome := func(item miotmodel.Home) {
		result.Homes = append(result.Homes, Home{
			ID:       strconv.FormatInt(item.ID, 10),
			OwnerUID: strconv.FormatInt(item.OwnerUID, 10),
			Name:     item.Name,
			Shared:   item.IsShareHome(),
		})
		seen[item.ID] = true
	}
	for _, item := range response.Homes {
		appendHome(item)
	}
	if response.ShareHomes != nil {
		for _, item := range *response.ShareHomes {
			if !seen[item.ID] {
				appendHome(item)
			}
		}
	}
	return result, nil
}
