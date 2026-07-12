package tui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"ModTooL10n/config"
	"ModTooL10n/engine"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Provider List Item ────────────────────────────────────────

type providerItem struct {
	provider *config.Provider
}

func (i providerItem) Title() string       { return i.provider.Name }
func (i providerItem) Description() string { return "" }
func (i providerItem) FilterValue() string { return i.provider.Name }

type addProviderItem struct{}

func (a addProviderItem) Title() string       { return "" }
func (a addProviderItem) Description() string { return "" }
func (a addProviderItem) FilterValue() string { return "" }

// ── Provider Delegate ─────────────────────────────────────────

type providerDelegate struct{}

func (d providerDelegate) Height() int                             { return 4 }
func (d providerDelegate) Spacing() int                            { return 1 }
func (d providerDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d providerDelegate) Render(w io.Writer, m list.Model, idx int, item list.Item) {
	pi, ok := item.(providerItem)
	focused := idx == m.Index()

	if !ok {
		if _, isAdd := item.(addProviderItem); isAdd {
			text := nameStyle.Render("+ 添加提供商")
			if focused {
				text = focusLine(text, true)
			} else {
				text = padLine(text)
			}
			fmt.Fprint(w, lipgloss.JoinVertical(lipgloss.Left, text, "", "", ""))
		}
		return
	}

	p := pi.provider

	lines := []string{}

	nameText := nameStyle.Render(p.Name)
	lines = append(lines, focusLine(nameText, focused))
	lines = append(lines, focusLine(dimStyle.Render(p.BaseURL), focused))
	lines = append(lines, focusLine(keyStyle.Render(maskKey(p.APIKey)), focused))

	modelLine := "模型: "
	activeModels := make([]string, 0)
	for _, mm := range p.Models {
		if mm.Active {
			nm := mm.Name
			if mm.Name == p.SelectedModel {
				nm = selModelStyle.Render(mm.Name)
			}
			activeModels = append(activeModels, nm)
		}
	}
	if len(activeModels) > 0 {
		modelLine += strings.Join(activeModels, ", ")
	} else {
		modelLine += dimStyle.Render("(无激活模型)")
	}
	lines = append(lines, focusLine(modelLine, focused))

	block := lipgloss.JoinVertical(lipgloss.Left, lines...)
	fmt.Fprint(w, block)
}

// ── Provider List View ────────────────────────────────────────

func (m *Model) providerListView() string {
	var b strings.Builder
	// 错误应该通过统一的错误页面显示，这里不应该有错误
	// 如果有错误，说明逻辑有问题，应该跳转到错误页面
	if m.err != nil {
		b.WriteString(errStyle.Render("错误: "+m.err.Error()) + "\n\n")
		m.err = nil
	}

	b.WriteString(m.list.View())
	b.WriteString("\n")

	b.WriteString(padLine(helpStyle.Render("方向键: 浏览  •  Enter: 查看详情")))
	return b.String()
}

