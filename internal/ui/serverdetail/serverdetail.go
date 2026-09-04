package serverdetail

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/ui/components"
	"github.com/pdaether/ploi-tui/internal/ui/theme"
)

type Tab int

const (
	OverviewTab Tab = iota
	MonitoringTab
	SitesTab
)

type Model struct {
	Server            *api.Server
	Tab               Tab
	Loading           bool
	Err               error
	Databases         []api.Database
	DatabasesLoaded   bool
	DatabasesLoading  bool
	DatabasesErr      error
	Samples           []api.MonitoringSample
	MonitoringLoaded  bool
	MonitoringLoading bool
	MonitoringErr     error
	Sites             []api.Site
	SitesLoaded       bool
	SitesLoading      bool
	SitesErr          error
	SitesCursor       int
}

func (m Model) TabBar() string {
	labels := []string{"Overview", "Monitoring", "Sites"}
	parts := make([]string, len(labels))
	for i, label := range labels {
		if Tab(i) == m.Tab {
			parts[i] = theme.Header.Render("*" + label + "*")
		} else {
			parts[i] = theme.Muted.Render(label)
		}
	}
	return strings.Join(parts, " │ ")
}

func (m Model) View(width int, requestedHeight ...int) string {
	if m.Server == nil {
		return theme.Muted.Render("No server selected.")
	}
	width = max(1, width)
	if m.Loading {
		return theme.Muted.Render("⟳ Loading server...")
	}
	if m.Err != nil {
		return theme.Error.Render("API error: "+m.Err.Error()) + "\n\n" + theme.Muted.Render("Press r to try again.")
	}
	height := 0
	if len(requestedHeight) > 0 {
		height = requestedHeight[0]
	}
	switch m.Tab {
	case MonitoringTab:
		return m.monitoringView(width, height)
	case SitesTab:
		return m.sitesView(width)
	default:
		return m.overviewView(width)
	}
}

func (m Model) overviewView(width int) string {
	server := m.Server
	name := server.Name
	if name == "" {
		name = fmt.Sprintf("Server %d", server.ID)
	}
	status := components.ServerStatus(server.Status)
	lines := []string{
		alignRight(name, status, width),
		theme.Rule.Render(strings.Repeat("─", width)),
	}
	lines = append(lines, detailColumns([]detailRow{
		{"Server ID", fmt.Sprintf("%d", server.ID), "IP address", valueOrDash(server.IPAddress)},
		{"Created", formatDate(server.CreatedAt), "PHP version", components.FormatVersion(server.PHPVersion)},
		{"Sites", fmt.Sprintf("%d", server.SitesCount), "Managed DBs", m.databaseSummary()},
		{"Monitoring", monitoringStatus(server.Monitoring), "", ""},
	}, width)...)
	lines = append(lines, "", theme.Header.Render("QUICK STATS"))
	if len(m.Samples) > 0 {
		samples := chronological(m.Samples)
		cpu := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.CPU) })
		ram := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.RAM) })
		disk := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.Disk) })
		lines = append(lines,
			metricSummary("CPU", cpu, float64(samples[len(samples)-1].CPU), width),
			metricSummary("RAM", ram, float64(samples[len(samples)-1].RAM), width),
			metricSummary("DISK", disk, float64(samples[len(samples)-1].Disk), width),
		)
	} else if m.MonitoringLoading {
		lines = append(lines, theme.Muted.Render("⟳ Loading resource stats…"))
	} else if m.MonitoringErr != nil {
		lines = append(lines, theme.Muted.Render("Resource stats unavailable. Open Monitoring for details."))
	} else {
		lines = append(lines, theme.Muted.Render("Open Monitoring to load live resource stats."))
	}
	lines = append(lines, "", theme.Header.Render("ACTIONS"),
		theme.Key.Render("s")+"       SSH into server",
		theme.Key.Render("o")+"       Open in Ploi",
		theme.Key.Render("ctrl+r")+"  Restart server",
		theme.Key.Render("c")+"       Copy IP address",
	)
	return strings.Join(lines, "\n")
}

