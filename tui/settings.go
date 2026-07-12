package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type settingsRow struct {
	label string
	value string
}

var retryModeLabels = []string{"新上下文", "原上下文纠错", "阈值创建新上下文"}
var retryModeValues = []string{"new_context", "correct", "threshold"}

func retryModeIndex(val string) int {
	for i, v := range retryModeValues {
		if v == val {
			return i
		}
	}
	return 1
}

func (m *Model) settingsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 设置 "))
	b.WriteString("\n\n")

	rows := m.settingsRows()
	for i, row := range rows {
		label := row.label
		val := row.value

		available := m.width - 6
		if available < 20 {
			available = 20
		}
		labelW := lipgloss.Width(label)
		gap := available - labelW - lipgloss.Width(val)
		if gap < 2 {
			gap = 2
		}

		line := fmt.Sprintf("  %s%s%s", label, strings.Repeat(" ", gap), val)
		b.WriteString(focusLine(line, i == m.settingsCursor) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(padLine(helpStyle.Render("← →: 调整  •  Enter: 保存并返回  •  Esc: 取消")))
	return b.String()
}

func (m *Model) settingsRows() []settingsRow {
	modeIdx := retryModeIndex(m.cfg.RetryMode)
	modeLabel := retryModeLabels[modeIdx]
	modeDisplay := fmt.Sprintf("◀ %s ▶", modeLabel)

	rows := []settingsRow{
		{label: "分块长度", value: fmt.Sprintf("◀ %d ▶", m.cfg.MaxChunkKeys)},
		{label: "失败重试次数", value: fmt.Sprintf("◀ %d ▶", m.cfg.MaxRetries)},
		{label: "重试策略", value: modeDisplay},
	}

	if m.cfg.RetryMode == "threshold" {
		rows = append(rows, settingsRow{
			label: "重试切换阈值",
			value: fmt.Sprintf("◀ %d ▶", m.cfg.RetryThreshold),
		})
	}

	return rows
}

func (m *Model) handleSettingsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rows := m.settingsRows()
	total := len(rows)

	switch msg.String() {
	case "up", "k":
		if total > 0 {
			m.settingsCursor = (m.settingsCursor - 1 + total) % total
		}
	case "down", "j":
		if total > 0 {
			m.settingsCursor = (m.settingsCursor + 1) % total
		}
	case "left", "h":
		m.adjustSetting(-1)
	case "right", "l":
		m.adjustSetting(1)
	case "enter":
		if err := m.cfg.Save(); err != nil {
			return m, m.showErrorCmd(err, stSettings)
		}
		m.state = stMenu
	case "esc":
		m.state = stMenu
	}
	return m, nil
}

func (m *Model) adjustSetting(dir int) {
	thresholdVisible := m.cfg.RetryMode == "threshold"

	if m.settingsCursor == 0 {
		m.cfg.MaxChunkKeys += dir * 10
		if m.cfg.MaxChunkKeys < 10 {
			m.cfg.MaxChunkKeys = 10
		}
		if m.cfg.MaxChunkKeys > 500 {
			m.cfg.MaxChunkKeys = 500
		}
		return
	}

	// Cursor 1 = 失败重试次数
	if m.settingsCursor == 1 {
		m.cfg.MaxRetries += dir
		if m.cfg.MaxRetries < 1 {
			m.cfg.MaxRetries = 1
		}
		if m.cfg.MaxRetries > 10 {
			m.cfg.MaxRetries = 10
		}
		if m.cfg.RetryThreshold > m.cfg.MaxRetries {
			m.cfg.RetryThreshold = m.cfg.MaxRetries
		}
		if m.cfg.RetryThreshold < 0 {
			m.cfg.RetryThreshold = 0
		}
		return
	}

	// Cursor 2 = 重试策略
	if m.settingsCursor == 2 {
		idx := retryModeIndex(m.cfg.RetryMode)
		idx = (idx + dir + len(retryModeValues)) % len(retryModeValues)
		m.cfg.RetryMode = retryModeValues[idx]
		return
	}

	// Cursor 3 = 重试切换阈值 (only if threshold mode)
	if thresholdVisible && m.settingsCursor == 3 {
		m.cfg.RetryThreshold += dir
		if m.cfg.RetryThreshold < 1 {
			m.cfg.RetryThreshold = 1
		}
		if m.cfg.RetryThreshold > m.cfg.MaxRetries {
			m.cfg.RetryThreshold = m.cfg.MaxRetries
		}
		if m.cfg.MaxRetries == 0 {
			m.cfg.RetryThreshold = 0
		}
		return
	}
}
