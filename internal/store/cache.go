package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/config"
)

const serverCacheTTL = 2 * time.Minute

type Cache struct {
	dir string
	now func() time.Time
}

type serverCacheFile struct {
	SavedAt time.Time    `json:"saved_at"`
	Servers []api.Server `json:"servers"`
}

func New() (*Cache, error) {
	dir, err := config.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return &Cache{dir: dir, now: time.Now}, nil
}

func (c *Cache) LoadServers() ([]api.Server, bool, error) {
	data, err := os.ReadFile(c.serverPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read server cache: %w", err)
	}
	var cached serverCacheFile
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, false, fmt.Errorf("decode server cache: %w", err)
	}
	if cached.SavedAt.IsZero() || c.now().Sub(cached.SavedAt) > serverCacheTTL {
		return nil, false, nil
	}
	return cached.Servers, true, nil
}

func (c *Cache) SaveServers(servers []api.Server) error {
	if err := os.MkdirAll(c.dir, 0o700); err != nil {
		return fmt.Errorf("create cache directory: %w", err)
	}
	if err := os.Chmod(c.dir, 0o700); err != nil {
		return fmt.Errorf("secure cache directory: %w", err)
	}
	data, err := json.Marshal(serverCacheFile{SavedAt: c.now().UTC(), Servers: servers})
	if err != nil {
		return fmt.Errorf("encode server cache: %w", err)
	}
	temp, err := os.CreateTemp(c.dir, ".servers-*.tmp")
	if err != nil {
		return fmt.Errorf("create server cache: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(0o600); err != nil {
		_ = temp.Close()
		return fmt.Errorf("secure server cache: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return fmt.Errorf("write server cache: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close server cache: %w", err)
	}
	if err := os.Rename(tempPath, c.serverPath()); err != nil {
		return fmt.Errorf("replace server cache: %w", err)
	}
	return nil
}

func (c *Cache) serverPath() string { return filepath.Join(c.dir, "servers.json") }
