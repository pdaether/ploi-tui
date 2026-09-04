package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/BurntSushi/toml"
)

const AppDir = "ploi-tui"

type AuthConfig struct {
	APIToken string `toml:"api_token,omitempty"`
}

type Config struct {
	Auth AuthConfig `toml:"auth,omitempty"`

	filePath string
}

func DefaultConfig() *Config {
	return &Config{}
}

func UserConfigDir() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, AppDir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Roaming", AppDir), nil
	}
	return filepath.Join(home, ".config", AppDir), nil
}

func UserCacheDir() (string, error) {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, AppDir), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Local", "cache", AppDir), nil
	}
	return filepath.Join(home, ".cache", AppDir), nil
}

func FilePath() (string, error) {
	dir, err := UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

func Load() (*Config, error) {
	path, err := FilePath()
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()
	cfg.filePath = path

	data, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		return cfg, nil
	case err != nil:
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := ensureConfigDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if err := fixPermissions(path); err != nil {
		return nil, fmt.Errorf("secure config %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) Save() error {
	dir, err := UserConfigDir()
	if err != nil {
		return err
	}
	if err := ensureConfigDir(dir); err != nil {
		return err
	}
	path := c.filePath
	if path == "" {
		path = filepath.Join(dir, "config.toml")
		c.filePath = path
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(c); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return fixPermissions(path)
}

func (c *Config) Path() string {
	if c.filePath == "" {
		p, _ := FilePath()
		return p
	}
	return c.filePath
}

func fixPermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return os.Chmod(path, 0o600)
	}
	return nil
}

func ensureConfigDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat config dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("config path %s is not a directory", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("secure config dir %s: %w", dir, err)
		}
	}
	return nil
}
