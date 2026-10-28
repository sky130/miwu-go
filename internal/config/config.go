package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"miwu-go/internal/fileutil"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DefaultAccount string `yaml:"default,omitempty"`
}

type File struct {
	path string
}

func NewFile(dir string) *File {
	return &File{path: filepath.Join(dir, "config.yaml")}
}

func DefaultDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "miwu"), nil
}

func (f *File) Load() (Config, error) {
	b, err := os.ReadFile(f.path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var c Config
	if err = yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	return c, nil
}

func (f *File) Save(c Config) error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err = fileutil.WriteAtomic(f.path, b); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}