func (m *Model) handleProviderListKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	totalItems := len(m.cfg.Providers) + 1 // +1 for add provider item
	switch msg.String() {
	case "up", "k":
		if totalItems > 0 {
			idx := m.list.Index()
			m.list.Select((idx - 1 + totalItems) % totalItems)
		}
		return m, nil
	case "down", "j":
		if totalItems > 0 {
			idx := m.list.Index()
			m.list.Select((idx + 1) % totalItems)
		}
		return m, nil
	case "enter", " ":
		idx := m.list.Index()
		if idx >= 0 && idx < len(m.cfg.Providers) {
			m.provider = &m.cfg.Providers[idx]
			m.state = stProviderDetail
			m.detailCursor = 0
		} else if idx == len(m.cfg.Providers) {
			m.state = stAddName
			m.nameInput.Reset()
			m.nameInput.Focus()
			m.err = nil
		}
		return m, nil
	case "esc":
		m.state = stMenu
		m.provider = nil
		return m, nil
	case "ctrl+c", "q":
		m.quitting = true
		return m, tea.Quit
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// ── Provider Detail View ──────────────────────────────────────

func (m *Model) providerDetailView() string {
	p := m.provider
	if p == nil {
		m.state = stMenu
		return ""
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render(" 提供商详情 "))
	b.WriteString("\n\n")

	var infoParts []string
	infoParts = append(infoParts, padLine(labelStyle.Render("名称")+"  "+p.Name))
	infoParts = append(infoParts, "")
	infoParts = append(infoParts, padLine(labelStyle.Render("API地址")+"  "+dimStyle.Render(p.BaseURL)))
	infoParts = append(infoParts, "")
	infoParts = append(infoParts, padLine(labelStyle.Render("API密钥")+"  "+keyStyle.Render(maskKey(p.APIKey))))
	infoParts = append(infoParts, "")
	infoParts = append(infoParts, padLine(labelStyle.Render("模 型")))

	for _, mm := range p.Models {
		text := checkBox(mm.Active)
		if mm.Name == p.SelectedModel {
			text += selModelStyle.Render(mm.Name)
		} else {
			text += mm.Name
		}
		infoParts = append(infoParts, padLine(text))
	}

	b.WriteString(lipgloss.JoinVertical(lipgloss.Left, infoParts...))
	b.WriteString("\n\n")

	b.WriteString(padLine(labelStyle.Render("操作选项")) + "\n")
	labels := actionLabels(m)
	for i, a := range labels {
		line := fmt.Sprintf("  %s", a)
		b.WriteString(focusLine(line, i == m.detailCursor) + "\n")
	}

	// 错误应该通过统一的错误页面显示，这里不应该有错误
	// 如果有错误，说明逻辑有问题，应该跳转到错误页面
	if m.err != nil {
		b.WriteString("\n" + errStyle.Render("错误: "+m.err.Error()))
		m.err = nil
	}

	b.WriteString("\n")
	b.WriteString(padLine(helpStyle.Render("方向键: 浏览  •  Enter: 执行  •  Esc: 返回")))
	return b.String()
}

func actionLabels(m *Model) []string {
	p := m.provider
	if p == nil {
		return nil
	}
	labels := []string{"修改API地址", "修改API密钥", "管理模型"}
	if hasActiveModels(p) {
		labels = append(labels, "选择使用的模型")
	}
	labels = append(labels, "移除该提供商")
	return labels
}

func hasActiveModels(p *config.Provider) bool {
	for _, mm := range p.Models {
		if mm.Active {
			return true
		}
	}
	return false
}

func (m *Model) handleProviderDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	labels := actionLabels(m)
	switch msg.String() {
	case "up", "k":
		wrapCursor(&m.detailCursor, len(labels), -1)
	case "down", "j":
		wrapCursor(&m.detailCursor, len(labels), 1)
	case "enter":
		return m.execProviderAction(m.detailCursor)
	case "esc":
		m.state = stProviderList
	}
	return m, nil
}

func (m *Model) execProviderAction(idx int) (tea.Model, tea.Cmd) {
	p := m.provider
	if p == nil {
		return m, nil
	}

	offset := 0

	if idx == offset {
		m.state = stEditURL
		m.urlInput.SetValue(p.BaseURL)
		m.urlInput.Focus()
		m.err = nil
		return m, nil
	}
	offset++

	if idx == offset {
		m.state = stEditKey
		m.keyInput.Reset()
		m.keyInput.Focus()
		m.err = nil
		return m, nil
	}
	offset++

	if idx == offset {
		m.detailCursor = 0
		m.state = stModelLoading
		m.err = nil
		return m, m.compareModels(p)
	}
	offset++

	if hasActiveModels(p) {
		if idx == offset {
			m.detailCursor = 0
			m.state = stGlobalSelectModel
			m.err = nil
			return m, nil
		}
		offset++
	}

	if idx == offset {
		m.state = stConfirmRemove
		return m, nil
	}

	return m, nil
}

// ── Input View (Add/Edit) ─────────────────────────────────────

