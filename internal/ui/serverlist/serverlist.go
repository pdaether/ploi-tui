package serverlist

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/ui/components"
	"github.com/pdaether/ploi-tui/internal/ui/theme"
)

type Model struct {
	servers      []api.Server
	cursor       int
	offset       int
	height       int
	width        int
	filter       string
	filterBefore string
	filtering    bool
}

func New() Model { return Model{} }

func (m *Model) SetServers(servers []api.Server) {
	selectedID := int64(0)
	if selected, ok := m.Selected(); ok {
		selectedID = selected.ID
	}
	m.servers = append([]api.Server(nil), servers...)
	if selectedID != 0 {
		for i, server := range m.visible() {
			if server.ID == selectedID {
				m.cursor = i
				m.ensureCursorVisible(len(m.visible()))
				return
			}
		}
	}
	m.clampCursor()
	m.ensureCursorVisible(len(m.visible()))
}

func (m *Model) UpdateServer(server api.Server) {
	for i := range m.servers {
		if m.servers[i].ID == server.ID {
			m.servers[i] = server
			m.ensureCursorVisible(len(m.visible()))
			return
		}
	}
}

func (m *Model) SetHeight(height int) {
	m.height = max(0, height)
	m.ensureCursorVisible(len(m.visible()))
}

func (m *Model) SetWidth(width int) {
	m.width = max(0, width)
	m.ensureCursorVisible(len(m.visible()))
}

func (m Model) All() []api.Server {
	return append([]api.Server(nil), m.servers...)
}

func (m Model) Visible() []api.Server { return append([]api.Server(nil), m.visible()...) }

func (m Model) Selected() (api.Server, bool) {
	visible := m.visible()
	if m.cursor < 0 || m.cursor >= len(visible) {
		return api.Server{}, false
	}
	return visible[m.cursor], true
}

func (m Model) Cursor() int { return m.cursor }

func (m Model) Filter() string { return m.filter }

func (m Model) Filtering() bool { return m.filtering }

func (m *Model) Move(delta int) {
	if len(m.visible()) == 0 {
		return
	}
	m.cursor += delta
	m.clampCursor()
	m.ensureCursorVisible(len(m.visible()))
}

func (m *Model) BeginFilter() {
	m.filterBefore = m.filter
	m.filtering = true
}

func (m *Model) UpdateFilter(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		m.filter = m.filterBefore
		m.filtering = false
	case "enter":
		m.filtering = false
	case "backspace", "ctrl+h":
		runes := []rune(m.filter)
		if len(runes) > 0 {
			m.filter = string(runes[:len(runes)-1])
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			m.filter += string(msg.Runes)
			if msg.Type == tea.KeySpace {
				m.filter += " "
			}
		}
	}
	m.cursor = 0
	m.offset = 0
	m.clampCursor()
}

func (m *Model) View(width int, requestedHeight ...int) string {
	width = max(1, width)
	m.width = width
	visible := m.visible()
	height := m.height
	if len(requestedHeight) > 0 {
		height = max(0, requestedHeight[0])
	}
	lines := m.header(width)
	if len(visible) == 0 {
		if len(m.servers) == 0 {
			lines = append(lines, theme.Muted.Render("No servers yet. Create one at https://ploi.io"), theme.Muted.Render("Press r to retry."))
		} else {
			lines = append(lines, theme.Muted.Render("No servers match the current filter."))
		}
		return strings.Join(lines, "\n")
	}

	offset, rowLimit := m.viewport(len(visible), height, width)
	end := len(visible)
	if rowLimit > 0 {
		end = min(end, offset+rowLimit)
	}
	for i := offset; i < end; i++ {
		lines = append(lines, renderRow(visible[i], i == m.cursor, width))
	}
	if (m.filtering || m.filter != "") && (rowLimit == 0 || len(lines)+m.rowHeight(width) <= height) {
		lines = append(lines, theme.Rule.Render(fmt.Sprintf("── %d match%s ──", len(visible), plural(len(visible)))))
	}
	return strings.Join(lines, "\n")
}

func (m Model) visible() []api.Server {
	if m.filter == "" {
		return m.servers
	}
	needle := strings.ToLower(m.filter)
	visible := make([]api.Server, 0, len(m.servers))
	for _, server := range m.servers {
		fields := []string{server.Name, server.IPAddress, server.Status, server.Type}
		for _, field := range fields {
			if strings.Contains(strings.ToLower(field), needle) {
				visible = append(visible, server)
				break
			}
		}
	}
	return visible
}

func (m *Model) clampCursor() {
	count := len(m.visible())
	if count == 0 {
		m.cursor = 0
		return
	}
	m.cursor = max(0, min(count-1, m.cursor))
}

