package ui

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/ui/serverdetail"
	"github.com/pdaether/ploi-tui/internal/ui/sitedetail"
)

func TestAppLoadsUserAndShowsQuota(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Limit", "60")
		w.Header().Set("X-RateLimit-Remaining", "54")
		body := `{"data":{"name":"Dennis","email":"dennis@example.com"}}`
		if r.URL.Path == "/api/servers" {
			body = `{"data":[{"id":7,"name":"web-1","ip_address":"139.59.201.10","status":"active","sites_count":12,"php_version":8.3}],"links":{"next":null},"meta":{"current_page":1,"last_page":1}}`
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.New("token", api.WithBaseURL(srv.URL)))
	msg := app.Init()()
	model, cmd := app.Update(msg)
	model, _ = model.(App).Update(cmd())
	view := model.(App).View()
	for _, want := range []string{"dennis@example.com", "SERVERS (1)", "web-1", "quota 54/60"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}
}

func TestAppOpensServerAndLoadsMonitoring(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user":
			writeTestJSON(t, w, `{"data":{"email":"dennis@example.com"}}`)
		case "/api/servers":
			writeTestJSON(t, w, `{"data":[{"id":7,"name":"web-1","ip_address":"139.59.201.10","status":"active","monitoring":true}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
		case "/api/servers/7":
			writeTestJSON(t, w, `{"data":{"id":7,"name":"web-1","ip_address":"139.59.201.10","status":"active","monitoring":true,"php_version":8.3}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.New("token", api.WithBaseURL(srv.URL)))
	model, cmd := app.Update(app.Init()())
	model, cmd = model.(App).Update(cmd())
	model, cmd = model.(App).Update(tea.KeyMsg{Type: tea.KeyEnter})
	appModel := model.(App)
	if cmd == nil {
		t.Fatal("enter should load server details")
	}

	model, cmd = appModel.Update(cmd())
	appModel = model.(App)
	model, _ = appModel.Update(databasesLoadedMsg{requestID: appModel.databaseRequestID, serverID: 7, databases: []api.Database{{Type: "postgresql"}}})
	appModel = model.(App)
	model, _ = appModel.Update(monitoringLoadedMsg{
		requestID:  appModel.monitoringRequestID,
		generation: appModel.monitoringGeneration,
		serverID:   7,
		samples:    []api.MonitoringSample{{CPU: 5.2, RAM: 21.8, Disk: 40, LoadAverage: 0.19}},
	})
	appModel = model.(App)
	view := appModel.View()
	for _, want := range []string{"web-1", "139.59.201.10", "PostgreSQL (1)", "QUICK STATS", "CPU"} {
		if !strings.Contains(view, want) {
			t.Errorf("overview missing %q:\n%s", want, view)
		}
	}
	model, cmd = appModel.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if cmd == nil {
		t.Fatal("monitoring tab should schedule its refresh")
	}
	view = model.(App).View()
	for _, want := range []string{"MONITORING", "CPU %", "now 5.2%"} {
		if !strings.Contains(view, want) {
			t.Errorf("monitoring missing %q:\n%s", want, view)
		}
	}
}

func TestAppLoadsMonitoringAfterSelectingTabWhileDetailLoads(t *testing.T) {
	var detailRequested bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/servers/1":
			detailRequested = true
			writeTestJSON(t, w, `{"data":{"id":1,"name":"web-1","monitoring":true}}`)
		default:
			writeTestJSON(t, w, `{"data":[]}`)
		}
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.New("token", api.WithBaseURL(srv.URL)))
	app.user = &api.User{Email: "user@example.com"}
	app.serversLoaded = true
	app.serverList.SetServers([]api.Server{{ID: 1, Name: "web-1", Monitoring: false}})
	model, detailCmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model, monitorCmd := model.(App).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if monitorCmd != nil {
		t.Fatal("monitoring must wait for detail response")
	}
	model, monitorCmd = model.(App).Update(detailCmd())
	if !detailRequested || monitorCmd == nil {
		t.Fatalf("detailRequested=%v monitorCmd=%v", detailRequested, monitorCmd != nil)
	}
	model, _ = model.(App).Update(monitoringLoadedMsg{
		requestID:  model.(App).monitoringRequestID,
		generation: model.(App).monitoringGeneration,
		serverID:   1,
	})
	if !model.(App).detail.MonitoringLoaded {
		t.Fatal("monitoring response was not accepted after detail response")
	}
}