func (m Model) monitoringView(width, height int) string {
	if !m.Server.Monitoring {
		return monitoringUnavailable()
	}
	if m.MonitoringLoading {
		return theme.Muted.Render("⟳ Loading monitoring…")
	}
	if m.MonitoringErr != nil {
		if monitoringUnavailableError(m.MonitoringErr) {
			return monitoringUnavailable()
		}
		return theme.Error.Render("API error: "+m.MonitoringErr.Error()) + "\n\n" + theme.Muted.Render("Press r to try again.")
	}
	if !m.MonitoringLoaded {
		return theme.Muted.Render("Monitoring has not been loaded yet.")
	}

	lines := []string{
		theme.Header.Render("MONITORING — last 24h") + "  " + theme.Muted.Render("refreshes every 60s"),
		theme.Rule.Render(strings.Repeat("─", width)),
	}
	if len(m.Samples) == 0 {
		lines = append(lines, theme.Muted.Render("No monitoring samples returned."))
		return strings.Join(lines, "\n")
	}

	samples := chronological(m.Samples)
	cpu := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.CPU) })
	ram := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.RAM) })
	load := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.LoadAverage) })
	disk := components.Values(samples, func(sample api.MonitoringSample) float64 { return float64(sample.Disk) })
	last := samples[len(samples)-1]
	lines = append(lines,
		metricChart("CPU %", cpu, components.FormatPercent(last.CPU), 0, 100, width, height),
		metricChart("MEMORY %", ram, components.FormatPercent(last.RAM), 0, 100, width, height),
		metricChart("LOAD AVG", load, components.FormatLoad(last.LoadAverage), 0, maxSeries(load), width, height),
		metricChart("DISK %", disk, components.FormatPercent(last.Disk), 0, 100, width, height),
	)
	return strings.Join(lines, "\n")
}

