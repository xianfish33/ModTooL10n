package tui

import (
	"fmt"
	"strings"

	"ModTooL10n/engine"

	tea "github.com/charmbracelet/bubbletea"
)

// ── Batch Key Handlers ───────────────────────────────────────

func (m *Model) handleBatchPathKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v := strings.TrimSpace(m.pathInput.Value()); v != "" {
			m.state = stBatchLang
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

func (m *Model) handleBatchLangKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		path := strings.TrimSpace(m.pathInput.Value())
		langStr := strings.TrimSpace(m.langInput.Value())
		if langStr == "" {
			langStr = "简体中文"
		}
		m.state = stBatchResolving
		return m, m.resolveLang(path, langStr, true)
	case "esc":
		m.state = stBatchPath
	}
	var cmd tea.Cmd
	m.langInput, cmd = m.langInput.Update(msg)
	return m, cmd
}

// ── Start Batch Translation ──────────────────────────────────

func (m *Model) startBatchTranslate(path string, targetLang string) tea.Cmd {
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

		bp := &engine.BatchProgress{}
		m.batchProgress = bp

		eng.StartBatch(path, targetLang, code, bp)
		return progressTickMsg{}
	}
}

// ── Batch Mouse Handler ──────────────────────────────────────

func (m *Model) handleBatchMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.batchProgress == nil {
		return m, nil
	}

	bp := m.batchProgress
	bp.Mu.Lock()
	defer bp.Mu.Unlock()

	switch msg.Type {
	case tea.MouseWheelUp:
		if bp.ScrollOffset > 0 {
			bp.ScrollOffset--
		}
	case tea.MouseWheelDown:
		bp.ScrollOffset++
	}

	return m, nil
}

// ── Batch Wait View (multi-mod progress) ─────────────────────