func TestAppIgnoresStaleResponses(t *testing.T) {
	app := NewApp(api.New("token"))
	app.serverRequestID = 2
	app.serversLoaded = false
	model, _ := app.Update(serversLoadedMsg{
		requestID: 1,
		servers:   []api.Server{{ID: 1, Name: "stale"}},
	})
	if got := model.(App).serverList.All(); len(got) != 0 {
		t.Fatalf("stale server response changed list: %+v", got)
	}

	app = NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1", Monitoring: true}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server, Tab: serverdetail.MonitoringTab, MonitoringLoading: true}
	app.monitoringRequestID = 2
	app.monitoringGeneration = 2
	model, _ = app.Update(monitoringLoadedMsg{
		requestID:  1,
		generation: 1,
		serverID:   7,
		samples:    []api.MonitoringSample{{CPU: 99}},
	})
	if got := model.(App).detail.Samples; len(got) != 0 {
		t.Fatalf("stale monitoring response changed samples: %+v", got)
	}
}

func TestAppMonitoringRefreshTick(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1", Monitoring: true}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server, Tab: serverdetail.MonitoringTab, MonitoringLoaded: true}
	app.monitoringGeneration = 4
	model, cmd := app.Update(monitoringTickMsg{serverID: 7, generation: 4})
	if cmd == nil {
		t.Fatal("monitoring tick should start a request")
	}
	if model.(App).detail.MonitoringLoading != true {
		t.Fatal("monitoring request should be marked loading")
	}
}

func TestAppShowsMonitoringUnavailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/servers/1/monitor" {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}
		writeTestJSON(t, w, `{"message":"You do not have monitoring installed on this server."}`)
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.New("token", api.WithBaseURL(srv.URL)))
	server := api.Server{ID: 1, Name: "web-1", Monitoring: true}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server, Tab: serverdetail.MonitoringTab}
	app.detail.MonitoringLoading = true
	app.monitoringRequestID = 1
	app.monitoringGeneration = 1
	model, _ := app.Update(monitoringLoadedMsg{
		requestID:  1,
		generation: 1,
		serverID:   1,
		err: &api.APIError{
			StatusCode: http.StatusUnprocessableEntity,
			Message:    "You do not have monitoring installed on this server.",
		},
	})
	view := model.(App).View()
	if !strings.Contains(view, "MONITORING NOT AVAILABLE") || !strings.Contains(view, "ploi.io/pricing") {
		t.Fatalf("unavailable view = %s", view)
	}
}

