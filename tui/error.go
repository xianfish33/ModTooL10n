package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// showErrors 显示错误并跳转到统一错误页面
func (m *Model) showError(err error, prevState state) tea.Cmd {
	m.err = err
	m.prevState = prevState
	m.state = stError
	return nil
}

// showErrorCmd 返回一个显示错误的命令
func (m *Model) showErrorCmd(err error, prevState state) tea.Cmd {
	return func() tea.Msg {
		return errMsg{err: err, prevState: prevState}
	}
}