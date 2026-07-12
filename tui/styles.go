package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#3B82F6")).Padding(0, 2)
	labelStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9CA3AF"))
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	keyStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5565"))
	nameStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#E5E7EB"))
	selModelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#60A5FA"))
	checkStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))
	warnStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	unavailStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))

	errTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFF")).Background(lipgloss.Color("#EF4444")).Padding(0, 2)

	barLine = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(lipgloss.Color("#3B82F6")).
		PaddingLeft(1).
		Render

	padLine = lipgloss.NewStyle().PaddingLeft(2).Render
)

func checkBox(on bool) string {
	if on {
		return checkStyle.Render("■") + " "
	}
	return inactiveStyle.Render("□") + " "
}

func focusLine(text string, focused bool) string {
	if focused {
		return barLine(text)
	}
	return padLine(text)
}

var spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#3B82F6"))

func wrapCursor(cursor *int, total, dir int) {
	if total <= 0 {
		*cursor = 0
		return
	}
	*cursor = (*cursor + dir + total) % total
}

func maskKey(key string) string {
	if len(key) > 8 {
		return key[:4] + "****" + key[len(key)-4:]
	} else if key != "" {
		return "****"
	}
	return "(空)"
}
