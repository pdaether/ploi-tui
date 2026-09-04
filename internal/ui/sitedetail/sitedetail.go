package sitedetail

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/ui/components"
	"github.com/pdaether/ploi-tui/internal/ui/theme"
)

type Model struct {
	Server              *api.Server
	Site                *api.Site
	Loading             bool
	Err                 error
	Certificates        []api.Certificate
	CertificatesLoaded  bool
	CertificatesLoading bool
	CertificatesErr     error
}

func (m Model) View(width int) string {
	if m.Site == nil {
		return theme.Muted.Render("No site selected.")
	}
	width = max(1, width)
	if m.Loading {
		return theme.Muted.Render("⟳ Loading site...")
	}
	if m.Err != nil {
		return theme.Error.Render("API error: "+m.Err.Error()) + "\n\n" + theme.Muted.Render("Press r to try again.")
	}

	domain := valueOrDash(m.Site.Domain)
	serverID := m.Site.ServerID
	if m.Server != nil {
		serverID = m.Server.ID
	}
	lines := []string{
		alignRight(domain, components.SiteStatus(m.Site.Status), width),
		theme.Rule.Render(strings.Repeat("─", width)),
	}
	lines = append(lines, detailRows([]row{
		{"Site ID", fmt.Sprintf("%d", m.Site.ID), "Server ID", fmt.Sprintf("%d", serverID)},
		{"Domain", domain, "Type", valueOrDash(m.Site.ProjectType)},
		{"System user", valueOrDash(m.Site.SystemUser), "PHP version", components.FormatVersion(m.Site.PHPVersion)},
		{"Web dir", valueOrDash(m.Site.WebDirectory), "Disk usage", diskUsage(m.Site.DiskUsage)},
		{"Last deploy", relativeTime(m.Site.LastDeployAt), "Repository", repositoryStatus(m.Site.HasRepository)},
		{"Project root", valueOrDash(m.Site.ProjectRoot), "", ""},
	}, width)...)
	lines = append(lines, "", theme.Header.Render("CERTIFICATES"))
	lines = append(lines, m.certificatesView(width)...)
	lines = append(lines, "", theme.Header.Render("ACTIONS"),
		theme.Key.Render("s")+"       SSH into site",
		theme.Key.Render("o")+"       Open in Ploi",
		theme.Key.Render("b")+"       Open domain",
	)
	return strings.Join(lines, "\n")
}

func (m Model) certificatesView(width int) []string {
	switch {
	case m.CertificatesLoading:
		return []string{theme.Muted.Render("⟳ Loading certificates...")}
	case m.CertificatesErr != nil:
		return []string{theme.Error.Render("Unable to load certificates: " + m.CertificatesErr.Error())}
	case !m.CertificatesLoaded:
		return []string{theme.Muted.Render("Certificates have not been loaded yet.")}
	case len(m.Certificates) == 0:
		return []string{theme.Muted.Render("No certificates configured for this site.")}
	}

	lines := make([]string, 0, len(m.Certificates))
	for _, certificate := range m.Certificates {
		lines = append(lines, certificateRow(certificate, width))
	}
	return lines
}

func certificateRow(certificate api.Certificate, width int) string {
	typeName := certificateType(certificate.Type)
	domain := components.Truncate(valueOrDash(certificate.Domain), max(8, width-39))
	expiry := certificateExpiry(certificate.ExpiresAt, time.Now().UTC())
	status := components.CertificateStatus(certificate.Status)
	return fmt.Sprintf("  %s  %s  %s  %s", pad(typeName, 14), pad(domain, max(8, width-39)), expiry, status)
}

func certificateExpiry(expiresAt *api.Time, now time.Time) string {
	if expiresAt == nil || expiresAt.IsZero() {
		return theme.Muted.Render("—")
	}
	days := int(math.Ceil(expiresAt.Sub(now).Hours() / 24))
	if days <= 0 {
		return components.SeverityStyle(api.SeverityBad).Render("expired")
	}
	value := fmt.Sprintf("%dd", days)
	return components.SeverityStyle(api.CertExpirySeverity(expiresAt, now)).Render(value)
}

func certificateType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "letsencrypt", "let's encrypt":
		return "Let's Encrypt"
	case "custom":
		return "Custom"
	default:
		return valueOrDash(value)
	}
}

type row struct {
	leftLabel  string
	leftValue  string
	rightLabel string
	rightValue string
}

func detailRows(rows []row, width int) []string {
	if width < 60 {
		lines := make([]string, 0, len(rows)*2)
		for _, row := range rows {
			lines = append(lines,
				fmt.Sprintf("  %-15s %s", row.leftLabel, components.Truncate(row.leftValue, max(1, width-19))),
				fmt.Sprintf("  %-15s %s", row.rightLabel, components.Truncate(row.rightValue, max(1, width-19))),
			)
		}
		return lines
	}
	leftValueWidth := 18
	for _, row := range rows {
		leftValueWidth = max(leftValueWidth, lipgloss.Width(row.leftValue))
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		rightWidth := max(1, width-2-15-leftValueWidth-2-15-1)
		lines = append(lines, fmt.Sprintf("  %-15s %-*s  %-15s %s", row.leftLabel, leftValueWidth, components.Truncate(row.leftValue, leftValueWidth), row.rightLabel, components.Truncate(row.rightValue, rightWidth)))
	}
	return lines
}

func alignRight(left, right string, width int) string {
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return theme.Header.Render(left) + strings.Repeat(" ", gap) + right
}

func diskUsage(usage *api.DiskUsage) string {
	if usage == nil || usage.Human == "" {
		return "—"
	}
	return usage.Human
}

func relativeTime(value *api.Time) string {
	if value == nil || value.IsZero() {
		return "—"
	}
	delta := time.Since(value.Time).Round(time.Minute)
	if delta < time.Minute {
		return "just now"
	}
	if delta < time.Hour {
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	}
	if delta < 48*time.Hour {
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	}
	return value.Format("2006-01-02")
}

func repositoryStatus(hasRepository bool) string {
	if hasRepository {
		return "configured"
	}
	return "—"
}

func valueOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func pad(value string, width int) string {
	return value + strings.Repeat(" ", max(0, width-lipgloss.Width(value)))
}