func (m *Model) batchWaitView() string {
	bp := m.batchProgress
	if bp == nil {
		return m.loadingView("正在准备...")
	}

	bp.Mu.Lock()
	defer bp.Mu.Unlock()

	barWidth := 20
	if m.width > 60 {
		barWidth = m.width - 40
	}
	m.transBar.Width = barWidth

	// ── Phase: preparing ──
	if bp.Phase == "preparing" {
		var all []string
		all = append(all, titleStyle.Render(" 正在批量翻译... "))
		all = append(all, "")
		all = append(all, padLine(m.spinner.View()+" "+dimStyle.Render("正在解压mod文件...")))
		all = append(all, m.viewportPad(all, padLine(helpStyle.Render("翻译进行中，请稍候...  滚轮: 上下查看")))...)
		return strings.Join(all, "\n")
	}

	// ── Phase: completed ──
	if bp.Phase == "completed" {
		var all []string
		all = append(all, titleStyle.Render(" 批量翻译完成 "))
		all = append(all, "")
		all = append(all, padLine(successStyle.Render(fmt.Sprintf("共翻译 %d 个mod", bp.TotalMods))))
		all = append(all, m.viewportPad(all, padLine(helpStyle.Render("翻译进行中，请稍候...  滚轮: 上下查看")))...)
		return strings.Join(all, "\n")
	}

	// ── Phase: translating ──
	// 1. Build header lines (fixed, not scrollable)
	var header []string
	header = append(header, titleStyle.Render(" 正在批量翻译... "))
	header = append(header, "")

	// Calculate overall stats
	doneMods := 0
	totalParsedKeys := 0
	totalAllKeys := 0
	totalDoneChunks := 0
	totalAllChunks := 0
	for _, mp := range bp.Mods {
		totalAllKeys += mp.TotalKeys
		totalAllChunks += mp.TotalChunks
		for _, cs := range mp.Chunks {
			totalParsedKeys += cs.ParsedKeys
			if cs.Status == "done" || (cs.ParsedKeys == cs.KeysCount && cs.KeysCount > 0) {
				totalDoneChunks++
			}
		}
		if mp.Phase == "completed" || mp.Phase == "failed" {
			doneMods++
		}
	}

	var overallPct float64
	if totalAllKeys > 0 {
		overallPct = float64(totalParsedKeys) / float64(totalAllKeys)
		if overallPct > 1.0 {
			overallPct = 1.0
		}
	}
	overallBar := m.transBar.ViewAs(overallPct)
	header = append(header, padLine(labelStyle.Render("mod总进度:")))
	header = append(header, padLine(fmt.Sprintf("%s  Mod %d/%d  分块 %d/%d  键 %d/%d  %.0f%%",
		overallBar, doneMods, bp.TotalMods, totalDoneChunks, totalAllChunks, totalParsedKeys, totalAllKeys, overallPct*100)))
	header = append(header, "")

	// 2. Build footer line (fixed, not scrollable)
	footer := padLine(helpStyle.Render("翻译进行中，请稍候...  滚轮: 上下查看"))

	// 3. Build ALL scrollable content lines (one per mod, with chunk details)
	var contentLines [][]string
	for _, mp := range bp.Mods {
		var lines []string

		modTitle := mp.ModName
		if modTitle == "" {
			modTitle = mp.ModID
		}

		// Show green text if language file already exists, skip progress
		if mp.LangExists {
			lines = append(lines, padLine(labelStyle.Render(fmt.Sprintf("  %s (%s)", modTitle, mp.ModID))))
			lines = append(lines, padLine(successStyle.Render(fmt.Sprintf("    %s", mp.LangMsg))))
			contentLines = append(contentLines, lines)
			continue
		}

		lines = append(lines, padLine(labelStyle.Render(fmt.Sprintf("  %s (%s)", modTitle, mp.ModID))))

		var modParsed int
		var modDoneChunks int
		for _, cs := range mp.Chunks {
			modParsed += cs.ParsedKeys
			if cs.Status == "done" || (cs.ParsedKeys == cs.KeysCount && cs.KeysCount > 0) {
				modDoneChunks++
			}
		}

		var modPct float64
		if mp.TotalKeys > 0 {
			modPct = float64(modParsed) / float64(mp.TotalKeys)
			if modPct > 1.0 {
				modPct = 1.0
			}
		}
		modBar := m.transBar.ViewAs(modPct)

		statusText := ""
		switch mp.Phase {
		case "pending":
			statusText = dimStyle.Render("等待中")
		case "translating":
			statusText = keyStyle.Render("翻译中")
		case "merging":
			statusText = dimStyle.Render("合并中")
		case "saving":
			statusText = dimStyle.Render("保存中")
		case "completed":
			statusText = successStyle.Render("完成")
		case "failed":
			statusText = errStyle.Render("失败")
		}

		lines = append(lines, padLine(fmt.Sprintf("    %s  %s  分块 %d/%d  键 %d/%d",
			modBar, statusText, modDoneChunks, mp.TotalChunks, modParsed, mp.TotalKeys)))

		for j, cs := range mp.Chunks {
			var chunkPct float64
			if cs.ParsedKeys > 0 && cs.KeysCount > 0 {
				chunkPct = float64(cs.ParsedKeys) / float64(cs.KeysCount)
				if chunkPct > 1.0 {
					chunkPct = 1.0
				}
			}
			chunkBar := m.transBar.ViewAs(chunkPct)

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

			lines = append(lines, padLine(fmt.Sprintf("      %s  分块 %d  %s  %d/%d", statusIcon, j+1, chunkBar, cs.ParsedKeys, cs.KeysCount)))
		}

		contentLines = append(contentLines, lines)
	}

	// 4. Calculate total content lines
	totalContent := 0
	for _, cl := range contentLines {
		totalContent += len(cl)
	}

	// Pad to even number of lines so scroll offset (×2) aligns correctly
	if totalContent%2 != 0 && len(contentLines) > 0 {
		last := contentLines[len(contentLines)-1]
		contentLines[len(contentLines)-1] = append(last, "")
		totalContent++
	}

	// 5. Viewport height for content = total height - header - footer - 2 (always reserve for both indicators)
	viewportHeight := m.height - len(header) - 1 - 2
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	// 6. Clamp scroll offset (stable: indicators don't affect contentHeight)
	scrollLines := bp.ScrollOffset * 2
	maxScrollLines := totalContent - viewportHeight
	if maxScrollLines < 0 {
		maxScrollLines = 0
	}
	if scrollLines > maxScrollLines {
		scrollLines = maxScrollLines
	}
	if scrollLines < 0 {
		scrollLines = 0
	}
	bp.ScrollOffset = (scrollLines + 1) / 2

	// 7. Determine indicators based on final position
	showUp := scrollLines > 0
	showDown := scrollLines+viewportHeight < totalContent

	// 8. Flatten content lines and slice
	var flatContent []string
	for _, cl := range contentLines {
		flatContent = append(flatContent, cl...)
	}

	startLine := scrollLines
	endLine := startLine + viewportHeight
	if endLine > len(flatContent) {
		endLine = len(flatContent)
	}

	// 9. Assemble: header + [up indicator] + visible content + [down indicator] + footer
	var all []string
	all = append(all, header...)
	if showUp {
		all = append(all, padLine(dimStyle.Render("  ▲ 向上滚动查看更多")))
	}
	all = append(all, flatContent[startLine:endLine]...)
	if showDown {
		all = append(all, padLine(dimStyle.Render("  ▼ 向下滚动查看更多")))
	}

	// Pad remaining to push footer to bottom
	for len(all) < m.height-1 {
		all = append(all, "")
	}
	all = append(all, footer)

	return strings.Join(all, "\n")
}

