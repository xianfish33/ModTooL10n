package tui

import (
	"fmt"
	"strings"
	"time"

	"ModTooL10n/engine"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Translate Key Handlers ───────────────────────────────────

func (m *Model) handleTransPathKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v := strings.TrimSpace(m.pathInput.Value()); v != "" {
			m.state = stTransLang
			m.langInput.SetValue("简体中文")
			m.langInput.Focus()
		}
	case "esc":
		m.state = stMenu
	}
	var cmd tea.Cmd
	m.pathInput, cmd = m.pathInput.Update(msg)
	return m, cmd
}

func (m *Model) handleTransLangKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path := strings.TrimSpace(m.pathInput.Value())
		langStr := strings.TrimSpace(m.langInput.Value())
		if langStr == "" {
			langStr = "简体中文"
		}
		m.state = stResolvingLang
		return m, m.resolveLang(path, langStr, false)
	case "esc":
		m.state = stTransPath
	}
	var cmd tea.Cmd
	m.langInput, cmd = m.langInput.Update(msg)
	return m, cmd
}

// ── Language Code Resolution ──────────────────────────────────

func (m *Model) resolveLang(path, langStr string, isBatch bool) tea.Cmd {
	return func() tea.Msg {
		provider, err := m.cfg.GetSelectedProvider()
		if err != nil {
			return langResolvedMsg{err: fmt.Errorf("获取提供商失败: %v", err)}
		}
		modelName := provider.GetActiveModel()
		if modelName == "" {
			return langResolvedMsg{err: fmt.Errorf("当前提供商没有激活的模型")}
		}

		eng := engine.NewEngine(engine.ProviderConfig{
			BaseURL:    provider.BaseURL,
			APIKey:     provider.APIKey,
			ModelName:  modelName,
			MaxKeys:    m.cfg.MaxChunkKeys,
			MaxRetries: m.cfg.MaxRetries,
		})
		code := eng.ResolveLangCode(langStr)
		return langResolvedMsg{code: code}
	}
}

// ── Start Translation ────────────────────────────────────────

func (m *Model) startStreamTranslate(path string, targetLang string, isBatch bool) tea.Cmd {
	if isBatch {
		return m.startBatchTranslate(path, targetLang)
	}
	return m.startSingleTranslate(path, targetLang)
}

func (m *Model) startSingleTranslate(path string, targetLang string) tea.Cmd {
	return func() tea.Msg {
		provider, err := m.cfg.GetSelectedProvider()
		if err != nil {
			return transDoneMsg{err: fmt.Errorf("获取提供商失败: %v", err)}
		}

		eng := engine.NewEngine(engine.ProviderConfig{
			BaseURL:    provider.BaseURL,
			APIKey:     provider.APIKey,
			ModelName:  provider.GetActiveModel(),
			MaxKeys:    m.cfg.MaxChunkKeys,
			MaxRetries: m.cfg.MaxRetries,
		})

		if err := eng.ValidateProvider(); err != nil {
			return transDoneMsg{err: err}
		}

		code := m.targetLangCode
		if code == "" {
			code = targetLang
		}

		pd := &engine.SingleProgress{}
		m.transProgress = pd

		eng.StartSingle(path, targetLang, code, pd)
		return progressTickMsg{}
	}
}

// ── Progress Tick ─────────────────────────────────────────────

func (m *Model) transTick() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return progressTickMsg{}
	})
}

// ── Single Translation Wait View ─────────────────────────────