func TestAppLoadsSitesAndOpensSiteDetail(t *testing.T) {
	var sitesRequested, siteRequested, certsRequested bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/servers/7/sites":
			sitesRequested = true
			writeTestJSON(t, w, `{"data":[{"id":9,"server_id":7,"domain":"app.example.io","status":"active","project_type":"laravel"}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
		case "/api/servers/7/sites/9":
			siteRequested = true
			writeTestJSON(t, w, `{"data":{"id":9,"server_id":7,"domain":"app.example.io","status":"active","project_type":"laravel","web_directory":"/public"}}`)
		case "/api/servers/7/sites/9/certificates":
			certsRequested = true
			writeTestJSON(t, w, `{"data":[{"id":1,"site_id":9,"server_id":7,"type":"letsencrypt","domain":"app.example.io","status":"active","active":true,"expires_at":"2026-11-29 05:25:05"}],"links":{},"meta":{"current_page":1,"last_page":1}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.New("token", api.WithBaseURL(srv.URL)))
	server := api.Server{ID: 7, Name: "web-1"}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server}
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if cmd == nil || !model.(App).detail.SitesLoading {
		t.Fatal("sites tab should start a lazy sites request")
	}
	model, cmd = model.(App).Update(cmd())
	if !sitesRequested || !model.(App).detail.SitesLoaded {
		t.Fatal("sites response was not accepted")
	}
	model, cmd = model.(App).Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || model.(App).screen != siteDetailScreen {
		t.Fatal("enter should open the selected site")
	}
	model, cmd = model.(App).Update(cmd())
	if !siteRequested || cmd == nil {
		t.Fatal("site response should start certificate loading")
	}
	model, _ = model.(App).Update(cmd())
	appModel := model.(App)
	if !certsRequested || !appModel.siteDetail.CertificatesLoaded {
		t.Fatal("certificate response was not accepted")
	}
	for _, want := range []string{"app.example.io", "/public", "CERTIFICATES", "Let's Encrypt"} {
		if !strings.Contains(appModel.View(), want) {
			t.Errorf("site detail missing %q:\n%s", want, appModel.View())
		}
	}
}

func TestAppRestartConfirmationAndSuccess(t *testing.T) {
	var restarted bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/servers/7/restart" {
			http.NotFound(w, r)
			return
		}
		restarted = true
		writeTestJSON(t, w, `{"message":"Server is now rebooting"}`)
	}))
	t.Cleanup(srv.Close)

	app := NewApp(api.New("token", api.WithBaseURL(srv.URL)))
	server := api.Server{ID: 7, Name: "web-1", Status: "active"}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server}
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	if cmd != nil || !model.(App).restartConfirm || !strings.Contains(model.(App).View(), "Restart server?") {
		t.Fatal("ctrl+r should show the restart confirmation")
	}
	model, cmd = model.(App).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil || model.(App).restartConfirm {
		t.Fatal("y should submit the restart and close confirmation")
	}
	model, _ = model.(App).Update(cmd())
	appModel := model.(App)
	if !restarted || appModel.detail.Server.Status != "rebooting" || !strings.Contains(appModel.View(), "Restart sent") {
		t.Fatalf("restart result not shown: restarted=%v status=%q view=%s", restarted, appModel.detail.Server.Status, appModel.View())
	}
}

func TestAppUsesCachedServers(t *testing.T) {
	app := NewApp(api.New("token"))
	model, _ := app.Update(cachedServersLoadedMsg{servers: []api.Server{{ID: 7, Name: "cached-web"}}, ok: true})
	appModel := model.(App)
	if !appModel.serversLoaded || len(appModel.serverList.All()) != 1 || appModel.serverList.All()[0].Name != "cached-web" {
		t.Fatalf("cached servers were not displayed: %+v", appModel.serverList.All())
	}
}

func TestAppCopiesSelectedServerIP(t *testing.T) {
	app := NewApp(api.New("token"))
	app.user = &api.User{Email: "user@example.com"}
	app.serversLoaded = true
	app.serverList.SetServers([]api.Server{{ID: 7, Name: "web-1", IPAddress: "139.59.201.10"}})
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	appModel := model.(App)
	if cmd == nil {
		t.Fatal("copy should start a clipboard command")
	}
	model, _ = appModel.Update(ipCopiedMsg{})
	if !strings.Contains(model.(App).View(), "IP address copied") {
		t.Fatalf("copy feedback = %q", model.(App).View())
	}
}

