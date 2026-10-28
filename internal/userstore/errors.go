package userstore

import (
	"errors"
)

var (
	ErrNotFound           = errors.New("account not found")
	ErrCorrupt            = errors.New("corrupt account file")
	ErrUnsupportedVersion = errors.New("unsupported account file version")
)
