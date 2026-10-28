package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigRoundTripAndFailedReplacement(t *testing.T) {
	dir := t.TempDir()
	store := NewFile(dir)
	initial, err := store.Load()
	if err != nil || initial.DefaultAccount != "" {
		t.Fatal(err)
	}
	want := Config{DefaultAccount: "9007199254740993"}
	if err = store.Save(want); err != nil {
		t.Fatal(err)
	}
	got, err := NewFile(dir).Load()
	if err != nil || got != want {
		t.Fatalf("config did not survive restart: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "config.yaml"))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("wrong config permissions")
	}
	// A destination directory forces replacement failure even when tests run as root.
	blocked := NewFile(filepath.Join(dir, "blocked"))
	if err = os.MkdirAll(blocked.path, 0700); err != nil {
		t.Fatal(err)
	}
	if err = blocked.Save(want); err == nil {
		t.Fatal("write failure reported as success")
	}
	entries, err := os.ReadDir(filepath.Dir(blocked.path))
	if err != nil || len(entries) != 1 {
		t.Fatal("temporary files leaked")
	}
}
