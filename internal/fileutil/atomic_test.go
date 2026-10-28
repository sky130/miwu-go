package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "file")
	if err := WriteAtomic(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "new" {
		t.Fatal("replacement failed")
	}
}
