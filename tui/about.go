package tui

import (
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type link struct {
	text string
	url  string
	x    int
	y    int
	w    int
	h    int
}

type openURLCmd struct {
	url string
}

var aboutLinks []link

func (m *Model) aboutView() string {
	aboutLinks = nil

	var all []string
	all = append(all, titleStyle.Render(" 关于 "))
	all = append(all, "")
	all = append(all, padLine(nameStyle.Render("ModTooL10n"))+"  "+dimStyle.Render("v1.0.0"))
	all = append(all, "")
	all = append(all, padLine(labelStyle.Render("Minecraft模组自动汉化工具")))
	all = append(all, padLine(dimStyle.Render("自动提取、翻译、注入模组语言文件")))
	all = append(all, "")
	all = append(all, "")

	links := []struct {
		text  string
		url   string
		color string
	}{
		{"特别谢鸣 - opencodezen 提供免费模型额度", "https://opencode.ai/docs/zh-cn/zen/", "#B7B1B1"},
		{"@咸味闲鱼", "https://github.com/xianfish33", "#3BA3F0"},
		{"GitHub仓库", "https://github.com/example/ModTooL10n/issues", "#FFFFFF"},
	}

	for _, l := range links {
		styled := lipgloss.NewStyle().Foreground(lipgloss.Color(l.color)).Underline(true).Render(l.text)
		lineIdx := len(all)
		all = append(all, padLine(styled))
		aboutLinks = append(aboutLinks, link{
			text: l.text, url: l.url, x: 2, y: lineIdx, w: lipgloss.Width(styled), h: 1,
		})
	}

	all = append(all, "")
	all = append(all, m.viewportPad(all, padLine(helpStyle.Render("按 Esc 返回主菜单")))...)

	return strings.Join(all, "\n")
}

func (m *Model) handleAboutKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter", " ":
		m.state = stMenu
		m.err = nil
	}
	return m, nil
}

func (m *Model) handleAboutMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Type != tea.MouseLeft {
		return m, nil
	}
	for _, lnk := range aboutLinks {
		if msg.Y == lnk.y && msg.X >= lnk.x && msg.X < lnk.x+lnk.w {
			return m, openURL(lnk.url)
		}
	}
	return m, nil
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		cmd.Start()
		return nil
	}
}
