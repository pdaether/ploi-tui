package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"
)

const (
	KeyringService = "ploi-tui"
	KeyringUser    = "api-token"
)

var ErrTokenNotFound = errors.New("no ploi API token stored")

type TokenLocation int

const (
	LocationNone TokenLocation = iota
	LocationKeyring
	LocationFile
)

func (l TokenLocation) String() string {
	switch l {
	case LocationKeyring:
		return "OS keyring"
	case LocationFile:
		return "config file"
	default:
		return "nowhere"
	}
}

type Keyring interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type systemKeyring struct{}

func (systemKeyring) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}

func (systemKeyring) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}

func (systemKeyring) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

func isKeyringMissing(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, keyring.ErrNotFound) || errors.Is(err, keyring.ErrUnsupportedPlatform) {
		return true
	}
	msg := strings.ToLower(strings.ReplaceAll(err.Error(), "-", ""))
	for _, marker := range []string{"not found", "unsupported", "secretservice", "secret service", "dbus", "no such file or directory", "connection refused", "no such interface"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

type Store struct {
	cfg       *Config
	keyringer Keyring
}

func NewStore(cfg *Config) *Store {
	return &Store{cfg: cfg, keyringer: systemKeyring{}}
}

func (s *Store) Path() string { return s.cfg.Path() }

func (s *Store) Config() *Config { return s.cfg }

func (s *Store) WithKeyring(k Keyring) *Store {
	s.keyringer = k
	return s
}

type StoredToken struct {
	Token    string
	Location TokenLocation
}

func (s *Store) LoadToken() (StoredToken, error) {
	tok, err := s.keyringer.Get(KeyringService, KeyringUser)
	switch {
	case err == nil && tok != "":
		return StoredToken{Token: tok, Location: LocationKeyring}, nil
	case isKeyringMissing(err):
	default:
		return StoredToken{}, fmt.Errorf("read token from keyring: %w", err)
	}

	if tok := strings.TrimSpace(s.cfg.Auth.APIToken); tok != "" {
		return StoredToken{Token: tok, Location: LocationFile}, nil
	}
	return StoredToken{}, ErrTokenNotFound
}

func (s *Store) SaveToken(token string) (TokenLocation, error) {
	err := s.keyringer.Set(KeyringService, KeyringUser, token)
	if err == nil {
		if err := s.clearFileToken(); err != nil {
			return LocationKeyring, err
		}
		return LocationKeyring, nil
	}
	if !isKeyringMissing(err) {
		return LocationNone, fmt.Errorf("store token in keyring: %w", err)
	}
	s.cfg.Auth.APIToken = token
	if err := s.cfg.Save(); err != nil {
		return LocationNone, err
	}
	return LocationFile, nil
}

func (s *Store) DeleteToken() error {
	var errs []error
	if err := s.keyringer.Delete(KeyringService, KeyringUser); err != nil && !isKeyringMissing(err) {
		errs = append(errs, fmt.Errorf("delete token from keyring: %w", err))
	}
	if s.cfg.Auth.APIToken != "" {
		s.cfg.Auth.APIToken = ""
		if err := s.cfg.Save(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *Store) clearFileToken() error {
	if s.cfg.Auth.APIToken == "" {
		return nil
	}
	s.cfg.Auth.APIToken = ""
	return s.cfg.Save()
}
