package components

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pdaether/ploi-tui/internal/api"
	"github.com/pdaether/ploi-tui/internal/ui/theme"
)

var statusSymbols = map[api.Severity]string{
	api.SeverityNeutral: "●",
	api.SeverityOK:      "●",
	api.SeverityBusy:    "◐",
	api.SeverityBad:     "✕",
}

func ServerStatus(raw string) string {
	return status(raw, api.ServerStatusSeverity(raw))
}

func SiteStatus(raw string) string {
	return status(raw, api.SiteStatusSeverity(raw))
}

func CertificateStatus(raw string) string {
	return status(raw, api.CertificateStatusSeverity(raw))
}

func status(raw string, severity api.Severity) string {
	status := api.NormalizeStatus(raw)
	if status == "" {
		status = "unknown"
	}
	symbol := statusSymbols[severity]
	if severity == api.SeverityNeutral {
		return theme.Muted.Render(symbol) + " " + theme.Muted.Render(status)
	}
	return severityStyle(severity).Render(symbol) + " " + severityStyle(severity).Render(status)
}

func SeverityStyle(severity api.Severity) lipgloss.Style {
	return severityStyle(severity)
}

func severityStyle(severity api.Severity) lipgloss.Style {
	switch severity {
	case api.SeverityOK:
		return theme.Success
	case api.SeverityBusy:
		return theme.Warning
	case api.SeverityBad:
		return theme.Error
	default:
		return theme.Muted
	}
}

func FormatVersion(version api.FlexNum) string {
	if version <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f", float64(version))
}

func FormatPercent(value api.FlexNum) string {
	return fmt.Sprintf("%.1f%%", float64(value))
}

func FormatLoad(value api.FlexNum) string {
	return strconv.FormatFloat(float64(value), 'f', 2, 64)
}

func Truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	runes := []rune(value)
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:min(width-1, len(runes))]) + "…"
}

func Sparkline(values []float64, width int, minValue, maxValue float64) string {
	blocks := []rune("▁▂▃▄▅▆▇█")

	if width <= 0 {
		return ""
	}
	if len(values) == 0 {
		return strings.Repeat(" ", width)
	}
	if maxValue <= minValue {
		minValue, maxValue = values[0], values[0]
		for _, value := range values[1:] {
			minValue = min(minValue, value)
			maxValue = max(maxValue, value)
		}
		if maxValue <= minValue {
			maxValue = minValue + 1
		}
	}

	var b strings.Builder
	b.Grow(width)
	for i := 0; i < width; i++ {
		index := 0
		if width > 1 {
			index = i * (len(values) - 1) / (width - 1)
		}
		value := values[index]
		if math.IsNaN(value) || math.IsInf(value, 0) {
			value = minValue
		}
		level := int(math.Round((value - minValue) / (maxValue - minValue) * float64(len(blocks)-1)))
		level = max(0, min(len(blocks)-1, level))
		b.WriteRune(blocks[level])
	}
	return b.String()
}

// BarChart renders a compact, vertically scaled chart with one bar per sample.
// Each string is one row, ordered from the highest to lowest values.
func BarChart(values []float64, width, height int, minValue, maxValue float64) []string {
	if width <= 0 || height <= 0 {
		return nil
	}
	if maxValue <= minValue {
		minValue, maxValue = chartRange(values, minValue, maxValue)
	}

	rows := make([][]rune, height)
	for row := range rows {
		rows[row] = make([]rune, width)
		for column := range rows[row] {
			rows[row][column] = ' '
		}
	}
	if len(values) == 0 {
		return chartRows(rows)
	}

	for column := 0; column < width; column++ {
		index := 0
		if width > 1 {
			index = column * (len(values) - 1) / (width - 1)
		}
		value := values[index]
		if math.IsNaN(value) || math.IsInf(value, 0) {
			value = minValue
		}
		level := (value - minValue) / (maxValue - minValue) * float64(height)
		level = max(0, min(float64(height), level))
		for row := 0; row < height; row++ {
			fill := level - float64(height-row-1)
			if fill >= 1 {
				rows[row][column] = '█'
			} else if fill > 0 {
				rows[row][column] = partialBlock(fill)
			}
		}
	}
	return chartRows(rows)
}

func chartRange(values []float64, minValue, maxValue float64) (float64, float64) {
	if len(values) > 0 {
		minValue, maxValue = values[0], values[0]
		for _, value := range values[1:] {
			minValue = min(minValue, value)
			maxValue = max(maxValue, value)
		}
	}
	if maxValue <= minValue {
		maxValue = minValue + 1
	}
	return minValue, maxValue
}

func partialBlock(fill float64) rune {
	blocks := []rune("▂▃▄▅▆▇")
	index := int(math.Ceil(fill*float64(len(blocks)))) - 1
	return blocks[max(0, min(len(blocks)-1, index))]
}

func chartRows(rows [][]rune) []string {
	result := make([]string, len(rows))
	for row := range rows {
		result[row] = string(rows[row])
	}
	return result
}

func Values[T any](items []T, value func(T) float64) []float64 {
	result := make([]float64, len(items))
	for i, item := range items {
		result[i] = value(item)
	}
	return result
}
