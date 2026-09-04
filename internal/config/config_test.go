package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testEnv(t *testing.T) {
	t.Helper()
	base := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "config-home"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "cache"))
}

func TestUserDirs(t *testing.T) {
	testEnv(t)

	cfgDir, err := UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir: %v", err)
	}
	if !filepath.IsAbs(cfgDir) || filepath.Base(cfgDir) != AppDir {
		t.Errorf("cfgDir = %q", cfgDir)
	}
	cacheDir, err := UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir: %v", err)
	}
	if !filepath.IsAbs(cacheDir) || filepath.Base(cacheDir) != AppDir {
		t.Errorf("cacheDir = %q", cacheDir)
	}

	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	cfgDir2, err := UserConfigDir()
	if err != nil {
		t.Fatalf("UserConfigDir without XDG: %v", err)
	}
	if want := filepath.Join(".config", AppDir); filepath.Base(filepath.Dir(cfgDir2)) != filepath.Dir(want) || filepath.Base(cfgDir2) != AppDir {
		t.Errorf("cfgDir without XDG = %q", cfgDir2)
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	testEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load fresh: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(cfg.Path())
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(filepath.Dir(cfg.Path()))
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("config dir perms = %o, want 700", perm)
	}

	_, err = Load()
	if err != nil {
		t.Fatalf("Reload: %v", err)
	}
}

func TestLoadMissingFileUsesDefaults(t *testing.T) {
	testEnv(t)
	_, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func TestFixPermissionsTightensLooseFile(t *testing.T) {
	testEnv(t)
	path := filepath.Join(t.TempDir(), "loose.toml")
	if err := os.WriteFile(path, []byte("[ssh]\ncommand = 'x'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := fixPermissions(path); err != nil {
		t.Fatalf("fixPermissions: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perms = %o, want 600", info.Mode().Perm())
	}

	strict := filepath.Join(t.TempDir(), "strict.toml")
	if err := os.WriteFile(strict, []byte("x = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := func() os.FileMode {
		i, _ := os.Stat(strict)
		return i.Mode().Perm()
	}()
	_ = fixPermissions(strict)
	if after := func() os.FileMode {
		i, _ := os.Stat(strict)
		return i.Mode().Perm()
	}(); after != before {
		t.Errorf("strict file perms changed: %o -> %o", before, after)
	}
}

func TestEnsureConfigDirTightensLooseDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ensureConfigDir(dir); err != nil {
		t.Fatalf("ensureConfigDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("directory perms = %o, want 700", info.Mode().Perm())
	}
}
