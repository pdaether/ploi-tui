package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

type fakeKeyring struct {
	items map[string]string
	fail  error
}

func newFakeKeyring() *fakeKeyring { return &fakeKeyring{items: map[string]string{}} }

func key(service, user string) string { return service + "\x00" + user }

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	if v, ok := f.items[key(service, user)]; ok {
		return v, nil
	}
	return "", keyring.ErrNotFound
}

func (f *fakeKeyring) Set(service, user, password string) error {
	if f.fail != nil {
		return f.fail
	}
	f.items[key(service, user)] = password
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if f.fail != nil {
		return f.fail
	}
	delete(f.items, key(service, user))
	return nil
}

func testStore(t *testing.T) (*Store, *fakeKeyring) {
	t.Helper()
	testEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fk := newFakeKeyring()
	return NewStore(cfg).WithKeyring(fk), fk
}

func TestSaveTokenToKeyring(t *testing.T) {
	store, fk := testStore(t)

	loc, err := store.SaveToken("tok-123")
	if err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if loc != LocationKeyring {
		t.Errorf("location = %v, want keyring", loc)
	}
	if fk.items[key(KeyringService, KeyringUser)] != "tok-123" {
		t.Errorf("keyring item = %q", fk.items[key(KeyringService, KeyringUser)])
	}

	got, err := store.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if got.Token != "tok-123" || got.Location != LocationKeyring {
		t.Errorf("loaded = %+v", got)
	}
}

func TestSaveTokenFallsBackToFile(t *testing.T) {
	store, fk := testStore(t)
	fk.fail = errors.New("Cannot autolaunch D-Bus without X11 $DISPLAY")

	loc, err := store.SaveToken("tok-file")
	if err != nil {
		t.Fatalf("SaveToken: %v", err)
	}
	if loc != LocationFile {
		t.Errorf("location = %v, want file", loc)
	}
	if loc.String() != "config file" {
		t.Errorf("location string = %q", loc.String())
	}

	info, err := os.Stat(store.cfg.Path())
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 600", perm)
	}

	got, err := store.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if got.Token != "tok-file" || got.Location != LocationFile {
		t.Errorf("loaded = %+v", got)
	}

	data, err := os.ReadFile(store.cfg.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `api_token = "tok-file"`) {
		t.Errorf("config file missing token:\n%s", data)
	}
}

func TestKeyringPreferredOverFile(t *testing.T) {
	store, _ := testStore(t)

	store.cfg.Auth.APIToken = "file-token"
	if err := store.cfg.Save(); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	fk2 := newFakeKeyring()
	fk2.items[key(KeyringService, KeyringUser)] = "keyring-token"
	s3 := NewStore(reloaded).WithKeyring(fk2)

	got, err := s3.LoadToken()
	if err != nil {
		t.Fatalf("LoadToken: %v", err)
	}
	if got.Location != LocationKeyring || got.Token != "keyring-token" {
		t.Errorf("loaded = %+v", got)
	}
}

func TestDeleteTokenRemovesBothLocations(t *testing.T) {
	store, fk := testStore(t)

	store.cfg.Auth.APIToken = "legacy-file"
	if err := store.cfg.Save(); err != nil {
		t.Fatal(err)
	}
	fk.items[key(KeyringService, KeyringUser)] = "stale-keyring"

	if err := store.DeleteToken(); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, exists := fk.items[key(KeyringService, KeyringUser)]; exists {
		t.Errorf("keyring entry still present")
	}
	if store.cfg.Auth.APIToken != "" {
		t.Errorf("file token still set in config")
	}

	if _, err := store.LoadToken(); !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("want ErrTokenNotFound, got %v", err)
	}
}

func TestDeleteTokenIdempotent(t *testing.T) {
	store, _ := testStore(t)
	if err := store.DeleteToken(); err != nil {
		t.Fatalf("DeleteToken on empty store: %v", err)
	}
}

func TestLoadTokenNotFound(t *testing.T) {
	store, _ := testStore(t)
	_, err := store.LoadToken()
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("want ErrTokenNotFound, got %v", err)
	}
}

func TestConfigPreservedAroundTokenWrite(t *testing.T) {
	testEnv(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}

	fk := newFakeKeyring()
	fk.fail = errors.New("secret service not available")
	store := NewStore(cfg).WithKeyring(fk)
	if _, err := store.SaveToken("t"); err != nil {
		t.Fatal(err)
	}

	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reloaded.Auth.APIToken, "t") {
		t.Errorf("token not persisted")
	}
}

func TestFilePathMatchesConfigDir(t *testing.T) {
	testEnv(t)
	fp, err := FilePath()
	if err != nil {
		t.Fatal(err)
	}
	dir, _ := UserConfigDir()
	if fp != filepath.Join(dir, "config.toml") {
		t.Errorf("FilePath = %q, want under %q", fp, dir)
	}
}
