package ui

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pdaether/ploi-tui/internal/api"
)

type ipCopiedMsg struct{ err error }

type browserOpenedMsg struct {
	err    error
	target string
}

type restartFinishedMsg struct{ err error }

type sshFinishedMsg struct {
	err    error
	output string
}

type restartPollMsg struct{ serverID int64 }

func (a App) restartServer(id int64) tea.Cmd {
	return func() tea.Msg { return restartFinishedMsg{err: a.client.RestartServer(a.context(), id)} }
}

func (a App) restartPoll(serverID int64) tea.Cmd {
	return tea.Tick(10*time.Second, func(time.Time) tea.Msg { return restartPollMsg{serverID: serverID} })
}

func (a *App) ssh(server *api.Server, user string) tea.Cmd {
	if server.IPAddress == "" {
		a.toast = a.newToast("Server has no IP address")
		return clearToast(a.toast.id)
	}
	if user == "" {
		user = "ploi"
	}
	if !validSSHUsername(user) {
		a.toast = a.newToast("Invalid SSH username received from Ploi")
		return clearToast(a.toast.id)
	}
	a.sshError = ""
	var stderr bytes.Buffer
	cmd := sshCommand(user, server.IPAddress)
	// Keep SSH diagnostics visible while connected and retain them when the TUI resumes.
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return sshFinishedMsg{err: err, output: stderr.String()}
	})
}

func validSSHUsername(user string) bool {
	for i := range len(user) {
		char := user[i]
		letter := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
		if i == 0 {
			if !letter && char != '_' {
				return false
			}
			continue
		}
		digit := char >= '0' && char <= '9'
		if !letter && !digit && char != '_' && char != '-' {
			return false
		}
	}
	return user != ""
}

func sshCommand(user, host string) *exec.Cmd {
	return exec.Command("ssh", "--", user+"@"+host)
}

func sshUsers(users []api.SystemUser) []api.SystemUser {
	unique := make(map[string]api.SystemUser, len(users)+1)
	unique["ploi"] = api.SystemUser{Name: "ploi", Root: "/home/ploi"}
	for _, user := range users {
		if validSSHUsername(user.Name) {
			unique[user.Name] = user
		}
	}
	result := make([]api.SystemUser, 0, len(unique))
	for _, user := range unique {
		result = append(result, user)
	}
	return result
}

func systemUsersUnavailable(err error) bool {
	var apiErr *api.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func (a *App) copyIP(server api.Server) tea.Cmd {
	if server.IPAddress == "" {
		a.toast = a.newToast("Server has no IP address")
		return clearToast(a.toast.id)
	}
	ip := server.IPAddress
	return func() tea.Msg { return ipCopiedMsg{err: copyToClipboard(ip)} }
}

func (a *App) openPloiServer(server api.Server) tea.Cmd {
	url := fmt.Sprintf("https://ploi.io/servers/%d", server.ID)
	return openBrowser(url, "server in Ploi")
}

func (a *App) openPloiSite(server api.Server, site api.Site) tea.Cmd {
	url := fmt.Sprintf("https://ploi.io/servers/%d/sites/%d", server.ID, site.ID)
	return openBrowser(url, "site in Ploi")
}

func (a *App) openSiteDomain(site api.Site) tea.Cmd {
	domain := strings.TrimSpace(site.Domain)
	if domain == "" {
		a.toast = a.newToast("Site has no domain")
		return clearToast(a.toast.id)
	}
	return openBrowser("https://"+domain, "site domain")
}

func openBrowser(url, target string) tea.Cmd {
	command, args := browserCommand(url)
	return func() tea.Msg {
		return browserOpenedMsg{err: exec.Command(command, args...).Run(), target: target}
	}
}

func browserCommand(url string) (string, []string) {
	if runtime.GOOS == "darwin" {
		return "open", []string{url}
	}
	return "xdg-open", []string{url}
}

func copyToClipboard(value string) error {
	command, args := clipboardCommand()
	if command == "" {
		return fmt.Errorf("install wl-clipboard, xclip, or xsel")
	}
	cmd := exec.Command(command, args...)
	cmd.Stdin = strings.NewReader(value)
	if output, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return fmt.Errorf("%s: %s", command, message)
		}
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

var lookPath = exec.LookPath

func clipboardCommand() (string, []string) {
	if runtime.GOOS == "darwin" {
		return "pbcopy", nil
	}
	for _, candidate := range []struct {
		name string
		args []string
	}{
		{name: "wl-copy"},
		{name: "xclip", args: []string{"-selection", "clipboard"}},
		{name: "xsel", args: []string{"--clipboard", "--input"}},
	} {
		if _, err := lookPath(candidate.name); err == nil {
			return candidate.name, candidate.args
		}
	}
	return "", nil
}