func (m *Model) ensureCursorVisible(count int) {
	if count == 0 {
		m.offset = 0
		return
	}
	_, rowLimit := m.viewport(count, m.height, m.width)
	if rowLimit <= 0 {
		m.offset = 0
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rowLimit {
		m.offset = m.cursor - rowLimit + 1
	}
	m.offset = max(0, min(count-rowLimit, m.offset))
}

func (m Model) viewport(count, height, width int) (int, int) {
	if count == 0 || height <= 0 {
		return 0, count
	}
	headerLines := 2
	if isWide(width) {
		headerLines++
	}
	trailingLines := 0
	if m.filtering || m.filter != "" {
		trailingLines++
	}
	rowLimit := max(1, (height-headerLines-trailingLines)/m.rowHeight(width))
	rowLimit = min(count, rowLimit)
	offset := max(0, min(count-rowLimit, m.offset))
	if m.cursor < offset {
		offset = m.cursor
	}
	if m.cursor >= offset+rowLimit {
		offset = m.cursor - rowLimit + 1
	}
	return offset, rowLimit
}

func (m Model) header(width int) []string {
	left := fmt.Sprintf("SERVERS (%d)", len(m.servers))
	right := "↑↓ navigate"
	if m.filtering || m.filter != "" {
		right = "filter: " + strconv.Quote(m.filter)
		if m.filtering {
			right += "  esc cancel"
		}
	}

	lines := []string{alignHeader(left, right, width)}
	if isWide(width) {
		nameWidth := wideNameWidth(width)
		lines = append(lines, "  "+theme.Muted.Render(pad("NAME", nameWidth))+"  "+theme.Muted.Render(pad("IP ADDRESS", 15))+"  "+theme.Muted.Render(pad("STATUS", 14))+"  "+theme.Muted.Render(pad("SITES", 7))+"  "+theme.Muted.Render("RUNTIME"))
	}
	return append(lines, theme.Rule.Render(strings.Repeat("─", width)))
}

func (m Model) rowHeight(width int) int {
	if isWide(width) || width >= 54 {
		return 1
	}
	return 2
}

func renderRow(server api.Server, selected bool, width int) string {
	if isWide(width) {
		return renderWideRow(server, selected, width)
	}
	if width >= 54 {
		return renderMediumRow(server, selected, width)
	}
	return renderCompactRow(server, selected, width)
}

func renderWideRow(server api.Server, selected bool, width int) string {
	nameWidth := wideNameWidth(width)
	return prefix(selected) + formatName(server.Name, nameWidth, selected) + "  " +
		pad(valueOrDash(server.IPAddress), 15) + "  " +
		pad(components.ServerStatus(server.Status), 14) + "  " +
		pad(fmt.Sprintf("%d sites", server.SitesCount), 7) + "  " + runtime(server)
}

func renderMediumRow(server api.Server, selected bool, width int) string {
	nameWidth := max(12, width-35)
	return prefix(selected) + formatName(server.Name, nameWidth, selected) + "  " +
		pad(valueOrDash(server.IPAddress), 15) + "  " + components.ServerStatus(server.Status)
}

func renderCompactRow(server api.Server, selected bool, width int) string {
	status := components.ServerStatus(server.Status)
	nameWidth := max(8, width-lipgloss.Width(status)-4)
	first := prefix(selected) + formatName(server.Name, nameWidth, selected) + "  " + status
	meta := valueOrDash(server.IPAddress) + "  ·  " + fmt.Sprintf("%d sites", server.SitesCount)
	if value := runtime(server); value != "" {
		meta += "  ·  " + value
	}
	return first + "\n   " + theme.Muted.Render(components.Truncate(meta, max(1, width-3)))
}

func formatName(name string, width int, selected bool) string {
	name = components.Truncate(valueOrDash(name), width)
	if name == "" {
		name = "(unnamed)"
	}
	name = pad(name, width)
	if selected {
		return theme.Header.Render(name)
	}
	return name
}

func prefix(selected bool) string {
	if selected {
		return theme.Key.Render("▸ ")
	}
	return "  "
}

func runtime(server api.Server) string {
	if server.PHPVersion > 0 {
		return "PHP " + components.FormatVersion(server.PHPVersion)
	}
	return ""
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

func alignHeader(left, right string, width int) string {
	left = components.Truncate(left, width)
	right = components.Truncate(right, max(1, width-lipgloss.Width(left)-2))
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return theme.Header.Render(left) + strings.Repeat(" ", gap) + theme.Muted.Render(right)
}

func isWide(width int) bool {
	return width >= 74
}

func wideNameWidth(width int) int {
	return min(30, max(18, width-55))
}

func plural(count int) string {
	if count == 1 {
		return " match"
	}
	return " matches"
}
