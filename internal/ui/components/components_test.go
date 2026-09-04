package components

import (
	"strings"
	"testing"

	"github.com/pdaether/ploi-tui/internal/api"
)

func TestServerStatusAndFormatting(t *testing.T) {
	if got := ServerStatus("Server active"); !strings.Contains(got, "active") {
		t.Fatalf("ServerStatus = %q", got)
	}
	if got := ServerStatus("deploy-failed"); !strings.Contains(got, "deploy-failed") {
		t.Fatalf("ServerStatus = %q", got)
	}
	if got := FormatVersion(0); got != "—" {
		t.Errorf("FormatVersion(0) = %q", got)
	}
	if got := FormatVersion(8.3); got != "8.3" {
		t.Errorf("FormatVersion(8.3) = %q", got)
	}
}

func TestSparkline(t *testing.T) {
	if got := Sparkline([]float64{0, 50, 100}, 5, 0, 100); got != "▁▁▅▅█" {
		t.Errorf("Sparkline = %q", got)
	}
	if got := Sparkline(nil, 4, 0, 100); got != "    " {
		t.Errorf("empty Sparkline = %q", got)
	}
	if got := Sparkline([]float64{1}, 3, 0, 1); got != "███" {
		t.Errorf("single-value Sparkline = %q", got)
	}
}

func TestBarChart(t *testing.T) {
	got := BarChart([]float64{0, 50, 100}, 3, 3, 0, 100)
	want := []string{"  █", " ▄█", " ██"}
	if len(got) != len(want) {
		t.Fatalf("BarChart rows = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("BarChart row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("server-name", 8); got != "server-…" {
		t.Errorf("Truncate = %q", got)
	}
	if got := Truncate("server", 0); got != "" {
		t.Errorf("zero-width Truncate = %q", got)
	}
}

func TestValues(t *testing.T) {
	items := []api.MonitoringSample{{CPU: 1}, {CPU: 2.5}}
	got := Values(items, func(sample api.MonitoringSample) float64 { return float64(sample.CPU) })
	if len(got) != 2 || got[0] != 1 || got[1] != 2.5 {
		t.Errorf("Values = %v", got)
	}
}