// ── Add Provider Handlers ─────────────────────────────────────

func (m *Model) handleAddNameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v := strings.TrimSpace(m.nameInput.Value()); v != "" {
			for _, p := range m.cfg.Providers {
				if p.Name == v {
					return m, m.showErrorCmd(fmt.Errorf("提供商 %q 已存在", v), stProviderList)
				}
			}
			m.state = stAddURL
			m.urlInput.Focus()
		}
	case "esc":
		m.state = stProviderList
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m *Model) handleAddURLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v := strings.TrimSpace(m.urlInput.Value()); v != "" {
			m.state = stAddKey
			m.keyInput.Focus()
		}
	case "esc":
		m.state = stProviderList
	}
	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m *Model) handleAddKeyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(m.nameInput.Value())
		url := strings.TrimSpace(m.urlInput.Value())
		key := strings.TrimSpace(m.keyInput.Value())
		if name == "" || url == "" || key == "" {
			return m, m.showErrorCmd(fmt.Errorf("所有字段不能为空"), stAddKey)
		}
		m.addName = name
		m.addURL = url
		m.addKey = key
		m.state = stLoadingModels
		m.err = nil
		return m, m.fetchModels(name, url, key)
	case "esc":
		m.state = stProviderList
	}
	var cmd tea.Cmd
	m.keyInput, cmd = m.keyInput.Update(msg)
	return m, cmd
}

func (m *Model) fetchModels(name, url, key string) tea.Cmd {
	return func() tea.Msg {
		client := engine.NewLLMClient(url, key, "")
		listURL := url
		if listURL[len(listURL)-1] != '/' {
			listURL += "/"
		}
		listURL += "models"
		models, err := client.ListModels(listURL)
		if err != nil {
			return modelListFailedMsg{err: err, name: name, url: url, key: key}
		}
		m.loadedModels = models
		m.selectedAddModels = append([]string{}, models...)
		return modelsLoadedMsg{models}
	}
}

func (m *Model) fetchModelsWithURL(name, url, key, listURL string) tea.Cmd {
	return func() tea.Msg {
		client := engine.NewLLMClient(url, key, "")
		models, err := client.ListModels(listURL)
		if err != nil {
			return errMsg{err: err, prevState: stModelListURL}
		}
		m.loadedModels = models
		m.selectedAddModels = append([]string{}, models...)
		return modelsLoadedMsg{models}
	}
}

// ── Add Models View ───────────────────────────────────────────

func (m *Model) addModelsView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 选择模型 "))
	b.WriteString("\n\n")
	// 错误应该通过统一的错误页面显示，这里不应该有错误
	// 如果有错误，说明逻辑有问题，应该跳转到错误页面
	if m.err != nil {
		b.WriteString(errStyle.Render("错误: "+m.err.Error()) + "\n\n")
		m.err = nil
	}
	b.WriteString(padLine(labelStyle.Render("空格切换选中, Enter确认")))
	b.WriteString("\n\n")

	total := len(m.loadedModels)
	m.listScroll = m.scrollAdjust(m.detailCursor, total)
	maxVisible := (m.height - 10) / 1
	if maxVisible < 3 {
		maxVisible = 3
	}
	end := m.listScroll + maxVisible
	if end > total {
		end = total
	}

	if m.listScroll > 0 {
		b.WriteString(padLine(dimStyle.Render("↑ 更多...")) + "\n")
	}
	for i := m.listScroll; i < end; i++ {
		mm := m.loadedModels[i]
		sel := false
		for _, s := range m.selectedAddModels {
			if s == mm {
				sel = true
				break
			}
		}
		line := checkBox(sel) + mm
		b.WriteString(focusLine(line, i == m.detailCursor) + "\n")
	}
	if end < total {
		b.WriteString(padLine(dimStyle.Render("↓ 更多...")) + "\n")
	}

	b.WriteString("\n")
	if len(m.selectedAddModels) > 0 {
		b.WriteString(padLine(successStyle.Render(fmt.Sprintf("已选 %d 个模型", len(m.selectedAddModels)))))
	} else {
		b.WriteString(padLine(dimStyle.Render("尚未选择模型")))
	}
	b.WriteString("\n\n")
	b.WriteString(padLine(helpStyle.Render("方向键: 浏览  •  空格: 切换  •  Enter: 确认  •  Esc: 取消")))
	return b.String()
}

