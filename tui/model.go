package tui

import (
	"fmt"
	"strings"
	"time"

	"ModTooL10n/config"
	"ModTooL10n/engine"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type state int

type langResolvedMsg struct {
	code string
	err  error
}

const (
	stMenu state = iota

	// Provider management
	stProviderList
	stProviderDetail
	stAddName
	stAddURL
	stAddKey
	stAddModels
	stEditURL
	stEditKey
	stModelToggle
	stConfirmRemove
	stSyncing
	stLoadingModels
	stModelLoading
	stModelListURL
	stModelListURLEdit

	// Global model selection
	stGlobalSelectModel

	// Test model
	stTestModel
	stTestModelResult

	// Translate single mod
	stTransPath
	stTransLang
	stTransOutputMode
	stTransPackName
	stResolvingLang
	stTransWait
	stTransResult

	// Batch translate
	stBatchPath
	stBatchLang
	stBatchOutputMode
	stBatchPackName
	stBatchResolving
	stBatchWait
	stBatchResult

	// Error
	stError

	// About
	stAbout

	// Settings
	stSettings
)

type (
	modelsLoadedMsg    struct{ models []string }
	modelListFailedMsg struct {
		err  error
		name string
		url  string
		key  string
	}
	modelListURLEditMsg struct{ err error }
	errMsg              struct {
		err       error
		prevState state
	}
	testModelDoneMsg struct {
		err     error
		latency time.Duration
	}

	transDoneMsg struct {
		results []*engine.Result
		err     error
	}
)

type progressTickMsg struct{}

type valJob struct {
	origPath  string
	transPath string
}

type Model struct {
	state state

	cfg      *config.Config
	provider *config.Provider

	list    list.Model
	spinner spinner.Model

	nameInput textinput.Model
	urlInput  textinput.Model
	keyInput  textinput.Model
	pathInput textinput.Model
	langInput textinput.Model
	packNameInput textinput.Model

	loadedModels      []string
	selectedAddModels []string
	addName           string
	addURL            string
	addKey            string
	addModelListURL   string
	detailCursor      int
	listScroll        int
	menuCursor        int
	transStatus       string
	targetLangCode    string
	outputMode        string
	outputModeCursor  int
	packName          string
	transResults      []*engine.Result

	unavailableModels []string
	cloudModels       []string

	testModelErr     error
	testModelLatency time.Duration

	transProgress *engine.SingleProgress
	batchProgress *engine.BatchProgress
	transBar      progress.Model

	width     int
	height    int
	err       error
	prevState state
	quitting  bool
	settingsCursor int
}

func New(cfg *config.Config) *Model {
	items := make([]list.Item, len(cfg.Providers)+1)
	for i := range cfg.Providers {
		items[i] = providerItem{provider: &cfg.Providers[i]}
	}
	items[len(cfg.Providers)] = addProviderItem{}

	l := list.New(items, providerDelegate{}, 0, 0)
	l.Title = "翻译提供商管理"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.Styles.Title = titleStyle
	l.DisableQuitKeybindings()
	l.SetShowHelp(false)

	makeInput := func(placeholder string, width int, echo textinput.EchoMode) textinput.Model {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = placeholder
		ti.CharLimit = 512
		ti.Width = width
		if echo != textinput.EchoNormal {
			ti.EchoMode = echo
		}
		return ti
	}

	s := spinner.New()
	s.Style = spinnerStyle

	pb := progress.New(
		progress.WithSolidFill("#3B82F6"),
		progress.WithFillCharacters('█', '░'),
	)

	return &Model{
		state:     stMenu,
		cfg:       cfg,
		list:      l,
		spinner:   s,
		nameInput: makeInput("输入名称...", 60, textinput.EchoNormal),
		urlInput:  makeInput("https://api.openai.com/v1", 60, textinput.EchoNormal),
		keyInput:  makeInput("sk-...", 60, textinput.EchoPassword),
		pathInput: makeInput("输入文件或目录路径...", 60, textinput.EchoNormal),
		langInput: makeInput("简体中文", 60, textinput.EchoNormal),
		packNameInput: makeInput("输入资源包名称...", 60, textinput.EchoNormal),
		transBar:  pb,
	}
}

func (m *Model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width-4, msg.Height-6)

	case tea.KeyMsg:
		if m.quitting {
			return m, tea.Quit
		}
		return m.handleKey(msg)

	case tea.MouseMsg:
		if m.batchProgress != nil && (m.state == stBatchWait || m.state == stBatchResult) {
			return m.handleBatchMouse(msg)
		}
		if m.state == stAbout {
			return m.handleAboutMouse(msg)
		}

	case modelsLoadedMsg:
		m.loadedModels = msg.models
		if m.state == stModelLoading {
			m.detailCursor = 0
			m.state = stModelToggle
		} else {
			m.detailCursor = 0
			m.state = stAddModels
		}
		return m, nil

	case transDoneMsg:
		if msg.err != nil {
			return m, m.showErrorCmd(msg.err, m.state)
		}
		m.transResults = msg.results
		if m.state == stTransWait {
			m.state = stTransResult
		} else {
			m.state = stBatchResult
		}
		return m, nil

	case modelListFailedMsg:
		m.addName = msg.name
		m.addURL = msg.url
		m.addKey = msg.key
		m.urlInput.SetValue("")
		m.urlInput.Placeholder = "模型列表API地址 (默认: base_url/models)"
		m.urlInput.Focus()
		m.state = stModelListURL
		m.prevState = stAddKey
		return m, nil

	case modelListURLEditMsg:
		m.urlInput.SetValue("")
		m.urlInput.Placeholder = "模型列表API地址 (默认: base_url/models)"
		m.urlInput.Focus()
		m.state = stModelListURLEdit
		m.prevState = stProviderDetail
		return m, nil

	case langResolvedMsg:
		if msg.err != nil {
			return m, m.showErrorCmd(msg.err, m.state)
		}
		m.targetLangCode = msg.code
		isBatch := m.state == stBatchResolving
		if isBatch {
			m.state = stBatchWait
		} else {
			m.state = stTransWait
		}
		path := m.pathInput.Value()
		langStr := m.langInput.Value()
		if langStr == "" {
			langStr = "简体中文"
		}
		return m, m.startStreamTranslate(path, langStr, isBatch)

	case progressTickMsg:
		if m.batchProgress != nil {
			m.batchProgress.Mu.Lock()
			allDone := m.batchProgress.AllDone
			bpErr := m.batchProgress.Err
			bpResults := m.batchProgress.Results
			m.batchProgress.Mu.Unlock()
			if allDone {
				if bpErr != nil && len(bpResults) == 0 {
					return m, m.showErrorCmd(bpErr, m.state)
				}
				m.transResults = bpResults
				m.state = stBatchResult
				return m, nil
			}
		} else if m.transProgress != nil {
			m.transProgress.Mu.Lock()
			allDone := m.transProgress.AllDone
			pdErr := m.transProgress.Err
			pdResult := m.transProgress.Result
			m.transProgress.Mu.Unlock()
			if allDone {
				if pdResult != nil {
					m.transResults = []*engine.Result{pdResult}
					m.state = stTransResult
				} else if pdErr != nil {
					return m, m.showErrorCmd(pdErr, m.state)
				} else {
					return m, m.showErrorCmd(fmt.Errorf("未知错误：未返回结果"), m.state)
				}
				m.transProgress = nil
				return m, nil
			}
		}
		return m, m.transTick()

	case errMsg:
		m.err = msg.err
		if msg.prevState != 0 {
			m.prevState = msg.prevState
		} else {
			m.prevState = m.state
		}
		m.state = stError
		return m, nil

	case testModelDoneMsg:
		m.testModelErr = msg.err
		m.testModelLatency = msg.latency
		m.state = stTestModelResult
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) padForScreen(view string) string {
	if m.height <= 0 || view == "" {
		return view
	}
	lines := strings.Split(view, "\n")
	if len(lines) >= m.height {
		return view
	}
	lastIdx := len(lines) - 1
	for lastIdx >= 0 && strings.TrimSpace(lines[lastIdx]) == "" {
		lastIdx--
	}
	if lastIdx < 0 {
		return view
	}
	helpLine := lines[lastIdx]
	content := lines[:lastIdx]
	for len(content) < m.height-1 {
		content = append(content, "")
	}
	return strings.Join(content, "\n") + "\n" + helpLine
}