func (m *Model) transWaitView() string {
	if m.batchProgress != nil {
		return m.batchWaitView()
	}

	var b strings.Builder
	pd := m.transProgress
	if pd == nil {
		return m.loadingView("正在准备...")
	}

	pd.Mu.Lock()
	defer pd.Mu.Unlock()

	barWidth := 20
	if m.width > 80 {
		barWidth = m.width - 40
	}
	m.transBar.Width = barWidth

	switch pd.Phase {
	case "translating":
		b.WriteString(titleStyle.Render(" 正在翻译... "))
		b.WriteString("\n\n")

		doneChunks := 0
		totalParsed := 0
		totalKeys := 0
		for i := 0; i < len(pd.Chunks); i++ {
			cs := pd.Chunks[i]
			totalKeys += cs.KeysCount
			totalParsed += cs.ParsedKeys
			if cs.Status == "done" || (cs.ParsedKeys == cs.KeysCount && cs.KeysCount > 0) {
				doneChunks++
			}
		}

		var overallPct float64
		if totalKeys > 0 {
			overallPct = float64(totalParsed) / float64(totalKeys)
			if overallPct > 1.0 {
				overallPct = 1.0
			}
		}
		overallBar := m.transBar.ViewAs(overallPct)
		b.WriteString(padLine(fmt.Sprintf("%s  分块 %d/%d  键 %d/%d  %.0f%%", overallBar, doneChunks, pd.TotalChunks, totalParsed, totalKeys, overallPct*100)) + "\n\n")

		b.WriteString(padLine(labelStyle.Render("分块详情:")) + "\n")

		for i := 0; i < len(pd.Chunks); i++ {
			cs := pd.Chunks[i]
			var pct float64
			if cs.ParsedKeys > 0 && cs.KeysCount > 0 {
				pct = float64(cs.ParsedKeys) / float64(cs.KeysCount)
				if pct > 1.0 {
					pct = 1.0
				}
			}
			bar := m.transBar.ViewAs(pct)

			completed := cs.ParsedKeys == cs.KeysCount && cs.KeysCount > 0
			statusIcon := inactiveStyle.Render("□")
			switch {
			case completed || cs.Status == "done":
				statusIcon = successStyle.Render("■")
			case cs.Status == "retrying":
				statusIcon = keyStyle.Render(fmt.Sprintf("□ %d/%d", cs.Retry, cs.MaxRetries))
			case cs.Status == "translating":
				statusIcon = keyStyle.Render("□")
			case cs.Status == "failed":
				statusIcon = errStyle.Render("□")
			}

			b.WriteString(padLine(fmt.Sprintf(" %s  分块 %d  %s  %d/%d", statusIcon, i+1, bar, cs.ParsedKeys, cs.KeysCount)) + "\n")
		}

	case "merging":
		b.WriteString(titleStyle.Render(" 正在合并翻译结果... "))
		b.WriteString("\n\n")
		b.WriteString(padLine(m.spinner.View()+" "+dimStyle.Render("合并所有分块的翻译结果...")) + "\n")

	case "saving":
		b.WriteString(titleStyle.Render(" 正在保存翻译文件... "))
		b.WriteString("\n\n")
		b.WriteString(padLine(m.spinner.View()+" "+dimStyle.Render("保存翻译文件并打包JAR...")) + "\n")

	case "completed":
		b.WriteString(titleStyle.Render(" 翻译完成 "))
		b.WriteString("\n\n")
		b.WriteString(padLine(successStyle.Render("翻译完成，文件已保存")) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(padLine(helpStyle.Render("翻译进行中，请稍候...")))
	return b.String()
}

// ── Single Result View ───────────────────────────────────────

func (m *Model) transResultView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 翻译结果 "))
	b.WriteString("\n\n")

	if m.err != nil {
		// 错误应该通过统一的错误页面显示，这里不应该有错误
		// 如果有错误，说明逻辑有问题，应该跳转到错误页面
		b.WriteString(errStyle.Render("错误: "+m.err.Error()) + "\n\n")
		b.WriteString(padLine(helpStyle.Render("按 Enter 或 Esc 返回")))
		return b.String()
	}

	hasErrors := false
	for _, r := range m.transResults {
		if r == nil {
			continue
		}
		if r.Skipped {
			b.WriteString(fmt.Sprintf("  [%s] %s\n", successStyle.Render("已存在"), r.ModID))
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(r.SkipMsg)))
			if r.OutputPath != "" {
				b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render("输出: "+r.OutputPath)))
			}
		} else if r.ChunksFail > 0 && r.ChunksOK == 0 {
			hasErrors = true
			status := "失败"
			if len(r.Errors) > 0 {
				b.WriteString(fmt.Sprintf("  [%s] %s - %s\n", errStyle.Render(status), r.ModID, r.Errors[0]))
			} else {
				b.WriteString(fmt.Sprintf("  [%s] %s\n", errStyle.Render(status), r.ModID))
			}
		} else if r.ChunksFail > 0 {
			hasErrors = true
			b.WriteString(fmt.Sprintf("  [%s] %s", successStyle.Render("部分成功"), r.ModID))
			if r.OutputPath != "" {
				b.WriteString(fmt.Sprintf(" -> %s", r.OutputPath))
			}
			b.WriteString(fmt.Sprintf("  (%d/%d 分块失败, 已用原文填充)", r.ChunksFail, r.ChunksTotal))
			b.WriteString("\n")
		} else {
			b.WriteString(fmt.Sprintf("  [%s] %s", successStyle.Render("成功"), r.ModID))
			if r.OutputPath != "" {
				b.WriteString(fmt.Sprintf(" -> %s", r.OutputPath))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	help := "按 Enter 或 Esc 返回"
	if hasErrors {
		help = "d: 导出错误报告  •  Enter/Esc: 返回"
	}
	b.WriteString(padLine(helpStyle.Render(help)))
	return b.String()
}

func (m *Model) handleTransResultKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d":
		logPath, err := writeErrorReport(m.transResults, nil)
		if err != nil {
			return m, m.showErrorCmd(err, stTransResult)
		}
		return m, m.showErrorCmd(fmt.Errorf("错误报告已导出: %s", logPath), stTransResult)
	case "enter", "esc", " ":
		m.state = stMenu
		m.transResults = nil
		m.err = nil
	}
	return m, nil
}