func TestArrowKeyFinishesServerFilterAndNavigates(t *testing.T) {
	app := NewApp(api.New("token"))
	app.user = &api.User{Email: "user@example.com"}
	app.serversLoaded = true
	app.serverList.SetServers([]api.Server{
		{ID: 1, Name: "web-1"},
		{ID: 2, Name: "web-2"},
	})
	app.serverList.BeginFilter()
	app.serverList.UpdateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("web")})

	model, _ := app.Update(tea.KeyMsg{Type: tea.KeyDown})
	appModel := model.(App)
	if appModel.serverList.Filtering() {
		t.Fatal("down should finish filtering")
	}
	selected, ok := appModel.serverList.Selected()
	if !ok || selected.ID != 2 {
		t.Fatalf("selected = %+v, %v", selected, ok)
	}
}

func TestAppOpensServerInPloi(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1"}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server}

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("o should start the browser action")
	}
	model, _ = model.(App).Update(browserOpenedMsg{target: "server in Ploi"})
	if !strings.Contains(model.(App).View(), "Opened server in Ploi") {
		t.Fatalf("browser feedback = %q", model.(App).View())
	}
}

func TestBrowserCommand(t *testing.T) {
	url := "https://ploi.io/servers/7"
	command, args := browserCommand(url)
	if runtime.GOOS == "darwin" {
		if command != "open" || !slices.Equal(args, []string{url}) {
			t.Errorf("browserCommand() = %q, %v", command, args)
		}
		return
	}
	if command != "xdg-open" || !slices.Equal(args, []string{url}) {
		t.Errorf("browserCommand() = %q, %v", command, args)
	}
}

func TestSiteDetailActions(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1", IPAddress: "139.59.201.10"}
	site := api.Site{ID: 9, ServerID: 7, Domain: "app.example.io", SystemUser: "deploy"}
	app.screen = siteDetailScreen
	app.siteDetail = sitedetail.Model{Server: &server, Site: &site}

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatal("s should start SSH using the site's configured user")
	}
	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if cmd == nil {
		t.Fatal("o should open the site in Ploi")
	}
	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil {
		t.Fatal("b should open the site domain")
	}
}

func TestSiteDomainActionRequiresDomain(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7}
	site := api.Site{ID: 9}
	app.screen = siteDetailScreen
	app.siteDetail = sitedetail.Model{Server: &server, Site: &site}

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd == nil || !strings.Contains(model.(App).View(), "Site has no domain") {
		t.Fatal("empty site domain should show feedback")
	}
}

func TestClipboardCommand(t *testing.T) {
	original := lookPath
	t.Cleanup(func() { lookPath = original })
	lookPath = func(name string) (string, error) {
		if name == "xclip" {
			return "/usr/bin/xclip", nil
		}
		return "", os.ErrNotExist
	}
	if runtime.GOOS != "darwin" {
		command, args := clipboardCommand()
		if command != "xclip" || !slices.Equal(args, []string{"-selection", "clipboard"}) {
			t.Errorf("clipboardCommand() = %q, %v", command, args)
		}
	}
}

func TestAppShowsSSHErrorDetails(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1"}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server}
	model, _ := app.Update(sshFinishedMsg{err: fmt.Errorf("exit status 255"), output: "ssh: connect to host 139.59.201.10 port 22: Connection refused"})
	view := model.(App).View()
	if !strings.Contains(view, "SSH error") || !strings.Contains(view, "Connection refused") {
		t.Fatalf("SSH error details not rendered: %s", view)
	}
}

func TestAppLoadsPloiSystemUsersForSSH(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1", IPAddress: "139.59.201.10"}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server}
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	appModel := model.(App)
	if cmd == nil || !appModel.sshSelecting || !appModel.sshLoading {
		t.Fatal("s should open the API-backed SSH user picker")
	}
	model, _ = appModel.Update(systemUsersLoadedMsg{
		requestID: appModel.systemUsersRequestID,
		serverID:  7,
		users: []api.SystemUser{
			{Name: "deploy", Root: "/home/deploy"},
			{Name: "ploi", Root: "/home/ploi"},
		},
	})
	appModel = model.(App)
	if appModel.sshLoading || len(appModel.sshUsers) != 2 {
		t.Fatalf("SSH users = %+v", appModel.sshUsers)
	}
	for _, want := range []string{"SSH USERS", "deploy", "ploi", "/home/deploy"} {
		if !strings.Contains(appModel.View(), want) {
			t.Errorf("SSH picker missing %q:\n%s", want, appModel.View())
		}
	}
}