func (m *Model) inputView() string {
	var (
		title  string
		prompt string
		input  *textinput.Model
		info   string
	)

	switch m.state {
	case stAddName:
		title = "添加提供商 - 步骤 1/3"
		prompt = "提供商名称:"
		input = &m.nameInput
	case stAddURL:
		title = "添加提供商 - 步骤 2/3"
		prompt = "API地址:"
		input = &m.urlInput
	case stAddKey:
		title = "添加提供商 - 步骤 3/3"
		prompt = "API密钥:"
		input = &m.keyInput
	case stEditURL:
		title = "修改API地址"
		prompt = "新的API地址:"
		input = &m.urlInput
	case stEditKey:
		title = "修改API密钥"
		prompt = "新的API密钥:"
		input = &m.keyInput

	case stTransPath:
		title = "翻译单个Mod"
		prompt = "Mod jar文件路径:"
		input = &m.pathInput
	case stTransLang:
		title = "翻译单个Mod"
		prompt = "目标语言 (如 简体中文, zh_cn):"
		input = &m.langInput
	case stBatchPath:
		title = "批量翻译Mods目录"
		prompt = "Mods目录路径:"
		input = &m.pathInput
	case stBatchLang:
		title = "批量翻译Mods目录"
		prompt = "目标语言 (如 简体中文, zh_cn):"
		input = &m.langInput

	case stTransPackName:
		title = "翻译单个Mod"
		prompt = "资源包名称:"
		input = &m.packNameInput
	case stBatchPackName:
		title = "批量翻译Mods目录"
		prompt = "资源包名称:"
		input = &m.packNameInput

	case stModelListURL:
		title = "设置模型列表地址"
		prompt = "自动获取模型列表失败，请手动输入API地址"
		info = "例如: https://api.deepseek.com/models"
		input = &m.urlInput
	case stModelListURLEdit:
		title = "修改模型列表地址"
		prompt = "自动获取模型列表失败，请手动输入API地址"
		info = "例如: https://api.deepseek.com/models"
		input = &m.urlInput

	default:
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(" " + title + " "))
	b.WriteString("\n\n")
	b.WriteString(padLine(labelStyle.Render(prompt)) + "\n")
	if info != "" {
		b.WriteString(padLine(dimStyle.Render(info)) + "\n\n")
	}
	b.WriteString(padLine(input.View()) + "\n")
	b.WriteString(padLine(dimStyle.Render(strings.Repeat("─", input.Width))) + "\n\n")

	help := "Enter: 确认  •  Esc: 返回"
	switch m.state {
	case stAddName, stAddURL, stAddKey, stEditURL, stEditKey, stModelListURL, stModelListURLEdit:
		help = "Enter: 确认  •  Esc: 取消"
	}
	b.WriteString(padLine(helpStyle.Render(help)))
	return b.String()
}