func (m *Model) handleAddModelsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		wrapCursor(&m.detailCursor, len(m.loadedModels), -1)
		m.listScroll = m.scrollAdjust(m.detailCursor, len(m.loadedModels))
	case "down", "j":
		wrapCursor(&m.detailCursor, len(m.loadedModels), 1)
		m.listScroll = m.scrollAdjust(m.detailCursor, len(m.loadedModels))
	case " ":
		if m.detailCursor >= 0 && m.detailCursor < len(m.loadedModels) {
			model := m.loadedModels[m.detailCursor]
			found := false
			for i, s := range m.selectedAddModels {
				if s == model {
					m.selectedAddModels = append(m.selectedAddModels[:i], m.selectedAddModels[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				m.selectedAddModels = append(m.selectedAddModels, model)
			}
		}
	case "enter":
		if len(m.selectedAddModels) > 0 {
			if err := m.cfg.AddProvider(m.addName, m.addURL, m.addKey, m.addModelListURL, m.selectedAddModels); err != nil {
				m.err = err
				m.state = stProviderList
				m.refreshList()
				return m, nil
			}
		} else {
			m.err = fmt.Errorf("请至少选择一个模型")
			return m, nil
		}
		m.state = stProviderList
		m.refreshList()
	case "esc":
		m.state = stProviderList
	}
	return m, nil
}

// ── Edit Handlers ─────────────────────────────────────────────

func (m *Model) handleEditURLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v := strings.TrimSpace(m.urlInput.Value()); v != "" {
			if err := m.cfg.UpdateProvider(m.provider.Name, v, ""); err != nil {
				return m, m.showErrorCmd(err, stProviderDetail)
			}
			m.state = stProviderDetail
		}
	case "esc":
		m.state = stProviderDetail
	}
	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m *Model) handleEditKeyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		if v := strings.TrimSpace(m.keyInput.Value()); v != "" {
			if err := m.cfg.UpdateProvider(m.provider.Name, "", v); err != nil {
				return m, m.showErrorCmd(err, stProviderDetail)
			}
			m.state = stProviderDetail
		}
	case "esc":
		m.state = stProviderDetail
	}
	var cmd tea.Cmd
	m.keyInput, cmd = m.keyInput.Update(msg)
	return m, cmd
}

// ── Model Toggle (with auto-sync comparison) ──────────────────

func (m *Model) compareModels(p *config.Provider) tea.Cmd {
	return func() tea.Msg {
		client := engine.NewLLMClient(p.BaseURL, p.APIKey, "")
		listURL := p.ModelListURL
		if listURL == "" {
			listURL = p.BaseURL
			if listURL[len(listURL)-1] != '/' {
				listURL += "/"
			}
			listURL += "models"
		}
		cloudModels, err := client.ListModels(listURL)
		if err != nil {
			if p.ModelListURL == "" {
				return modelListURLEditMsg{err: err}
			}
			return errMsg{err: err, prevState: stModelLoading}
		}

		var unavailable []string
		for _, local := range p.Models {
			found := false
			for _, cloud := range cloudModels {
				if local.Name == cloud {
					found = true
					break
				}
			}
			if !found {
				unavailable = append(unavailable, local.Name)
			}
		}

		var merged []config.ModelConfig
		for _, cloud := range cloudModels {
			found := false
			for _, local := range p.Models {
				if local.Name == cloud {
					merged = append(merged, local)
					found = true
					break
				}
			}
			if !found {
				merged = append(merged, config.ModelConfig{Name: cloud, Active: false})
			}
		}

		p.Models = merged
		if err := m.cfg.Save(); err != nil {
			return errMsg{err: err, prevState: stModelToggle}
		}

		m.cloudModels = cloudModels
		m.unavailableModels = unavailable
		return modelsLoadedMsg{cloudModels}
	}
}

