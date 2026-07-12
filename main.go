package main

import (
	"log"

	"ModTooL10n/config"
	"ModTooL10n/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	log.SetFlags(0)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	p := tea.NewProgram(tui.New(cfg), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		log.Fatalf("程序运行出错: %v", err)
	}
}