func TestSSHUsernameValidation(t *testing.T) {
	for _, user := range []string{"ploi", "deploy", "customer-app", "_service", "Deploy2"} {
		if !validSSHUsername(user) {
			t.Errorf("validSSHUsername(%q) = false", user)
		}
	}
	for _, user := range []string{"", "-oProxyCommand=touch /tmp/pwned", "user@host", "two words", "user.name", "user\x1b[31m"} {
		if validSSHUsername(user) {
			t.Errorf("validSSHUsername(%q) = true", user)
		}
	}
}

func TestSSHCommandTerminatesOptionParsing(t *testing.T) {
	cmd := sshCommand("deploy", "139.59.201.10")
	if !slices.Equal(cmd.Args, []string{"ssh", "--", "deploy@139.59.201.10"}) {
		t.Fatalf("ssh args = %q", cmd.Args)
	}
}

func TestSSHUsersExcludeInvalidNames(t *testing.T) {
	users := sshUsers([]api.SystemUser{
		{Name: "deploy", Root: "/home/deploy"},
		{Name: "-oProxyCommand=malicious"},
		{Name: "user@host"},
	})
	if len(users) != 2 {
		t.Fatalf("ssh users = %+v", users)
	}
	for _, user := range users {
		if user.Name != "ploi" && user.Name != "deploy" {
			t.Fatalf("invalid SSH user retained: %+v", user)
		}
	}
}

func TestAppRejectsInvalidSiteSSHUsername(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, IPAddress: "139.59.201.10"}
	site := api.Site{ID: 9, SystemUser: "-oProxyCommand=malicious"}
	app.screen = siteDetailScreen
	app.siteDetail = sitedetail.Model{Server: &server, Site: &site}

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil || !strings.Contains(model.(App).View(), "Invalid SSH username") {
		t.Fatalf("invalid SSH username feedback = %q", model.(App).View())
	}
}

func TestAppFallsBackToPloiWhenSystemUsersAreUnavailable(t *testing.T) {
	app := NewApp(api.New("token"))
	server := api.Server{ID: 7, Name: "web-1", IPAddress: "139.59.201.10"}
	app.screen = serverDetailScreen
	app.detail = serverdetail.Model{Server: &server}
	app.systemUsersRequestID = 1
	app.sshSelecting = true
	app.sshLoading = true
	model, cmd := app.Update(systemUsersLoadedMsg{
		requestID: 1,
		serverID:  7,
		err:       &api.APIError{StatusCode: http.StatusNotFound, Message: "Unable to find this record."},
	})
	appModel := model.(App)
	if cmd == nil || appModel.sshSelecting || !strings.Contains(appModel.View(), "connecting as ploi") {
		t.Fatalf("system-user fallback was not shown: %s", appModel.View())
	}
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	if _, err := w.Write([]byte(body)); err != nil {
		t.Error(err)
	}
}

func TestAppHelpAndQuitKeys(t *testing.T) {
	app := NewApp(api.New("token"))
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if cmd != nil || !model.(App).help {
		t.Fatal("help key did not show the help overlay")
	}
	if !strings.Contains(model.(App).View(), "? HELP") {
		t.Fatal("help overlay is not rendered")
	}

	_, cmd = model.(App).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("q should quit from the help overlay")
	}
}

func TestAppRefreshShowsToast(t *testing.T) {
	app := NewApp(api.New("token"))
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("refresh should start account reload and toast timer")
	}
	if !strings.Contains(model.(App).View(), "Refreshing view...") {
		t.Fatal("refresh toast is not rendered")
	}
}
