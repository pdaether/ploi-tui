package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pdaether/ploi-tui/internal/ui/components"
	"github.com/pdaether/ploi-tui/internal/ui/theme"
)

type toastClearedMsg struct{ id int }

func (a App) View() string {
	if a.width <= 0 || a.height <= 0 {
		return ""
	}

	contentWidth := max(1, a.width-2)
	header := a.header(contentWidth)
	footer := a.statusBar()
	toastView := ""
	toastHeight := 0
	if a.toast != nil {
		toastView = lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Right).Render(theme.Panel.Render(theme.Success.Render("✓ ") + a.toast.message))
		toastHeight = lipgloss.Height(toastView)
	}
	bodyHeight := max(0, a.height-4-toastHeight)
	body := a.body(contentWidth, bodyHeight)
	if a.restartConfirm {
		body = a.restartConfirmation(contentWidth)
	} else if a.sshSelecting {
		body = a.sshUserSelection(contentWidth)
	}

	bodyLines := strings.Split(body, "\n")
	availableBodyLines := bodyHeight
	if len(bodyLines) > availableBodyLines {
		bodyLines = bodyLines[:availableBodyLines]
		if len(bodyLines) > 0 {
			bodyLines[len(bodyLines)-1] = theme.Muted.Render("…")
		}
	}
	lines := []string{header, ""}
	lines = append(lines, bodyLines...)
	for len(lines) < max(0, a.height-2-toastHeight) {
		lines = append(lines, "")
	}
	lines = append(lines, theme.Rule.Render(strings.Repeat("─", contentWidth)), footer)
	if toastView != "" {
		lines = append(lines, strings.Split(toastView, "\n")...)
	}
	if len(lines) > a.height {
		lines = lines[:a.height]
	}

	return strings.Join(lines, "\n")
}

func (a App) header(width int) string {
	identity := "connect"
	if a.user != nil && a.user.Email != "" {
		identity = a.user.Email
	}
	title := "ploi-tui"
	if a.screen == siteDetailScreen && a.siteDetail.Site != nil {
		title += " / " + a.siteDetail.Site.Domain
	} else if a.screen == serverDetailScreen && a.detail.Server != nil {
		name := a.detail.Server.Name
		if name == "" {
			name = fmt.Sprintf("server-%d", a.detail.Server.ID)
		}
		title += "  " + name
	}
	quota := a.client.Quota()
	quotaText := "quota --/--"
	if quota.Limit > 0 {
		quotaText = fmt.Sprintf("quota %d/%d", quota.Remaining, quota.Limit)
	}
	if quota.CoolingDown {
		quotaText = fmt.Sprintf("cooling down %s", quota.CooldownFor.Round(time.Second))
	}

	rightText := components.Truncate(quotaText, width)
	right := theme.Muted.Render(rightText)
	leftWidth := max(1, width-lipgloss.Width(right)-1)
	if lipgloss.Width(title)+2+lipgloss.Width(identity) > leftWidth {
		if lipgloss.Width(title) >= leftWidth {
			title = components.Truncate(title, leftWidth)
			identity = ""
		} else {
			identity = components.Truncate(identity, max(1, leftWidth-lipgloss.Width(title)-2))
		}
	}
	left := theme.Header.Render(title)
	if identity != "" {
		left += "  " + theme.Muted.Render(identity)
	}
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return left + strings.Repeat(" ", gap) + right
}

func (a App) body(width int, requestedHeight ...int) string {
	height := max(0, a.height-4)
	if len(requestedHeight) > 0 {
		height = max(0, requestedHeight[0])
	}
	var body string
	if a.help {
		body = a.helpView(width)
	} else if a.screen == siteDetailScreen {
		body = a.siteDetail.View(width)
	} else if a.screen == serverDetailScreen {
		body = a.detail.View(width, height)
	} else if a.err != nil {
		body = theme.Error.Render("API error: "+a.err.Error()) + "\n\n" + theme.Muted.Render("Press r to try again.")
	} else if a.user == nil {
		body = theme.Muted.Render("Loading account...")
	} else if !a.serversLoaded {
		body = a.welcomeView()
	} else if a.serverErr != nil {
		body = theme.Error.Render("API error: "+a.serverErr.Error()) + "\n\n" + theme.Muted.Render("Press r to try again.")
	} else {
		a.serverList.SetHeight(height)
		body = a.serverList.View(width, height)
	}
	if a.sshError != "" {
		body = theme.Error.Render("SSH error: "+a.sshError) + "\n\n" + body
	}
	return body
}

func (a App) welcomeView() string {
	name := a.user.Name
	if name == "" {
		name = a.user.Email
	}
	loading := "Loading servers..."
	if !a.serversLoading {
		loading = "Press r to load servers."
	}
	return theme.Header.Render("Welcome, "+name) + "\n\n" + theme.Muted.Render(loading)
}