func (m *Model) View() string {
	if m.quitting {
		return ""
	}

	var view string
	switch m.state {
	case stMenu:
		view = m.menuView()
	case stProviderList:
		view = m.providerListView()
	case stProviderDetail:
		view = m.providerDetailView()
	case stAddName, stAddURL, stAddKey, stEditURL, stEditKey:
		view = m.inputView()
	case stAddModels:
		view = m.addModelsView()
	case stModelToggle:
		view = m.modelToggleView()
	case stConfirmRemove:
		view = m.confirmRemoveView()
	case stSyncing:
		view = m.loadingView("正在同步模型列表...")
	case stLoadingModels:
		view = m.loadingView("正在从API获取模型列表...")
	case stModelLoading:
		view = m.loadingView("正在获取云端模型列表...")
	case stModelListURL, stModelListURLEdit:
		view = m.inputView()
	case stGlobalSelectModel:
		view = m.globalSelectModelView()
	case stTestModel:
		view = m.loadingView("正在测试模型连通性...")
	case stTestModelResult:
		view = m.testModelResultView()
	case stTransPath, stTransLang, stTransPackName, stBatchPath, stBatchLang, stBatchPackName:
		view = m.inputView()
	case stTransOutputMode, stBatchOutputMode:
		view = m.outputModeView()
	case stResolvingLang, stBatchResolving:
		view = m.loadingView("正在解析语言代码...")
	case stTransWait, stBatchWait:
		view = m.transWaitView()
	case stTransResult:
		view = m.transResultView()
	case stBatchResult:
		view = m.batchResultView()
	case stError:
		view = m.errorView()
	case stAbout:
		view = m.aboutView()
	case stSettings:
		view = m.settingsView()
	}
	return m.padForScreen(view)
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stMenu:
		return m.handleMenuKey(msg)

	case stProviderList:
		return m.handleProviderListKey(msg)
	case stProviderDetail:
		return m.handleProviderDetailKey(msg)
	case stAddName:
		return m.handleAddNameKey(msg)
	case stAddURL:
		return m.handleAddURLKey(msg)
	case stAddKey:
		return m.handleAddKeyKey(msg)
	case stAddModels:
		return m.handleAddModelsKey(msg)
	case stEditURL:
		return m.handleEditURLKey(msg)
	case stEditKey:
		return m.handleEditKeyKey(msg)
	case stModelToggle:
		return m.handleModelToggleKey(msg)
	case stConfirmRemove:
		return m.handleConfirmRemoveKey(msg)
	case stSyncing, stLoadingModels, stModelLoading, stResolvingLang, stBatchResolving, stTestModel:
		return m, nil
	case stTestModelResult:
		return m.handleTestModelResultKey(msg)
	case stModelListURL, stModelListURLEdit:
		return m.handleModelListURLKey(msg)
	case stGlobalSelectModel:
		return m.handleGlobalSelectModelKey(msg)

	case stTransPath:
		return m.handleTransPathKey(msg)
	case stTransLang:
		return m.handleTransLangKey(msg)
	case stTransOutputMode:
		return m.handleTransOutputModeKey(msg)
	case stTransPackName:
		return m.handleTransPackNameKey(msg)
	case stTransResult:
		return m.handleTransResultKey(msg)

	case stTransWait:
		// Allow exiting from progress view only when translation has finished
		if m.transProgress != nil {
			m.transProgress.Mu.Lock()
			done := m.transProgress.AllDone
			m.transProgress.Mu.Unlock()
			if done {
				switch msg.String() {
				case "enter", "esc", " ":
					m.state = stMenu
					m.transProgress = nil
					m.err = nil
					return m, nil
				}
			}
		}
		return m, nil

	case stBatchWait:
		// Allow exiting from progress view only when translation has finished
		if m.batchProgress != nil {
			m.batchProgress.Mu.Lock()
			done := m.batchProgress.AllDone
			m.batchProgress.Mu.Unlock()
			if done {
				switch msg.String() {
				case "enter", "esc", " ":
					m.state = stMenu
					m.batchProgress = nil
					m.err = nil
					return m, nil
				}
			}
		}
		return m, nil

	case stBatchPath:
		return m.handleBatchPathKey(msg)
	case stBatchLang:
		return m.handleBatchLangKey(msg)
	case stBatchOutputMode:
		return m.handleBatchOutputModeKey(msg)
	case stBatchPackName:
		return m.handleBatchPackNameKey(msg)
	case stBatchResult:
		return m.handleBatchResultKey(msg)

	case stError:
		return m.handleErrorKey(msg)
	case stAbout:
		return m.handleAboutKey(msg)
	case stSettings:
		return m.handleSettingsKey(msg)
	}
	return m, nil
}

// ── Error View ─────────────────────────────────────────────────

func (m *Model) errorView() string {
	var b strings.Builder
	b.WriteString(errTitleStyle.Render(" 错误 "))
	b.WriteString("\n\n")
	b.WriteString(padLine(errStyle.Render(m.err.Error())))
	b.WriteString("\n\n")
	b.WriteString(padLine(helpStyle.Render("按 Enter 或 Esc 返回")))
	return b.String()
}

func (m *Model) handleErrorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "esc":
		m.err = nil
		// Map loading/waiting states back to their input pages,
		// otherwise the user would get stuck on an infinite spinner.
		switch m.prevState {
		case stResolvingLang:
			m.state = stTransLang
		case stBatchResolving:
			m.state = stBatchLang
		case stTransWait:
			m.state = stTransLang
		case stBatchWait:
			m.state = stBatchLang
		default:
			m.state = m.prevState
		}
		m.prevState = 0
		return m, nil
	}
	return m, nil
}
