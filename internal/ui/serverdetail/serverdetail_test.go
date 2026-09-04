package serverdetail

import (
	"strings"
	"testing"
	"time"

	"github.com/pdaether/ploi-tui/internal/api"
)

func TestOverviewView(t *testing.T) {
	server := &api.Server{
		ID:         7,
		Name:       "web-1",
		IPAddress:  "139.59.201.10",
		PHPVersion: 8.3,
		SitesCount: 12,
		Status:     "active",
		Monitoring: true,
		CreatedAt:  api.Time{Time: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)},
	}
	m := Model{Server: server, DatabasesLoaded: true, Databases: []api.Database{{Type: "postgresql"}}, Samples: []api.MonitoringSample{{CPU: 5, RAM: 22, Disk: 40}}}
	view := m.View(80)
	for _, want := range []string{"web-1", "Server ID", "7", "139.59.201.10", "active", "8.3", "12", "PostgreSQL (1)", "QUICK STATS", "CPU"} {
		if !strings.Contains(view, want) {
			t.Errorf("overview missing %q:\n%s", want, view)
		}
	}
}

func TestMonitoringUsesThreeLineChartsWhenThereIsRoom(t *testing.T) {
	m := Model{
		Server:           &api.Server{ID: 7, Monitoring: true},
		Tab:              MonitoringTab,
		MonitoringLoaded: true,
		Samples: []api.MonitoringSample{
			{CPU: 5, RAM: 21, Disk: 40, LoadAverage: 0.2},
			{CPU: 80, RAM: 75, Disk: 60, LoadAverage: 1.4},
		},
	}
	expanded := m.View(80, 18)
	for _, want := range []string{"100 │", " 50 │", "  0 │"} {
		if !strings.Contains(expanded, want) {
			t.Errorf("expanded monitoring missing %q:\n%s", want, expanded)
		}
	}
	compact := m.View(80, 17)
	if strings.Contains(compact, "100 │") {
		t.Errorf("compact monitoring should use sparklines:\n%s", compact)
	}
}

func TestMonitoringViewAndUnavailableNotice(t *testing.T) {
	server := &api.Server{ID: 7, Name: "web-1", Monitoring: true}
	m := Model{
		Server:           server,
		Tab:              MonitoringTab,
		MonitoringLoaded: true,
		Samples: []api.MonitoringSample{
			{CPU: 5.2, RAM: 21.8, Disk: 40, LoadAverage: 0.19, Date: api.Time{Time: time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC)}},
			{CPU: 8, RAM: 25, Disk: 42, LoadAverage: 0.3, Date: api.Time{Time: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
		},
	}
	view := m.View(80)
	for _, want := range []string{"MONITORING", "CPU %", "MEMORY %", "LOAD AVG", "DISK %", "now 5.2%"} {
		if !strings.Contains(view, want) {
			t.Errorf("monitoring missing %q:\n%s", want, view)
		}
	}

	view = (Model{Server: &api.Server{Name: "web-1"}, Tab: MonitoringTab}).View(80)
	for _, want := range []string{"MONITORING NOT AVAILABLE", "ploi.io/pricing"} {
		if !strings.Contains(view, want) {
			t.Errorf("unavailable view missing %q:\n%s", want, view)
		}
	}
}

func TestLoadingAndErrorViews(t *testing.T) {
	server := &api.Server{Name: "web-1"}
	if got := (Model{Server: server, Loading: true}).View(80); !strings.Contains(got, "Loading server") {
		t.Errorf("loading view = %q", got)
	}
	if got := (Model{Server: server, Err: &api.APIError{StatusCode: 500, Message: "failed"}}).View(80); !strings.Contains(got, "API error") {
		t.Errorf("error view = %q", got)
	}
}

func TestDatabaseSummary(t *testing.T) {
	tests := []struct {
		name  string
		model Model
		want  string
	}{
		{name: "none", model: Model{DatabasesLoaded: true}, want: "none"},
		{name: "postgres", model: Model{DatabasesLoaded: true, Databases: []api.Database{{Type: "postgresql"}}}, want: "PostgreSQL (1)"},
		{name: "mixed", model: Model{DatabasesLoaded: true, Databases: []api.Database{{Type: "postgres"}, {Type: "mysql"}, {Type: "mysql"}}}, want: "PostgreSQL (1), MySQL (2)"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.model.databaseSummary(); !strings.Contains(got, test.want) {
				t.Errorf("databaseSummary() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDatabaseSummaryDoesNotInferMySQLFromServerVersion(t *testing.T) {
	m := Model{
		Server:          &api.Server{MySQLVersion: 8},
		DatabasesLoaded: true,
	}
	if got := m.databaseSummary(); !strings.Contains(got, "none") {
		t.Errorf("databaseSummary() = %q, want no managed databases", got)
	}
}

func TestSitesViewAndSelection(t *testing.T) {
	m := Model{
		Server:      &api.Server{ID: 7, Name: "web-1"},
		Tab:         SitesTab,
		SitesLoaded: true,
		Sites: []api.Site{
			{ID: 1, Domain: "app.example.io", Status: "active", ProjectType: "laravel", PHPVersion: 8.3, DiskUsage: &api.DiskUsage{Human: "1.2 GB"}},
			{ID: 2, Domain: "legacy.example.io", Status: "suspended", ProjectType: "static"},
		},
	}
	view := m.View(100)
	for _, want := range []string{"SITES (2)", "app.example.io", "active", "laravel", "1.2 GB", "legacy.example.io", "suspended"} {
		if !strings.Contains(view, want) {
			t.Errorf("sites view missing %q:\n%s", want, view)
		}
	}
	m.MoveSite(1)
	selected, ok := m.SelectedSite()
	if !ok || selected.ID != 2 {
		t.Errorf("selected site = %+v, %v", selected, ok)
	}
}
