package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/pdaether/ploi-tui/internal/config"
)

type fakeKeyring struct {
	items map[string]string
	fail  error
}

func (f *fakeKeyring) Get(service, user string) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	v, ok := f.items[service+"\x00"+user]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}

func (f *fakeKeyring) Set(service, user, password string) error {
	if f.fail != nil {
		return f.fail
	}
	f.items[service+"\x00"+user] = password
	return nil
}

func (f *fakeKeyring) Delete(service, user string) error {
	if f.fail != nil {
		return f.fail
	}
	delete(f.items, service+"\x00"+user)
	return nil
}

func testEnv(t *testing.T) *config.Store {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "cfg"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	fk := &fakeKeyring{items: map[string]string{}}
	fk.fail = errors.New("Cannot autolaunch D-Bus without X11 $DISPLAY")
	store := config.NewStore(cfg).WithKeyring(fk)
	orig := newStore
	newStore = func() (*config.Store, error) { return store, nil }
	t.Cleanup(func() { newStore = orig })
	return store
}

func TestConnectFlowStoresTokenAndShowsAccount(t *testing.T) {
	store := testEnv(t)

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if _, err := fmt.Fprint(w, `{"data":{"name":"Dennis","email":"dennis@example.com","plan":"Pro"}}`); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLOI_TUI_API_URL", srv.URL)

	var in bytes.Buffer
	in.WriteString("  secret-token\n")
	var out bytes.Buffer

	err := runConnectFlow(t.Context(), &in, &out, store)
	if err != nil {
		t.Fatalf("runConnectFlow: %v", err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("auth header = %q", gotAuth)
	}

	text := out.String()
	for _, want := range []string{
		"https://ploi.io/profile/api-keys",
		"✓ Token valid — dennis@example.com",
		"Dennis",
		"Pro",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "secret-token") {
		t.Errorf("token leaked into output:\n%s", text)
	}

	data, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `api_token = "secret-token"`) {
		t.Errorf("token not persisted to file:\n%s", data)
	}
	info, _ := os.Stat(store.Path())
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config perms = %o", perm)
	}
}

func TestStripBracketedPaste(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain token", "secret-token", "secret-token"},
		{"full markers", "\x1b[200~secret-token\x1b[201~", "secret-token"},
		{"start marker only", "\x1b[200~secret-token", "secret-token"},
		{"end marker only", "secret-token\x1b[201~", "secret-token"},
		{"repeated markers", "\x1b[200~\x1b[200~secret\x1b[201~\x1b[201~", "secret"},
		{"empty", "", ""},
		{"markers only", "\x1b[200~\x1b[201~", ""},
	}
	for _, tt := range tests {
		if got := stripBracketedPaste(tt.in); got != tt.want {
			t.Errorf("%s: stripBracketedPaste(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}
}

func TestTerminalPasswordReaderAcceptsLongPaste(t *testing.T) {
	token := strings.Repeat("long-token-", 200)
	in := strings.NewReader("\x1b[200~" + token + "\x1b[201~\r")
	var out bytes.Buffer
	ttyIO := struct {
		io.Reader
		io.Writer
	}{in, &out}

	got, err := readHiddenToken(ttyIO)
	if err != nil {
		t.Fatalf("ReadPassword: %v", err)
	}
	if got != token {
		t.Fatalf("token length = %d, want %d", len(got), len(token))
	}
	if strings.Contains(out.String(), token) {
		t.Fatal("token was echoed to the terminal")
	}
}

func TestConnectFlowStripsBracketedPaste(t *testing.T) {
	store := testEnv(t)

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if _, err := fmt.Fprint(w, `{"data":{"email":"paste@example.com","plan":"Pro"}}`); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLOI_TUI_API_URL", srv.URL)

	var out bytes.Buffer
	err := runConnectFlow(t.Context(), strings.NewReader("\x1b[200~ pasted-token \x1b[201~\n"), &out, store)
	if err != nil {
		t.Fatalf("runConnectFlow: %v", err)
	}
	if gotAuth != "Bearer pasted-token" {
		t.Errorf("auth header = %q, want %q", gotAuth, "Bearer pasted-token")
	}
}

func TestConnectFlowInvalidToken(t *testing.T) {
	store := testEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLOI_TUI_API_URL", srv.URL)

	var out bytes.Buffer
	err := runConnectFlow(t.Context(), strings.NewReader("bad-token\n"), &out, store)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !strings.Contains(err.Error(), "rejected") || !strings.Contains(err.Error(), apiKeyURL) {
		t.Errorf("error = %v", err)
	}
}

func TestConnectFlowEmptyToken(t *testing.T) {
	store := testEnv(t)
	var out bytes.Buffer
	err := runConnectFlow(t.Context(), strings.NewReader("\n"), &out, store)
	if err == nil || !strings.Contains(err.Error(), "no token entered") {
		t.Fatalf("err = %v", err)
	}
}

func TestConnectFlowNetworkError(t *testing.T) {
	store := testEnv(t)
	t.Setenv("PLOI_TUI_API_URL", "http://127.0.0.1:0")

	var out bytes.Buffer
	err := runConnectFlow(t.Context(), strings.NewReader("tok\n"), &out, store)
	if err == nil {
		t.Fatal("expected network error")
	}
	if !strings.Contains(err.Error(), "validate token") {
		t.Errorf("error = %v", err)
	}
}

func TestLogoutFlowRemovesCredentials(t *testing.T) {
	store := testEnv(t)

	if _, err := store.SaveToken("tok"); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(store.Path())
	if !strings.Contains(string(data), "tok") {
		t.Fatalf("setup failed, file:\n%s", data)
	}

	var out bytes.Buffer
	if err := runLogout(&out, store); err != nil {
		t.Fatalf("runLogout: %v", err)
	}
	if !strings.Contains(out.String(), "✓ Credentials removed.") {
		t.Errorf("output = %q", out.String())
	}
	if _, err := store.LoadToken(); !errors.Is(err, config.ErrTokenNotFound) {
		t.Fatalf("want ErrTokenNotFound, got %v", err)
	}
}

func TestLogoutFlowWithoutCredentials(t *testing.T) {
	store := testEnv(t)
	var out bytes.Buffer
	if err := runLogout(&out, store); err != nil {
		t.Fatalf("runLogout: %v", err)
	}
	if !strings.Contains(out.String(), "No stored credentials found.") {
		t.Errorf("output = %q", out.String())
	}
}

func TestConnectCommandWiring(t *testing.T) {
	testEnv(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := fmt.Fprint(w, `{"data":{"email":"wired@example.com","plan":"Basic"}}`); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PLOI_TUI_API_URL", srv.URL)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString("pipe-token\n"); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	oldStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	out := runCommand(t, "connect")
	if !strings.Contains(out, "wired@example.com") {
		t.Errorf("connect via cobra output = %q", out)
	}
}
