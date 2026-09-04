package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pdaether/ploi-tui/internal/api"
)

func TestServerCacheRoundTripAndTTL(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	c := &Cache{dir: t.TempDir(), now: func() time.Time { return now }}
	servers := []api.Server{{ID: 7, Name: "web-1", IPAddress: "139.59.201.10"}}
	if err := c.SaveServers(servers); err != nil {
		t.Fatalf("SaveServers: %v", err)
	}
	info, err := os.Stat(filepath.Join(c.dir, "servers.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("cache permissions = %o, want 600", info.Mode().Perm())
	}
	got, ok, err := c.LoadServers()
	if err != nil || !ok || len(got) != 1 || got[0].Name != "web-1" {
		t.Errorf("LoadServers() = %+v, %v, %v", got, ok, err)
	}
	c.now = func() time.Time { return now.Add(serverCacheTTL + time.Second) }
	if _, ok, err := c.LoadServers(); err != nil || ok {
		t.Errorf("expired LoadServers() ok=%v err=%v", ok, err)
	}
}
