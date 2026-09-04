package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/ui/serverdetail"
)

type userLoadedMsg struct {
	requestID uint64
	user      *api.User
	err       error
}

type serversLoadedMsg struct {
	requestID uint64
	servers   []api.Server
	err       error
}

type cachedServersLoadedMsg struct {
	servers []api.Server
	ok      bool
}

type serverLoadedMsg struct {
	requestID uint64
	serverID  int64
	server    *api.Server
	err       error
}

type databasesLoadedMsg struct {
	requestID uint64
	serverID  int64
	databases []api.Database
	err       error
}

type monitoringLoadedMsg struct {
	requestID  uint64
	generation uint64
	serverID   int64
	samples    []api.MonitoringSample
	err        error
}

type monitoringTickMsg struct {
	serverID   int64
	generation uint64
}

type sitesLoadedMsg struct {
	requestID uint64
	serverID  int64
	sites     []api.Site
	err       error
}

type siteLoadedMsg struct {
	requestID uint64
	serverID  int64
	siteID    int64
	site      *api.Site
	err       error
}

type certificatesLoadedMsg struct {
	requestID uint64
	serverID  int64
	siteID    int64
	certs     []api.Certificate
	err       error
}

type systemUsersLoadedMsg struct {
	requestID uint64
	serverID  int64
	users     []api.SystemUser
	err       error
}

func (a *App) handleRequestMsg(message tea.Msg) (tea.Cmd, bool) {
	switch msg := message.(type) {
	case userLoadedMsg:
		if msg.requestID != a.userRequestID {
			return nil, true
		}
		a.user, a.err = msg.user, msg.err
		if msg.err != nil {
			a.serversLoading = false
			a.toast = a.newToast("Unable to refresh account")
			return clearToast(a.toast.id), true
		}
		a.serversLoaded = false
		return a.beginLoadServers(), true

	case serversLoadedMsg:
		if msg.requestID != a.serverRequestID {
			return nil, true
		}
		a.serversLoading = false
		a.serversLoaded = true
		a.serverErr = msg.err
		if msg.err == nil {
			a.serverList.SetServers(msg.servers)
			if a.cache != nil {
				_ = a.cache.SaveServers(msg.servers)
			}
			return nil, true
		}
		a.toast = a.newToast("Unable to load servers")
		return clearToast(a.toast.id), true

	case cachedServersLoadedMsg:
		if msg.ok && !a.serversLoaded {
			a.serverList.SetServers(msg.servers)
			a.serversLoaded = true
		}
		return nil, true

	case serverLoadedMsg:
		if msg.requestID != a.detailRequestID || a.detail.Server == nil || a.detail.Server.ID != msg.serverID {
			return nil, true
		}
		if msg.err == nil && msg.server == nil {
			msg.err = fmt.Errorf("empty server response")
		}
		a.detail.Loading = false
		a.detail.Err = msg.err
		if msg.err == nil {
			a.detail.Server = msg.server
			a.serverList.UpdateServer(*msg.server)
			cmds := []tea.Cmd{a.beginLoadDatabases()}
			if a.detail.Tab == serverdetail.MonitoringTab || msg.server.Monitoring {
				cmds = append(cmds, a.beginLoadMonitoring())
			}
			if a.detail.Tab == serverdetail.SitesTab {
				cmds = append(cmds, a.beginLoadSites())
			}
			return tea.Batch(cmds...), true
		}
		a.toast = a.newToast("Unable to load server")
		return clearToast(a.toast.id), true

	case databasesLoadedMsg:
		if msg.requestID != a.databaseRequestID || a.detail.Server == nil || a.detail.Server.ID != msg.serverID {
			return nil, true
		}
		a.detail.DatabasesLoading = false
		a.detail.DatabasesLoaded = msg.err == nil
		a.detail.Databases = msg.databases
		a.detail.DatabasesErr = msg.err
		return nil, true

	case monitoringLoadedMsg:
		if msg.requestID != a.monitoringRequestID || msg.generation != a.monitoringGeneration ||
			a.screen != serverDetailScreen ||
			a.detail.Server == nil || a.detail.Server.ID != msg.serverID {
			return nil, true
		}
		a.detail.MonitoringLoading = false
		a.detail.MonitoringErr = msg.err
		if msg.err == nil {
			a.detail.MonitoringLoaded = true
			a.detail.Samples = msg.samples
			if a.detail.Tab == serverdetail.MonitoringTab {
				return a.monitoringTick(msg.serverID, a.monitoringGeneration), true
			}
			return nil, true
		}
		a.detail.MonitoringLoaded = false
		if a.detail.Tab == serverdetail.MonitoringTab {
			a.toast = a.newToast("Unable to load monitoring")
			return clearToast(a.toast.id), true
		}
		return nil, true

	case monitoringTickMsg:
		if a.screen != serverDetailScreen || a.detail.Tab != serverdetail.MonitoringTab ||
			a.detail.Server == nil || a.detail.Server.ID != msg.serverID ||
			a.monitoringGeneration != msg.generation || a.detail.MonitoringLoading {
			return nil, true
		}
		return a.beginLoadMonitoring(), true

	case sitesLoadedMsg:
		if msg.requestID != a.sitesRequestID || a.screen != serverDetailScreen || a.detail.Server == nil || a.detail.Server.ID != msg.serverID {
			return nil, true
		}
		a.detail.SitesLoading = false
		a.detail.SitesLoaded = msg.err == nil
		a.detail.SitesErr = msg.err
		if msg.err == nil {
			a.detail.SetSites(msg.sites)
		} else if a.detail.Tab == serverdetail.SitesTab {
			a.toast = a.newToast("Unable to load sites")
			return clearToast(a.toast.id), true
		}
		return nil, true

	case siteLoadedMsg:
		if msg.requestID != a.siteRequestID || a.screen != siteDetailScreen || a.siteDetail.Server == nil || a.siteDetail.Site == nil ||
			a.siteDetail.Server.ID != msg.serverID || a.siteDetail.Site.ID != msg.siteID {
			return nil, true
		}
		if msg.err == nil && msg.site == nil {
			msg.err = fmt.Errorf("empty site response")
		}
		a.siteDetail.Loading = false
		a.siteDetail.Err = msg.err
		if msg.err == nil {
			a.siteDetail.Site = msg.site
			return a.beginLoadCertificates(), true
		}
		a.toast = a.newToast("Unable to load site")
		return clearToast(a.toast.id), true

	case certificatesLoadedMsg:
		if msg.requestID != a.certificatesRequestID || a.screen != siteDetailScreen || a.siteDetail.Server == nil || a.siteDetail.Site == nil ||
			a.siteDetail.Server.ID != msg.serverID || a.siteDetail.Site.ID != msg.siteID {
			return nil, true
		}
		a.siteDetail.CertificatesLoading = false
		a.siteDetail.CertificatesLoaded = msg.err == nil
		a.siteDetail.Certificates = msg.certs
		a.siteDetail.CertificatesErr = msg.err
		if msg.err != nil {
			a.toast = a.newToast("Unable to load certificates")
			return clearToast(a.toast.id), true
		}
		return nil, true

	case systemUsersLoadedMsg:
		if msg.requestID != a.systemUsersRequestID || a.screen != serverDetailScreen || a.detail.Server == nil || a.detail.Server.ID != msg.serverID {
			return nil, true
		}
		a.sshLoading = false
		a.sshErr = msg.err
		if systemUsersUnavailable(msg.err) {
			a.sshSelecting = false
			a.toast = a.newToast("System users unavailable; connecting as ploi")
			return tea.Batch(a.ssh(a.detail.Server, "ploi"), clearToast(a.toast.id)), true
		}
		if msg.err != nil {
			return nil, true
		}
		a.sshUsers = sshUsers(msg.users)
		sort.SliceStable(a.sshUsers, func(i, j int) bool { return a.sshUsers[i].Name < a.sshUsers[j].Name })
		return nil, true
	}
	return nil, false
}

