package userstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"miwu-go/internal/fileutil"
	"miwu-go/internal/miot/model/user"
)

const schemaVersion = 1

type storeRecord struct {
	Version  int           `json:"version"`
	Identity user.Identity `json:"identity"`
	Info     *user.Info    `json:"info"`
	Session  sealedBox     `json:"session"` // 加密后的 user.Session
}

type FileStore struct {
	dir    string
	cipher *Cipher
	mu     sync.RWMutex
}

func NewFileStore(dir string, cipher *Cipher) (*FileStore, error) {
	if cipher == nil {
		return nil, errors.New("account cipher is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir, cipher: cipher}, nil
}

func (s *FileStore) Save(ctx context.Context, c user.Credentials) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.UserID == "" {
		return errors.New("userstore: empty userID")
	}

	plain, err := json.Marshal(c.Session)
	if err != nil {
		return err
	}

	box, err := s.cipher.Seal(plain)
	if err != nil {
		return err
	}

	rec := storeRecord{Version: schemaVersion, Identity: c.Identity, Session: box}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	return fileutil.WriteAtomic(s.pathFor(c.UserID), data)
}

func (s *FileStore) Load(ctx context.Context, userID string) (user.Credentials, error) {
	if err := ctx.Err(); err != nil {
		return user.Credentials{}, err
	}
	s.mu.RLock()
	data, err := os.ReadFile(s.pathFor(userID))
	s.mu.RUnlock()
	if errors.Is(err, os.ErrNotExist) {
		return user.Credentials{}, ErrNotFound
	}
	if err != nil {
		return user.Credentials{}, fmt.Errorf("load account: %w", err)
	}

	var rec storeRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return user.Credentials{}, fmt.Errorf("%w: invalid JSON", ErrCorrupt)
	}

	if rec.Version != schemaVersion {
		return user.Credentials{}, fmt.Errorf("%w: %d", ErrUnsupportedVersion, rec.Version)
	}

	plain, err := s.cipher.Open(rec.Session)
	if err != nil {
		return user.Credentials{}, fmt.Errorf("%w: decrypt session: %w", ErrCorrupt, err)
	}
	if rec.Identity.UserID != userID {
		return user.Credentials{}, fmt.Errorf("%w: identity mismatch", ErrCorrupt)
	}

	var sess user.Session
	if err := json.Unmarshal(plain, &sess); err != nil {
		return user.Credentials{}, fmt.Errorf("%w: invalid decrypted session", ErrCorrupt)
	}

	return user.Credentials{Identity: rec.Identity, Session: sess}, nil
}

func (s *FileStore) Delete(ctx context.Context, userID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	err := os.Remove(s.pathFor(userID))
	if errors.Is(err, os.ErrNotExist) {
		return ErrNotFound
	}
	return err
}

func (s *FileStore) List(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read account record: %w", err)
		}
		var rec storeRecord
		if json.Unmarshal(data, &rec) != nil || rec.Identity.UserID == "" {
			return nil, fmt.Errorf("%w: invalid account record", ErrCorrupt)
		}
		if rec.Version != schemaVersion {
			return nil, fmt.Errorf("%w: %d", ErrUnsupportedVersion, rec.Version)
		}
		ids = append(ids, rec.Identity.UserID)
	}
	sort.Strings(ids)
	return ids, nil
}

func (s *FileStore) pathFor(userID string) string {
	sum := sha256.Sum256([]byte(userID))
	return filepath.Join(s.dir, hex.EncodeToString(sum[:16])+".json")
}