func (a App) helpView(width int) string {
	left := []string{
		theme.Header.Render("? HELP"),
		"",
		theme.Key.Render("j/k ↑↓") + "  navigate",
		theme.Key.Render("?") + "       toggle help",
		theme.Key.Render("q") + "       quit",
		theme.Key.Render("r") + "       refresh view",
		theme.Key.Render("R") + "       refresh all",
	}
	right := []string{
		theme.Header.Render("GLOBAL"),
		"",
		theme.Key.Render("enter") + "   select / drill down",
		theme.Key.Render("esc") + "     back / close overlay",
		theme.Key.Render("h/l ←→") + " switch tabs",
		theme.Key.Render("1-9") + "     switch tabs",
		theme.Key.Render("/") + "       filter server list",
	}
	if a.screen == siteDetailScreen {
		return ""
	}
	if a.screen == serverDetailScreen {
		right = append(right, "", theme.Header.Render("SERVER DETAIL"), theme.Key.Render("2")+"       monitoring", theme.Key.Render("s")+"       SSH into server", theme.Key.Render("o")+"       open in Ploi", theme.Key.Render("c")+"       copy IP address", theme.Key.Render("ctrl+r")+"  restart server")
	} else {
		right = append(right, "", theme.Header.Render("SERVER LIST"), theme.Key.Render("enter")+"   open server")
	}
	return joinColumns(left, right, width)
}

func (a App) statusBar() string {
	if a.help {
		return theme.Muted.Render("esc close help  ·  q quit")
	}
	if a.restartConfirm {
		return theme.Warning.Render("Restart server?") + "  " + theme.Key.Render("y") + " restart  ·  " + theme.Key.Render("n/esc") + " cancel"
	}
	if a.sshSelecting {
		return theme.Muted.Render("j/k navigate · enter connect · esc cancel")
	}
	if a.screen == serverDetailScreen {
		return a.detail.TabBar()
	}
	if a.screen == siteDetailScreen {
		return ""
	}
	if a.serverList.Filtering() {
		return theme.Muted.Render("type to filter · enter apply · esc cancel")
	}
	return theme.Key.Render("enter") + " open  ·  " + theme.Key.Render("j/k") + " navigate  ·  " + theme.Key.Render("c") + " copy IP  ·  " + theme.Key.Render("/") + " filter  ·  " + theme.Key.Render("r") + " refresh  ·  " + theme.Key.Render("? help  ·  q quit")
}

func (a App) sshUserSelection(width int) string {
	lines := []string{
		theme.Header.Render("SSH USERS"),
		theme.Rule.Render(strings.Repeat("─", width)),
	}
	switch {
	case a.sshLoading:
		lines = append(lines, theme.Muted.Render("⟳ Loading system users..."))
	case a.sshErr != nil:
		lines = append(lines, theme.Error.Render("Unable to load system users: "+a.sshErr.Error()), theme.Muted.Render("Press esc to cancel."))
	case len(a.sshUsers) == 0:
		lines = append(lines, theme.Muted.Render("No SSH users found."))
	default:
		for i, user := range a.sshUsers {
			prefix := "  "
			name := user.Name
			if i == a.sshCursor {
				prefix = theme.Key.Render("▸ ")
				name = theme.Header.Render(name)
			}
			root := user.Root
			if root == "" {
				root = "—"
			}
			lines = append(lines, prefix+name+"  "+theme.Muted.Render(root))
		}
	}
	return strings.Join(lines, "\n")
}

func (a App) restartConfirmation(width int) string {
	name := "this server"
	if a.detail.Server != nil && a.detail.Server.Name != "" {
		name = a.detail.Server.Name
	}
	lines := []string{
		theme.Warning.Render("Restart server?"),
		theme.Rule.Render(strings.Repeat("─", width)),
		fmt.Sprintf("%s will be rebooted.", name),
		theme.Muted.Render("Expect approximately 30-60 seconds of downtime."),
		"",
		theme.Key.Render("y") + " restart    " + theme.Key.Render("n / esc") + " cancel",
	}
	return strings.Join(lines, "\n")
}

func (a *App) newToast(message string) *toast {
	a.nextID++
	return &toast{id: a.nextID, message: message}
}

func clearToast(id int) tea.Cmd {
	return clearToastAfter(id, 3*time.Second)
}

func clearToastAfter(id int, duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg { return toastClearedMsg{id: id} })
}

func joinColumns(left, right []string, width int) string {
	if width < 50 {
		return strings.Join(append(append([]string(nil), left...), right...), "\n")
	}
	leftWidth := max(1, width/2-2)
	rows := max(len(left), len(right))
	lines := make([]string, 0, rows)
	for i := range rows {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		lines = append(lines, lipgloss.NewStyle().Width(leftWidth).Render(l)+r)
	}
	return strings.Join(lines, "\n")
}