func (m *Model) modelToggleView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 激活/停用模型 "))
	b.WriteString("\n\n")
	// 错误应该通过统一的错误页面显示，这里不应该有错误
	// 如果有错误，说明逻辑有问题，应该跳转到错误页面
	if m.err != nil {
		b.WriteString(errStyle.Render("错误: "+m.err.Error()) + "\n\n")
		m.err = nil
	}

	if len(m.unavailableModels) > 0 {
		b.WriteString(warnStyle.Render(fmt.Sprintf("检测到 %d 个模型在服务端已不可用，按 Enter 确认删除", len(m.unavailableModels))))
		b.WriteString("\n\n")
	}

	b.WriteString(padLine(labelStyle.Render("空格切换激活状态")))
	b.WriteString("\n\n")

	total := len(m.provider.Models)
	m.listScroll = m.scrollAdjust(m.detailCursor, total)
	maxVisible := (m.height - 10) / 1
	if maxVisible < 3 {
		maxVisible = 3
	}
	end := m.listScroll + maxVisible
	if end > total {
		end = total
	}

	if m.listScroll > 0 {
		b.WriteString(padLine(dimStyle.Render("↑ 更多...")) + "\n")
	}
	for i := m.listScroll; i < end; i++ {
		mm := m.provider.Models[i]
		unavailable := false
		for _, u := range m.unavailableModels {
			if mm.Name == u {
				unavailable = true
				break
			}
		}

		var text string
		if unavailable {
			text = unavailStyle.Render("□ " + mm.Name + " (已不可用)")
		} else {
			text = checkBox(mm.Active)
			if mm.Name == m.provider.SelectedModel {
				text += selModelStyle.Render(mm.Name)
			} else {
				text += mm.Name
			}
		}
		b.WriteString(focusLine(text, i == m.detailCursor) + "\n")
	}
	if end < total {
		b.WriteString(padLine(dimStyle.Render("↓ 更多...")) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(padLine(helpStyle.Render("方向键: 浏览  •  空格: 切换  •  Enter: 确认  •  Esc: 返回")))
	return b.String()
}

func (m *Model) handleModelToggleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		wrapCursor(&m.detailCursor, len(m.provider.Models), -1)
		m.listScroll = m.scrollAdjust(m.detailCursor, len(m.provider.Models))
	case "down", "j":
		wrapCursor(&m.detailCursor, len(m.provider.Models), 1)
		m.listScroll = m.scrollAdjust(m.detailCursor, len(m.provider.Models))
	case " ":
		if m.detailCursor >= 0 && m.detailCursor < len(m.provider.Models) {
			modelName := m.provider.Models[m.detailCursor].Name
			for _, u := range m.unavailableModels {
				if modelName == u {
					return m, nil
				}
			}
			if err := m.cfg.ToggleModelActive(m.provider.Name, modelName); err != nil {
				return m, m.showErrorCmd(err, stModelToggle)
			}
		}
	case "enter":
		for _, u := range m.unavailableModels {
			if err := m.cfg.RemoveModel(m.provider.Name, u); err != nil {
				return m, m.showErrorCmd(err, stModelToggle)
			}
		}
		m.unavailableModels = nil
		m.cloudModels = nil
		m.state = stProviderDetail
	case "esc":
		m.unavailableModels = nil
		m.cloudModels = nil
		m.state = stProviderDetail
	}
	return m, nil
}

// ── Test Model Connectivity ───────────────────────────────────

