package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/store"
	"github.com/pdaether/ploi-tui/internal/ui/components"
	"github.com/pdaether/ploi-tui/internal/ui/serverdetail"
	"github.com/pdaether/ploi-tui/internal/ui/serverlist"
	"github.com/pdaether/ploi-tui/internal/ui/sitedetail"
)

const (
	defaultWidth  = 80
	defaultHeight = 24
)

type screen int

const (
	serverListScreen screen = iota
	serverDetailScreen
	siteDetailScreen
)

type toast struct {
	id      int
	message string
}

type App struct {
	client *api.Client
	cache  *store.Cache
	user   *api.User
	err    error

	width          int
	height         int
	help           bool
	restartConfirm bool
	sshSelecting   bool
	sshUsers       []api.SystemUser
	sshCursor      int
	sshLoading     bool
	sshErr         error
	toast          *toast
	sshError       string
	sshErrorID     int
	nextID         int

	screen                screen
	serversLoaded         bool
	serversLoading        bool
	serverErr             error
	serverList            serverlist.Model
	detail                serverdetail.Model
	siteDetail            sitedetail.Model
	userRequestID         uint64
	serverRequestID       uint64
	detailRequestID       uint64
	databaseRequestID     uint64
	monitoringRequestID   uint64
	monitoringGeneration  uint64
	sitesRequestID        uint64
	siteRequestID         uint64
	certificatesRequestID uint64
	systemUsersRequestID  uint64
}

type Option func(*App)

func WithCache(cache *store.Cache) Option {
	return func(a *App) { a.cache = cache }
}

func NewApp(client *api.Client, opts ...Option) App {
	servers := serverlist.New()
	servers.SetHeight(defaultHeight - 4)
	servers.SetWidth(defaultWidth - 2)
	app := App{
		client:     client,
		width:      defaultWidth,
		height:     defaultHeight,
		serverList: servers,
	}
	for _, opt := range opts {
		opt(&app)
	}
	return app
}

