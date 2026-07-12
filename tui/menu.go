package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type menuState struct {
	cursor int
}

const menuItemCount = 5

func (m *Model) menuView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" ModTooL10n - Minecraft模组自动汉化工具 "))
	b.WriteString("\n\n")

	items := []string{
		"翻译单个Mod",
		"批量翻译Mods目录",
		"管理翻译提供商",
		"选择使用的模型",
		"关于",
	}

	for i, item := range items {
		line := fmt.Sprintf("  %s", item)
		b.WriteString(focusLine(line, i == m.menuCursor) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(padLine(helpStyle.Render("方向键: 浏览  •  Enter: 选择  •  Esc: 退出")))
	return b.String()
}

func (m *Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		wrapCursor(&m.menuCursor, menuItemCount, -1)
	case "down", "j":
		wrapCursor(&m.menuCursor, menuItemCount, 1)
	case "enter":
		switch m.menuCursor {
		case 0: // 翻译单个Mod
			m.state = stTransPath
			m.pathInput.Reset()
			m.pathInput.Focus()
			m.err = nil
		case 1: // 批量翻译Mods目录
			m.state = stBatchPath
			m.pathInput.Reset()
			m.pathInput.Focus()
			m.err = nil
		case 2: // 管理翻译提供商
			m.state = stProviderList
			m.list.Select(0)
		case 3: // 选择使用的模型
			m.state = stGlobalSelectModel
			m.detailCursor = 0
		case 4: // 关于
			m.state = stAbout
		}
	case "q", "esc", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}