func (a App) loadUser() tea.Msg {
	user, err := a.client.User(a.context())
	return userLoadedMsg{requestID: a.userRequestID, user: user, err: err}
}

func (a App) loadServers() tea.Msg {
	servers, err := a.client.Servers(a.context())
	return serversLoadedMsg{requestID: a.serverRequestID, servers: servers, err: err}
}

func (a App) loadCachedServers() tea.Msg {
	servers, ok, _ := a.cache.LoadServers()
	return cachedServersLoadedMsg{servers: servers, ok: ok}
}

func (a App) loadServer(id int64) tea.Cmd {
	requestID := a.detailRequestID
	return func() tea.Msg {
		server, err := a.client.Server(a.context(), id)
		return serverLoadedMsg{requestID: requestID, serverID: id, server: server, err: err}
	}
}

func (a App) loadMonitoring(id int64) tea.Cmd {
	requestID := a.monitoringRequestID
	generation := a.monitoringGeneration
	return func() tea.Msg {
		samples, err := a.client.Monitoring(a.context(), id)
		return monitoringLoadedMsg{requestID: requestID, generation: generation, serverID: id, samples: samples, err: err}
	}
}

func (a App) loadDatabases(id int64) tea.Cmd {
	requestID := a.databaseRequestID
	return func() tea.Msg {
		databases, err := a.client.Databases(a.context(), id)
		return databasesLoadedMsg{requestID: requestID, serverID: id, databases: databases, err: err}
	}
}

