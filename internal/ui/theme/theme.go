package theme

import "github.com/charmbracelet/lipgloss"

var (
	Header = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#5F00AF", Dark: "#D2A8FF"}).
		Bold(true)
	Rule = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#9A9A9A", Dark: "#6E7681"})
	Muted = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#666666", Dark: "#8B949E"})
	Key = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#005CC5", Dark: "#79C0FF"}).
		Bold(true)
	Success = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1A7F37", Dark: "#56D364"})
	Warning = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#9A6700", Dark: "#E3B341"})
	Error = lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#CF222E", Dark: "#FF7B72"})
	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#9A9A9A", Dark: "#6E7681"}).
		Padding(0, 1)
)