func (m *Model) testModelConnectivityGlobal(providerName, modelName string) tea.Cmd {
	return func() tea.Msg {
		var provider *config.Provider
		for i := range m.cfg.Providers {
			if m.cfg.Providers[i].Name == providerName {
				provider = &m.cfg.Providers[i]
				break
			}
		}
		if provider == nil {
			return testModelDoneMsg{err: fmt.Errorf("未找到提供商 %s", providerName)}
		}

		client := engine.NewLLMClient(provider.BaseURL, provider.APIKey, modelName)
		start := time.Now()
		_, err := client.Chat("你是一个测试助手", "请回复'连接成功'四个字")
		latency := time.Since(start)

		return testModelDoneMsg{err: err, latency: latency}
	}
}

func (m *Model) testModelResultView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 测试模型连通性 "))
	b.WriteString("\n\n")

	if m.testModelErr != nil {
		b.WriteString(errStyle.Render("测试失败") + "\n\n")
		b.WriteString(padLine(errStyle.Render(m.testModelErr.Error())) + "\n\n")
		b.WriteString(padLine(helpStyle.Render("按 Enter 或 Esc 返回")))
	} else {
		b.WriteString(successStyle.Render("测试成功") + "\n\n")
		b.WriteString(padLine(fmt.Sprintf("响应时间: %dms", m.testModelLatency.Milliseconds())) + "\n\n")
		b.WriteString(padLine(helpStyle.Render("按 Enter 或 Esc 返回")))
	}

	return b.String()
}

func (m *Model) handleTestModelResultKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.state = stGlobalSelectModel
		m.detailCursor = 0
		m.testModelErr = nil
		m.testModelLatency = 0
	}
	return m, nil
}

// ── Global Model Selection (separate page) ────────────────────

func (m *Model) globalSelectModelView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" 选择使用的模型 "))
	b.WriteString("\n\n")
	// 错误应该通过统一的错误页面显示，这里不应该有错误
	// 如果有错误，说明逻辑有问题，应该跳转到错误页面
	if m.err != nil {
		b.WriteString(errStyle.Render("错误: "+m.err.Error()) + "\n\n")
		m.err = nil
	}

	if len(m.cfg.Providers) == 0 {
		b.WriteString(dimStyle.Render("   暂无提供商，请先添加") + "\n\n")
		b.WriteString(padLine(helpStyle.Render("暂无提供商，请先添加")))
		return b.String()
	}

	type pair struct{ provider, model string }
	var pairs []pair
	for _, p := range m.cfg.Providers {
		for _, mm := range p.Models {
			if mm.Active {
				pairs = append(pairs, pair{p.Name, mm.Name})
			}
		}
	}

	if len(pairs) == 0 {
		b.WriteString(dimStyle.Render("   没有已激活的模型") + "\n\n")
		b.WriteString(padLine(helpStyle.Render("没有已激活的模型，请先在提供商管理中激活")))
		return b.String()
	}

	total := len(pairs)
	m.listScroll = m.scrollAdjust(m.detailCursor, total)
	maxVisible := (m.height - 8) / 1
	if maxVisible < 3 {
		maxVisible = 3
	}
	end := m.listScroll + maxVisible
	if end > total {
		end = total
	}

	if m.listScroll > 0 {
		b.WriteString(padLine(dimStyle.Render("↑ 更多...")) + "\n")
	}
	for i := m.listScroll; i < end; i++ {
		item := pairs[i]
		isCurrent := false
		for _, pp := range m.cfg.Providers {
			if pp.Name == item.provider && pp.SelectedModel == item.model {
				isCurrent = true
				break
			}
		}

		text := fmt.Sprintf("[%s] %s", item.provider, item.model)
		if isCurrent {
			text = checkBox(true) + selModelStyle.Render(text)
		} else {
			text = "  " + text
		}
		b.WriteString(focusLine(text, i == m.detailCursor) + "\n")
	}
	if end < total {
		b.WriteString(padLine(dimStyle.Render("↓ 更多...")) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(padLine(helpStyle.Render("方向键: 浏览  •  空格: 选择  •  t: 测试连通性  •  Enter/Esc: 退出")))
	return b.String()
}

func (m *Model) handleGlobalSelectModelKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	type pair struct{ provider, model string }
	var items []pair
	for _, p := range m.cfg.Providers {
		for _, mm := range p.Models {
			if mm.Active {
				items = append(items, pair{p.Name, mm.Name})
			}
		}
	}

	switch msg.String() {
	case "up", "k":
		wrapCursor(&m.detailCursor, len(items), -1)
		m.listScroll = m.scrollAdjust(m.detailCursor, len(items))
	case "down", "j":
		wrapCursor(&m.detailCursor, len(items), 1)
		m.listScroll = m.scrollAdjust(m.detailCursor, len(items))
	case " ":
		if m.detailCursor >= 0 && m.detailCursor < len(items) {
			item := items[m.detailCursor]
			m.cfg.SelectModel(item.provider, item.model)
		}
	case "t", "T":
		if m.detailCursor >= 0 && m.detailCursor < len(items) {
			item := items[m.detailCursor]
			m.state = stTestModel
			return m, m.testModelConnectivityGlobal(item.provider, item.model)
		}
	case "enter", "esc":
		m.state = stMenu
	}
	return m, nil
}