func (m Model) sitesView(width int) string {
	lines := []string{
		alignRight(fmt.Sprintf("SITES (%d)", len(m.Sites)), "↑↓ navigate", width),
		theme.Rule.Render(strings.Repeat("─", width)),
	}
	switch {
	case m.SitesLoading:
		lines = append(lines, theme.Muted.Render("⟳ Loading sites..."))
	case m.SitesErr != nil:
		lines = append(lines, theme.Error.Render("API error: "+m.SitesErr.Error()), theme.Muted.Render("Press r to try again."))
	case !m.SitesLoaded:
		lines = append(lines, theme.Muted.Render("Sites have not been loaded yet."))
	case len(m.Sites) == 0:
		lines = append(lines, theme.Muted.Render("No sites on this server."))
	default:
		for i, site := range m.Sites {
			lines = append(lines, siteRow(site, i == m.SitesCursor, width))
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) SelectedSite() (api.Site, bool) {
	if m.SitesCursor < 0 || m.SitesCursor >= len(m.Sites) {
		return api.Site{}, false
	}
	return m.Sites[m.SitesCursor], true
}

func (m *Model) MoveSite(delta int) {
	if len(m.Sites) == 0 {
		return
	}
	m.SitesCursor = max(0, min(len(m.Sites)-1, m.SitesCursor+delta))
}

func (m *Model) SetSites(sites []api.Site) {
	selectedID := int64(0)
	if selected, ok := m.SelectedSite(); ok {
		selectedID = selected.ID
	}
	m.Sites = append([]api.Site(nil), sites...)
	m.SitesCursor = 0
	for i, site := range m.Sites {
		if site.ID == selectedID {
			m.SitesCursor = i
			break
		}
	}
}

func siteRow(site api.Site, selected bool, width int) string {
	prefix := "  "
	if selected {
		prefix = theme.Key.Render("▸ ")
	}
	domainWidth := max(12, width-43)
	domain := components.Truncate(valueOrDash(site.Domain), domainWidth)
	if selected {
		domain = theme.Header.Render(domain)
	}
	project := components.Truncate(valueOrDash(site.ProjectType), 12)
	disk := "—"
	if site.DiskUsage != nil && site.DiskUsage.Human != "" {
		disk = site.DiskUsage.Human
	}
	if width < 60 {
		return prefix + domain + "  " + components.SiteStatus(site.Status) + "\n   " + theme.Muted.Render(components.Truncate(project+"  ·  PHP "+components.FormatVersion(site.PHPVersion)+"  ·  "+disk, max(1, width-3)))
	}
	return prefix + padSite(domain, domainWidth) + "  " + padSite(components.SiteStatus(site.Status), 15) + "  " + padSite(project, 12) + "  " + padSite("PHP "+components.FormatVersion(site.PHPVersion), 8) + "  " + disk
}

func padSite(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}

func monitoringUnavailable() string {
	return theme.Warning.Render("⊘ MONITORING NOT AVAILABLE") + "\n\n" +
		theme.Muted.Render("Install the monitoring agent on this server, or your plan does not include monitoring.") + "\n" +
		theme.Key.Render("→") + " " + theme.Muted.Render("https://ploi.io/pricing")
}

func monitoringUnavailableError(err error) bool {
	var apiErr *api.APIError
	return errors.As(err, &apiErr) && apiErr.MonitoringUnavailable()
}

type detailRow struct {
	leftLabel  string
	leftValue  string
	rightLabel string
	rightValue string
}

func detailColumns(rows []detailRow, width int) []string {
	leftLabelWidth := 15
	leftValueWidth := 18
	for _, row := range rows {
		leftValueWidth = max(leftValueWidth, lipgloss.Width(row.leftValue))
	}
	if width < 60 {
		lines := make([]string, 0, len(rows)*2)
		for _, row := range rows {
			lines = append(lines, fmt.Sprintf("  %-15s %s", row.leftLabel, fitValue(row.leftValue, width-19)))
			if row.rightLabel != "" {
				lines = append(lines, fmt.Sprintf("  %-15s %s", row.rightLabel, fitValue(row.rightValue, width-19)))
			}
		}
		return lines
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.rightLabel == "" {
			lines = append(lines, fmt.Sprintf("  %-*s %s", leftLabelWidth, row.leftLabel, fitValue(row.leftValue, max(1, width-2-leftLabelWidth-1))))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %-*s %-*s  %-15s %s", leftLabelWidth, row.leftLabel, leftValueWidth, fitValue(row.leftValue, leftValueWidth), row.rightLabel, fitValue(row.rightValue, max(1, width-2-leftLabelWidth-leftValueWidth-2-15-1))))
	}
	return lines
}

func metricSummary(label string, values []float64, current float64, width int) string {
	sparkWidth := max(8, min(18, width-30))
	return fmt.Sprintf("  %-5s %s  %s", label, components.Sparkline(values, sparkWidth, 0, 100), theme.Muted.Render(fmt.Sprintf("%.1f%%", current)))
}

func metricChart(label string, values []float64, current string, minValue, maxValue float64, width, height int) string {
	if height >= 18 && width >= 48 {
		chartWidth := max(12, width-9)
		chart := components.BarChart(values, chartWidth, 3, minValue, maxValue)
		middle := minValue + (maxValue-minValue)/2
		return strings.Join([]string{
			fmt.Sprintf("  %-9s now %s", label, current),
			fmt.Sprintf("  %3s │ %s", chartScale(maxValue), chart[0]),
			fmt.Sprintf("  %3s │ %s", chartScale(middle), chart[1]),
			fmt.Sprintf("  %3s │ %s", chartScale(minValue), chart[2]),
		}, "\n")
	}
	if width < 48 {
		sparkWidth := max(4, width-14)
		return fmt.Sprintf("  %-9s %s  now %s", label, components.Sparkline(values, sparkWidth, minValue, maxValue), current)
	}
	sparkWidth := max(12, width-28)
	spark := components.Sparkline(values, sparkWidth, minValue, maxValue)
	return fmt.Sprintf("  %-9s %s  now %s", label, spark, current)
}

func chartScale(value float64) string {
	if value >= 10 || value == 0 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}

func chronological(samples []api.MonitoringSample) []api.MonitoringSample {
	ordered := append([]api.MonitoringSample(nil), samples...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Date.IsZero() || ordered[j].Date.IsZero() {
			return false
		}
		return ordered[i].Date.Before(ordered[j].Date.Time)
	})
	return ordered
}

func maxSeries(values []float64) float64 {
	maximum := 1.0
	for _, value := range values {
		maximum = max(maximum, value)
	}
	return maximum
}

func alignRight(left, right string, width int) string {
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return theme.Header.Render(left) + strings.Repeat(" ", gap) + right
}

func valueOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func formatDate(value api.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Format("2006-01-02")
}

func monitoringStatus(installed bool) string {
	if installed {
		return theme.Success.Render("● installed")
	}
	return theme.Muted.Render("⊘ unavailable")
}

func (m Model) databaseSummary() string {
	if m.DatabasesLoading {
		return theme.Muted.Render("loading…")
	}
	if m.DatabasesErr != nil {
		return theme.Muted.Render("unavailable")
	}
	if !m.DatabasesLoaded {
		return theme.Muted.Render("not loaded")
	}
	if len(m.Databases) == 0 {
		return theme.Muted.Render("none")
	}

	counts := map[string]int{}
	for _, database := range m.Databases {
		name := databaseType(database.Type)
		counts[name]++
	}
	parts := make([]string, 0, len(counts))
	for _, name := range []string{"PostgreSQL", "MySQL", "MariaDB", "Other"} {
		if count := counts[name]; count > 0 {
			parts = append(parts, fmt.Sprintf("%s (%d)", name, count))
		}
	}
	return strings.Join(parts, ", ")
}

func databaseType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "postgres", "postgresql":
		return "PostgreSQL"
	case "mysql":
		return "MySQL"
	case "mariadb", "maria":
		return "MariaDB"
	default:
		return "Other"
	}
}

func fitValue(value string, width int) string {
	return components.Truncate(valueOrDash(value), max(1, width))
}