func (a App) Init() tea.Cmd {
	if a.cache == nil {
		return a.loadUser
	}
	return tea.Batch(a.loadUser, a.loadCachedServers)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.serverList.SetHeight(max(0, msg.Height-4))
		a.serverList.SetWidth(max(0, msg.Width-2))

	case tea.KeyMsg:
		return a, a.handleKey(msg)

	case restartFinishedMsg:
		if msg.err != nil {
			a.toast = a.newToast("Unable to restart server")
			return a, clearToast(a.toast.id)
		}
		if a.detail.Server != nil {
			a.detail.Server.Status = "rebooting"
			a.serverList.UpdateServer(*a.detail.Server)
		}
		a.toast = a.newToast("Restart sent")
		if a.detail.Server != nil {
			return a, tea.Batch(clearToast(a.toast.id), a.restartPoll(a.detail.Server.ID))
		}
		return a, clearToast(a.toast.id)

	case restartPollMsg:
		if a.screen == serverDetailScreen && a.detail.Server != nil && a.detail.Server.ID == msg.serverID {
			return a, a.beginLoadServer(msg.serverID)
		}

	case sshFinishedMsg:
		if msg.err != nil {
			detail := strings.TrimSpace(msg.output)
			if detail == "" {
				detail = msg.err.Error()
			}
			a.sshError = components.Truncate(detail, 600)
			a.nextID++
			a.sshErrorID = a.nextID
			a.toast = a.newToast("SSH failed; details shown below")
			return a, tea.Batch(clearToast(a.toast.id), clearSSHErrorAfter(a.sshErrorID, 5*time.Second), tea.ClearScreen)
		}
		a.sshError = ""
		a.sshErrorID = 0
		// The exec left its output on the terminal and bubbletea does not
		// erase it after resuming, so force a full repaint.
		return a, tea.ClearScreen

	case toastClearedMsg:
		if a.toast != nil && a.toast.id == msg.id {
			a.toast = nil
		}

	case sshErrorClearedMsg:
		if a.sshErrorID == msg.id {
			a.sshError = ""
			a.sshErrorID = 0
		}

	case ipCopiedMsg:
		if msg.err != nil {
			a.toast = a.newToast("Unable to copy IP: " + msg.err.Error())
		} else {
			a.toast = a.newToast("IP address copied")
		}
		return a, clearToastAfter(a.toast.id, 2*time.Second)

	case browserOpenedMsg:
		if msg.err != nil {
			a.toast = a.newToast("Unable to open " + msg.target + ": " + msg.err.Error())
		} else {
			a.toast = a.newToast("Opened " + msg.target)
		}
		return a, clearToast(a.toast.id)

	default:
		if cmd, handled := a.handleRequestMsg(msg); handled {
			return a, cmd
		}
	}

	return a, nil
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if a.screen == serverListScreen && a.serverList.Filtering() && (key == "up" || key == "down") {
		a.serverList.UpdateFilter(tea.KeyMsg{Type: tea.KeyEnter})
		return a.handleListKey(key)
	}
	if a.screen == serverListScreen && a.serverList.Filtering() && key != "ctrl+c" {
		a.serverList.UpdateFilter(msg)
		return nil
	}
	switch key {
	case "ctrl+c", "q":
		return tea.Quit
	case "?":
		a.help = !a.help
		return nil
	}

	if a.help {
		if key == "esc" {
			a.help = false
		}
		return nil
	}
	if a.restartConfirm {
		switch key {
		case "y":
			a.restartConfirm = false
			if a.detail.Server != nil {
				return a.restartServer(a.detail.Server.ID)
			}
		case "n", "esc":
			a.restartConfirm = false
		}
		return nil
	}
	if a.sshSelecting {
		switch key {
		case "esc":
			a.sshSelecting = false
		case "j", "down":
			if len(a.sshUsers) > 0 {
				a.sshCursor = min(len(a.sshUsers)-1, a.sshCursor+1)
			}
		case "k", "up":
			if len(a.sshUsers) > 0 {
				a.sshCursor = max(0, a.sshCursor-1)
			}
		case "enter":
			if !a.sshLoading && a.sshErr == nil && a.sshCursor < len(a.sshUsers) && a.detail.Server != nil {
				user := a.sshUsers[a.sshCursor]
				a.sshSelecting = false
				return a.ssh(a.detail.Server, user.Name)
			}
		}
		return nil
	}

	switch key {
	case "esc":
		switch a.screen {
		case siteDetailScreen:
			a.screen = serverDetailScreen
			a.siteDetail = sitedetail.Model{}
		case serverDetailScreen:
			a.screen = serverListScreen
			a.detail = serverdetail.Model{}
		}
		return nil
	case "ctrl+r":
		if a.screen == serverDetailScreen && a.detail.Server != nil {
			a.restartConfirm = true
		}
		return nil
	case "s":
		if a.screen == serverDetailScreen && a.detail.Server != nil {
			return a.beginLoadSystemUsers()
		}
		if a.screen == siteDetailScreen {
			return a.handleSiteDetailKey(key)
		}
		return nil
	case "c":
		if a.screen == serverDetailScreen && a.detail.Server != nil {
			return a.copyIP(*a.detail.Server)
		}
		if a.screen == serverListScreen {
			if server, ok := a.serverList.Selected(); ok {
				return a.copyIP(server)
			}
		}
		return nil
	case "r":
		a.toast = a.newToast("Refreshing view...")
		var cmd tea.Cmd
		if a.err != nil || a.user == nil {
			cmd = a.beginLoadUser()
		} else if a.screen == serverDetailScreen || a.screen == siteDetailScreen {
			cmd = a.refreshDetail()
		} else {
			cmd = a.refreshServers()
		}
		return tea.Batch(cmd, clearToast(a.toast.id))
	case "R":
		a.toast = a.newToast("Refreshing all data...")
		a.serversLoaded = false
		a.serversLoading = true
		a.serverErr = nil
		cmds := []tea.Cmd{a.beginLoadUser()}
		if a.screen == serverDetailScreen && a.detail.Server != nil {
			a.monitoringGeneration++
			cmds = append(cmds, a.beginLoadServer(a.detail.Server.ID))
		} else if a.screen == siteDetailScreen && a.siteDetail.Server != nil && a.siteDetail.Site != nil {
			cmds = append(cmds, a.beginLoadSite(a.siteDetail.Server.ID, a.siteDetail.Site.ID))
		}
		cmds = append(cmds, clearToast(a.toast.id))
		return tea.Batch(cmds...)
	}

	if a.screen == serverListScreen {
		return a.handleListKey(key)
	}
	if a.screen == siteDetailScreen {
		return a.handleSiteDetailKey(key)
	}
	return a.handleDetailKey(key)
}

func (a *App) handleListKey(key string) tea.Cmd {
	switch key {
	case "j", "down":
		a.serverList.Move(1)
	case "k", "up":
		a.serverList.Move(-1)
	case "/":
		a.serverList.BeginFilter()
	case "enter":
		selected, ok := a.serverList.Selected()
		if !ok {
			return nil
		}
		a.openServer(selected)
		return a.beginLoadServer(selected.ID)
	}
	return nil
}