func (a App) loadSites(id int64) tea.Cmd {
	requestID := a.sitesRequestID
	return func() tea.Msg {
		sites, err := a.client.Sites(a.context(), id)
		return sitesLoadedMsg{requestID: requestID, serverID: id, sites: sites, err: err}
	}
}

func (a App) loadSite(serverID, siteID int64) tea.Cmd {
	requestID := a.siteRequestID
	return func() tea.Msg {
		site, err := a.client.Site(a.context(), serverID, siteID)
		return siteLoadedMsg{requestID: requestID, serverID: serverID, siteID: siteID, site: site, err: err}
	}
}

func (a App) loadCertificates(serverID, siteID int64) tea.Cmd {
	requestID := a.certificatesRequestID
	return func() tea.Msg {
		certs, err := a.client.Certificates(a.context(), serverID, siteID)
		return certificatesLoadedMsg{requestID: requestID, serverID: serverID, siteID: siteID, certs: certs, err: err}
	}
}

func (a App) loadSystemUsers(serverID int64) tea.Cmd {
	requestID := a.systemUsersRequestID
	return func() tea.Msg {
		users, err := a.client.SystemUsers(a.context(), serverID)
		return systemUsersLoadedMsg{requestID: requestID, serverID: serverID, users: users, err: err}
	}
}

func (a App) monitoringTick(id int64, generation uint64) tea.Cmd {
	return tea.Tick(time.Minute, func(time.Time) tea.Msg {
		return monitoringTickMsg{serverID: id, generation: generation}
	})
}

func (a *App) beginLoadUser() tea.Cmd {
	a.userRequestID++
	a.err = nil
	a.serverRequestID++
	a.serversLoaded = false
	a.serversLoading = true
	a.serverErr = nil
	return a.loadUser
}

func (a *App) beginLoadServers() tea.Cmd {
	a.serverRequestID++
	a.serversLoading = true
	a.serverErr = nil
	return a.loadServers
}

func (a *App) beginLoadServer(id int64) tea.Cmd {
	a.detailRequestID++
	a.databaseRequestID++
	a.monitoringGeneration++
	a.sitesRequestID++
	a.detail.MonitoringLoading = false
	a.detail.DatabasesLoading = false
	a.detail.SitesLoading = false
	a.detail.Loading = true
	a.detail.Err = nil
	return a.loadServer(id)
}

func (a *App) beginLoadSites() tea.Cmd {
	if a.detail.Server == nil {
		return nil
	}
	a.sitesRequestID++
	a.detail.SitesLoaded = false
	a.detail.SitesLoading = true
	a.detail.SitesErr = nil
	return a.loadSites(a.detail.Server.ID)
}

func (a *App) beginLoadSite(serverID, siteID int64) tea.Cmd {
	a.siteRequestID++
	a.certificatesRequestID++
	a.siteDetail.Loading = true
	a.siteDetail.Err = nil
	a.siteDetail.CertificatesLoading = false
	return a.loadSite(serverID, siteID)
}

func (a *App) beginLoadCertificates() tea.Cmd {
	if a.siteDetail.Server == nil || a.siteDetail.Site == nil {
		return nil
	}
	a.certificatesRequestID++
	a.siteDetail.CertificatesLoaded = false
	a.siteDetail.CertificatesLoading = true
	a.siteDetail.CertificatesErr = nil
	return a.loadCertificates(a.siteDetail.Server.ID, a.siteDetail.Site.ID)
}

func (a *App) beginLoadSystemUsers() tea.Cmd {
	if a.detail.Server == nil {
		return nil
	}
	a.systemUsersRequestID++
	a.sshSelecting = true
	a.sshLoading = true
	a.sshErr = nil
	a.sshUsers = nil
	a.sshCursor = 0
	return a.loadSystemUsers(a.detail.Server.ID)
}

func (a *App) beginLoadDatabases() tea.Cmd {
	if a.detail.Server == nil {
		return nil
	}
	a.databaseRequestID++
	a.detail.DatabasesLoaded = false
	a.detail.DatabasesLoading = true
	a.detail.DatabasesErr = nil
	return a.loadDatabases(a.detail.Server.ID)
}

func (a *App) beginLoadMonitoring() tea.Cmd {
	a.monitoringRequestID++
	a.monitoringGeneration++
	if a.detail.Server == nil || !a.detail.Server.Monitoring {
		a.detail.MonitoringLoaded = true
		a.detail.MonitoringLoading = false
		a.detail.MonitoringErr = nil
		a.detail.Samples = nil
		return nil
	}
	a.detail.MonitoringLoaded = false
	a.detail.MonitoringLoading = true
	a.detail.MonitoringErr = nil
	return a.loadMonitoring(a.detail.Server.ID)
}

func (a App) context() context.Context { return context.Background() }