// viewportPad returns lines needed to push helpLine to viewport bottom.
func (m *Model) viewportPad(current []string, helpLine string) []string {
	var pad []string
	for len(current)+len(pad) < m.height-1 {
		pad = append(pad, "")
	}
	pad = append(pad, helpLine)
	return pad
}

// ── Batch Result View ────────────────────────────────────────

func (m *Model) batchResultView() string {
	bp := m.batchProgress

	var all []string
	all = append(all, titleStyle.Render(" 批量翻译结果 "))
	all = append(all, "")

	footer := padLine(helpStyle.Render("按 Enter 或 Esc 返回"))

	if m.err != nil {
		// 错误应该通过统一的错误页面显示，这里不应该有错误
		// 如果有错误，说明逻辑有问题，应该跳转到错误页面
		all = append(all, errStyle.Render("错误: "+m.err.Error()))
		all = append(all, "")
		all = append(all, m.viewportPad(all, footer)...)
		return strings.Join(all, "\n")
	}

	if bp == nil {
		all = append(all, padLine(dimStyle.Render("无翻译结果")))
		all = append(all, "")
		all = append(all, m.viewportPad(all, footer)...)
		return strings.Join(all, "\n")
	}

	bp.Mu.Lock()
	defer bp.Mu.Unlock()

	if !bp.AllDone {
		all = append(all, padLine(dimStyle.Render("翻译尚未完成...")))
		all = append(all, "")
		all = append(all, m.viewportPad(all, footer)...)
		return strings.Join(all, "\n")
	}

	if bp.Err != nil && len(bp.Results) == 0 {
		all = append(all, errStyle.Render("错误: "+bp.Err.Error()))
		all = append(all, "")
		all = append(all, m.viewportPad(all, footer)...)
		return strings.Join(all, "\n")
	}

	if len(bp.Results) == 0 {
		all = append(all, padLine(dimStyle.Render("无翻译结果")))
		all = append(all, "")
		all = append(all, m.viewportPad(all, footer)...)
		return strings.Join(all, "\n")
	}

	outputDir := engine.OutputRoot + "/"
	all = append(all, padLine(labelStyle.Render("输出目录: "+outputDir)))
	all = append(all, "")

	hasErrors := false
	for _, r := range bp.Results {
		if r == nil {
			continue
		}
		if r.Skipped {
			all = append(all, fmt.Sprintf("  [%s] %s", successStyle.Render("已存在"), r.ModID))
			all = append(all, fmt.Sprintf("    %s", dimStyle.Render(r.SkipMsg)))
			if r.OutputPath != "" {
				all = append(all, fmt.Sprintf("    %s", dimStyle.Render("输出: "+r.OutputPath)))
			}
		} else if r.ChunksFail > 0 && r.ChunksOK == 0 {
			hasErrors = true
			status := "失败"
			if len(r.Errors) > 0 {
				all = append(all, fmt.Sprintf("  [%s] %s - %s", errStyle.Render(status), r.ModID, r.Errors[0]))
			} else {
				all = append(all, fmt.Sprintf("  [%s] %s", errStyle.Render(status), r.ModID))
			}
		} else if r.ChunksFail > 0 {
			hasErrors = true
			line := fmt.Sprintf("  [%s] %s", successStyle.Render("部分成功"), r.ModID)
			if r.OutputPath != "" {
				line += fmt.Sprintf(" -> %s", r.OutputPath)
			}
			line += fmt.Sprintf("  (%d/%d 分块失败, 已用原文填充)", r.ChunksFail, r.ChunksTotal)
			all = append(all, line)
		} else {
			line := fmt.Sprintf("  [%s] %s", successStyle.Render("成功"), r.ModID)
			if r.OutputPath != "" {
				line += fmt.Sprintf(" -> %s", r.OutputPath)
			}
			all = append(all, line)
		}
	}

	helpText := "按 Enter 或 Esc 返回"
	if hasErrors {
		helpText = "d: 导出错误报告  •  Enter/Esc: 返回"
	}

	all = append(all, m.viewportPad(all, padLine(helpStyle.Render(helpText)))...)
	return strings.Join(all, "\n")
}

func (m *Model) handleBatchResultKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "d":
		bp := m.batchProgress
		var batchErr error
		var results []*engine.Result
		if bp != nil {
			bp.Mu.Lock()
			batchErr = bp.Err
			results = bp.Results
			bp.Mu.Unlock()
		}
		logPath, err := writeErrorReport(results, batchErr)
		if err != nil {
			return m, m.showErrorCmd(err, stBatchResult)
		}
		return m, m.showErrorCmd(fmt.Errorf("错误报告已导出: %s", logPath), stBatchResult)
	case "enter", "esc", " ":
		m.state = stMenu
		m.batchProgress = nil
		m.err = nil
	}
	return m, nil
}