func (a *App) handleDetailKey(key string) tea.Cmd {
	switch key {
	case "j", "down":
		if a.detail.Tab == serverdetail.SitesTab {
			a.detail.MoveSite(1)
		}
	case "k", "up":
		if a.detail.Tab == serverdetail.SitesTab {
			a.detail.MoveSite(-1)
		}
	case "enter":
		if a.detail.Tab == serverdetail.SitesTab {
			if site, ok := a.detail.SelectedSite(); ok && a.detail.Server != nil {
				a.openSite(site)
				return a.beginLoadSite(a.detail.Server.ID, site.ID)
			}
		}
	case "h", "left":
		return a.selectTab(a.previousTab())
	case "l", "right":
		return a.selectTab(a.nextTab())
	case "1":
		return a.selectTab(serverdetail.OverviewTab)
	case "2":
		return a.selectTab(serverdetail.MonitoringTab)
	case "3":
		return a.selectTab(serverdetail.SitesTab)
	case "o":
		if a.detail.Server != nil {
			return a.openPloiServer(*a.detail.Server)
		}
	}
	return nil
}

func (a *App) handleSiteDetailKey(key string) tea.Cmd {
	if a.siteDetail.Server == nil || a.siteDetail.Site == nil {
		return nil
	}
	switch key {
	case "s":
		return a.ssh(a.siteDetail.Server, a.siteDetail.Site.SystemUser)
	case "o":
		return a.openPloiSite(*a.siteDetail.Server, *a.siteDetail.Site)
	case "b":
		return a.openSiteDomain(*a.siteDetail.Site)
	}
	return nil
}

func (a *App) refreshServers() tea.Cmd {
	return a.beginLoadServers()
}

func (a *App) refreshDetail() tea.Cmd {
	if a.screen == siteDetailScreen {
		if a.siteDetail.Server == nil || a.siteDetail.Site == nil {
			return nil
		}
		return a.beginLoadSite(a.siteDetail.Server.ID, a.siteDetail.Site.ID)
	}
	if a.detail.Server == nil {
		return nil
	}
	if a.detail.Tab == serverdetail.MonitoringTab {
		return a.refreshMonitoring()
	}
	if a.detail.Tab == serverdetail.SitesTab {
		return a.beginLoadSites()
	}
	return a.beginLoadServer(a.detail.Server.ID)
}

func (a *App) refreshMonitoring() tea.Cmd {
	if a.detail.Server == nil || !a.detail.Server.Monitoring {
		return nil
	}
	return a.beginLoadMonitoring()
}

func (a *App) openServer(server api.Server) {
	serverCopy := server
	a.screen = serverDetailScreen
	a.monitoringGeneration++
	a.detail = serverdetail.Model{Server: &serverCopy, Loading: true}
}

func (a *App) openSite(site api.Site) {
	siteCopy := site
	a.screen = siteDetailScreen
	a.siteDetail = sitedetail.Model{Server: a.detail.Server, Site: &siteCopy, Loading: true}
}

func (a *App) selectTab(tab serverdetail.Tab) tea.Cmd {
	if a.detail.Server == nil {
		return nil
	}
	if tab == serverdetail.SitesTab {
		if a.detail.Tab == serverdetail.MonitoringTab {
			a.monitoringGeneration++
			a.detail.MonitoringLoading = false
		}
		a.detail.Tab = tab
		if a.detail.Loading || a.detail.SitesLoaded || a.detail.SitesLoading {
			return nil
		}
		return a.beginLoadSites()
	}
	if tab != serverdetail.MonitoringTab {
		if a.detail.Tab == serverdetail.MonitoringTab {
			a.monitoringGeneration++
			a.detail.MonitoringLoading = false
		}
		a.detail.Tab = tab
		return nil
	}
	a.detail.Tab = tab
	if a.detail.Loading || a.detail.MonitoringLoaded || a.detail.MonitoringLoading {
		if a.detail.MonitoringLoaded && a.detail.Server.Monitoring {
			return a.monitoringTick(a.detail.Server.ID, a.monitoringGeneration)
		}
		return nil
	}
	return a.beginLoadMonitoring()
}

func (a App) previousTab() serverdetail.Tab {
	if a.detail.Tab == serverdetail.OverviewTab {
		return serverdetail.SitesTab
	}
	return a.detail.Tab - 1
}

func (a App) nextTab() serverdetail.Tab {
	if a.detail.Tab == serverdetail.SitesTab {
		return serverdetail.OverviewTab
	}
	return a.detail.Tab + 1
}