// ── Confirm Remove ────────────────────────────────────────────

func (m *Model) confirmRemoveView() string {
	var b strings.Builder
	b.WriteString(errStyle.Render(" 确认移除 "))
	b.WriteString("\n\n")
	b.WriteString(padLine(fmt.Sprintf("确定要移除提供商 %q 吗？", m.provider.Name)) + "\n\n")
	b.WriteString(padLine(helpStyle.Render("y: 确认移除  •  n/esc: 取消")))
	return b.String()
}

func (m *Model) handleConfirmRemoveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := m.cfg.RemoveProvider(m.provider.Name); err != nil {
			return m, m.showErrorCmd(err, stProviderDetail)
		}
		m.refreshList()
		m.provider = nil
		m.state = stProviderList
	case "n", "N", "esc":
		m.state = stProviderDetail
	}
	return m, nil
}

// ── Helpers ───────────────────────────────────────────────────

func (m *Model) loadingView(msg string) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" ModTooL10n "))
	b.WriteString("\n\n")
	b.WriteString(padLine(m.spinner.View() + " " + msg))
	b.WriteString("\n\n")
	b.WriteString(padLine(helpStyle.Render("翻译进行中，请稍候...")))
	return b.String()
}

func (m *Model) handleModelListURLKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		url := strings.TrimSpace(m.urlInput.Value())
		if url == "" {
			return m, m.showErrorCmd(fmt.Errorf("请输入模型列表地址"), m.state)
		}
		if m.state == stModelListURL {
			m.addModelListURL = url
			m.state = stLoadingModels
			return m, m.fetchModelsWithURL(m.addName, m.addURL, m.addKey, url)
		}
		m.provider.ModelListURL = url
		if err := m.cfg.Save(); err != nil {
			return m, m.showErrorCmd(err, stProviderDetail)
		}
		m.state = stModelLoading
		m.detailCursor = 0
		return m, m.compareModels(m.provider)
	case "esc":
		if m.state == stModelListURL {
			m.state = stProviderList
		} else {
			m.state = stProviderDetail
		}
	}
	var cmd tea.Cmd
	m.urlInput, cmd = m.urlInput.Update(msg)
	return m, cmd
}

func (m *Model) scrollAdjust(cursor, total int) int {
	maxVisible := (m.height - 6) / 1
	if maxVisible < 3 {
		maxVisible = 3
	}
	if total <= maxVisible {
		return 0
	}
	offset := m.listScroll
	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+maxVisible {
		offset = cursor - maxVisible + 1
	}
	return offset
}

func (m *Model) refreshList() {
	items := make([]list.Item, len(m.cfg.Providers)+1)
	for i := range m.cfg.Providers {
		items[i] = providerItem{provider: &m.cfg.Providers[i]}
	}
	items[len(m.cfg.Providers)] = addProviderItem{}
	m.list.SetItems(items)
	if m.list.Width() <= 0 {
		m.list.SetSize(m.width-4, m.height-6)
	}
}
