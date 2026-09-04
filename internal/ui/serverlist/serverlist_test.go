package serverlist

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pdaether/ploi-tui/internal/api"
)

func testServers() []api.Server {
	return []api.Server{
		{ID: 1, Name: "web-1", IPAddress: "10.0.0.1", Status: "active"},
		{ID: 2, Name: "db-prod", IPAddress: "10.0.0.2", Status: "rebooting"},
		{ID: 3, Name: "staging", IPAddress: "10.0.0.3", Status: "unreachable"},
		{ID: 4, Name: "worker", IPAddress: "10.0.0.4", Status: "active"},
	}
}

func TestFilterAndSelection(t *testing.T) {
	m := New()
	m.SetServers(testServers())
	m.BeginFilter()
	m.UpdateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("prod")})
	if got := len(m.Visible()); got != 1 {
		t.Fatalf("visible count = %d, want 1", got)
	}
	selected, ok := m.Selected()
	if !ok || selected.Name != "db-prod" {
		t.Fatalf("selected = %+v, %v", selected, ok)
	}
	if !m.Filtering() {
		t.Fatal("filter should remain active until enter or esc")
	}
	m.UpdateFilter(tea.KeyMsg{Type: tea.KeyEnter})
	if m.Filtering() {
		t.Fatal("enter should finish filtering")
	}
}

func TestEscapeRestoresPreviousFilter(t *testing.T) {
	m := New()
	m.SetServers(testServers())
	m.BeginFilter()
	m.UpdateFilter(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("web")})
	m.UpdateFilter(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Filter() != "" || m.Filtering() {
		t.Errorf("filter after escape = %q, active=%v", m.Filter(), m.Filtering())
	}
}

func TestViewportKeepsSelectedServerVisible(t *testing.T) {
	m := New()
	m.SetServers(testServers())
	m.SetHeight(3)
	m.Move(3)
	selected, ok := m.Selected()
	if !ok || selected.Name != "worker" {
		t.Fatalf("selected = %+v, %v", selected, ok)
	}
	view := m.View(80)
	if !strings.Contains(view, "worker") || strings.Contains(view, "web-1") {
		t.Fatalf("viewport did not follow cursor:\n%s", view)
	}
}

func TestSetServersPreservesSelectionByID(t *testing.T) {
	m := New()
	m.SetServers(testServers())
	m.Move(1)
	m.SetServers([]api.Server{testServers()[1], testServers()[0]})
	selected, ok := m.Selected()
	if !ok || selected.ID != 2 {
		t.Fatalf("selected = %+v, %v", selected, ok)
	}
}

func TestWideViewUsesAlignedColumnsAndOnlyServerRuntime(t *testing.T) {
	m := New()
	m.SetServers([]api.Server{
		{ID: 1, Name: "statamic-01", IPAddress: "195.201.147.248", Status: "active", SitesCount: 11, PHPVersion: 8.2},
		{ID: 2, Name: "database-production", IPAddress: "162.55.216.250", Status: "rebooting", SitesCount: 0, MySQLVersion: 8},
	})
	view := m.View(100, 12)
	for _, want := range []string{"NAME", "IP ADDRESS", "STATUS", "SITES", "RUNTIME", "PHP 8.2"} {
		if !strings.Contains(view, want) {
			t.Errorf("wide view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "MySQL") || strings.Contains(view, "PostgreSQL") {
		t.Fatalf("list inferred a database engine from the server response:\n%s", view)
	}
}

func TestCompactViewKeepsServerMetadataWithoutMonitoring(t *testing.T) {
	m := New()
	m.SetServers([]api.Server{{
		ID:         1,
		Name:       "zeroseven-corporation-production",
		IPAddress:  "128.140.3.147",
		Status:     "active",
		SitesCount: 6,
		PHPVersion: 7.4,
	}})
	view := m.View(42, 8)
	for _, want := range []string{"zeroseven", "active", "128.140.3.147", "6 sites", "PHP 7.4"} {
		if !strings.Contains(view, want) {
			t.Errorf("compact view missing %q:\n%s", want, view)
		}
	}
	for _, unwanted := range []string{"CPU", "RAM", "DISK"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("compact list should not render monitoring data %q:\n%s", unwanted, view)
		}
	}
}
