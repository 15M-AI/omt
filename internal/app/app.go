package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type page int

const (
	homePage page = iota
	modelsPage
	chatPage
	addPage
	detailsPage
)

type chatMessage struct {
	role string
	text string
}

type modelsLoadedMsg struct {
	models []modelInfo
	err    error
}

type runtimeOutputMsg struct {
	title  string
	output string
	err    error
}

type generatedMsg struct {
	text string
	err  error
}

type serverStartedMsg struct {
	cmd     *apiServer
	err     error
	purpose string
}

type model struct {
	config      Config
	page        page
	cursor      int
	width       int
	height      int
	models      []modelInfo
	details     string
	activeModel string
	chat        []chatMessage
	chatInput   textarea.Model
	pathInput   textarea.Model
	viewport    viewport.Model
	busy        bool
	server      *apiServer
	status      string
	err         string
}

var (
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF"))
	subtitleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
	accentStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7CFFB2"))
	mutedStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#71717A"))
	selectedStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7CFFB2"))
	userStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7CFFB2"))
	assistantStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#A5B4FC"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FCA5A5"))
	panelStyle     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3F3F46")).Padding(0, 1)
)

func Run(config Config) error {
	input := textarea.New()
	input.Placeholder = "Message your model..."
	input.Prompt = "› "
	input.ShowLineNumbers = false
	input.SetHeight(3)
	input.CharLimit = 4096
	input.Focus()

	pathInput := textarea.New()
	pathInput.Placeholder = "/path/to/model.omf"
	pathInput.Prompt = "› "
	pathInput.ShowLineNumbers = false
	pathInput.SetHeight(3)
	pathInput.CharLimit = 4096
	pathInput.Focus()

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true
	m := model{
		config:    config,
		page:      homePage,
		chatInput: input,
		pathInput: pathInput,
		viewport:  vp,
		status:    "Local-first · OMF models · No cloud account required",
	}
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return loadModels()
}

func loadModels() tea.Cmd {
	return func() tea.Msg {
		models, err := installedModels()
		return modelsLoadedMsg{models: models, err: err}
	}
}

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.chatInput.SetWidth(max(20, m.width-12))
		m.pathInput.SetWidth(max(20, m.width-12))
		m.viewport.Width = max(20, m.width-8)
		m.viewport.Height = max(5, m.height-15)
	case modelsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.models = msg.models
		}
	case runtimeOutputMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.status = msg.output
			m.err = ""
			if msg.title == "build" {
				m.config.Runtime = msg.output
				m.status = "Runtime built: " + msg.output
			}
		}
		if msg.title == "add" {
			m.pathInput.Reset()
			return m, loadModels()
		}
		if msg.title == "show" && msg.err == nil {
			m.details = msg.output
			m.page = detailsPage
		}
	case generatedMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.chat = append(m.chat, chatMessage{role: "assistant", text: strings.TrimSpace(msg.text)})
			m.err = ""
			m.updateChatViewport()
			m.viewport.GotoBottom()
		}
	case serverStartedMsg:
		m.busy = false
		if msg.err != nil {
			m.err = msg.err.Error()
		} else {
			m.server = msg.cmd
			m.status = fmt.Sprintf("OMA server running at http://%s:%d", m.config.Host, m.config.Port)
			m.err = ""
			if msg.purpose == "chat" {
				m.page = chatPage
				m.chatInput.Focus()
				m.updateChatViewport()
			}
		}
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			_ = stopServer(m.server)
			return m, tea.Quit
		}
		if m.busy {
			return m, nil
		}
		return m.handleKey(msg)
	}

	var cmd tea.Cmd
	switch m.page {
	case chatPage:
		m.chatInput, cmd = m.chatInput.Update(message)
	case addPage:
		m.pathInput, cmd = m.pathInput.Update(message)
	case modelsPage:
		m.viewport, cmd = m.viewport.Update(message)
	}
	return m, cmd
}

func (m model) handleKey(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.page {
	case homePage:
		switch key.String() {
		case "up", "k":
			m.cursor = (m.cursor + 5) % 6
		case "down", "j":
			m.cursor = (m.cursor + 1) % 6
		case "enter":
			return m.activate(m.cursor)
		case "1":
			return m.activate(0)
		case "2":
			return m.activate(1)
		case "3":
			return m.activate(2)
		case "4":
			return m.activate(3)
		case "5":
			return m.activate(4)
		case "6", "q":
			_ = stopServer(m.server)
			return m, tea.Quit
		}
	case modelsPage:
		switch key.String() {
		case "esc", "q":
			m.page = homePage
		case "up", "k":
			if len(m.models) > 0 && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if len(m.models) > 0 && m.cursor < len(m.models)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.models) > 0 {
				m.activeModel = m.models[m.cursor].Name
				m.status = "Selected " + m.activeModel
				m.page = homePage
			}
		case "a":
			m.page = addPage
			m.pathInput.Focus()
		case "i":
			if len(m.models) > 0 {
				runtime, modelName := m.config.Runtime, m.models[m.cursor].Name
				m.busy = true
				return m, func() tea.Msg {
					output, err := runRuntime(runtime, "show", modelName)
					return runtimeOutputMsg{title: "show", output: output, err: err}
				}
			}
		}
	case detailsPage:
		if key.String() == "esc" || key.String() == "q" {
			m.page = modelsPage
		}
	case addPage:
		switch key.String() {
		case "esc":
			m.page = homePage
			m.pathInput.Reset()
		case "enter":
			path := strings.TrimSpace(m.pathInput.Value())
			if path == "" {
				m.err = "Enter the path to a local .omf file."
				return m, nil
			}
			m.busy = true
			m.err = ""
			runtime, modelPath := m.config.Runtime, path
			return m, func() tea.Msg {
				output, err := runRuntime(runtime, "add", modelPath)
				return runtimeOutputMsg{title: "add", output: output, err: err}
			}
		default:
			var cmd tea.Cmd
			m.pathInput, cmd = m.pathInput.Update(key)
			return m, cmd
		}
	case chatPage:
		switch key.String() {
		case "esc":
			m.page = homePage
		case "enter":
			prompt := strings.TrimSpace(m.chatInput.Value())
			if prompt == "" {
				return m, nil
			}
			m.chatInput.Reset()
			m.chat = append(m.chat, chatMessage{role: "user", text: prompt})
			m.busy = true
			m.err = ""
			m.updateChatViewport()
			m.viewport.GotoBottom()
			fullPrompt := m.buildPrompt()
			modelName := m.activeModel
			host, port := m.config.Host, m.config.Port
			return m, func() tea.Msg {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				text, err := generate(ctx, host, port, modelName, fullPrompt, 128)
				return generatedMsg{text: text, err: err}
			}
		case "ctrl+l":
			m.chat = nil
			m.updateChatViewport()
		default:
			var cmd tea.Cmd
			m.chatInput, cmd = m.chatInput.Update(key)
			return m, cmd
		}
	}
	return m, nil
}

func (m model) activate(index int) (tea.Model, tea.Cmd) {
	switch index {
	case 0:
		m.page = modelsPage
		m.cursor = 0
		return m, loadModels()
	case 1:
		m.page = modelsPage
		m.cursor = 0
		return m, loadModels()
	case 2:
		m.page = addPage
		m.pathInput.Focus()
	case 3:
		if m.activeModel == "" {
			m.page = modelsPage
			m.cursor = 0
			m.status = "Choose a model to start a chat."
			return m, loadModels()
		}
		if m.server != nil && m.server.running() {
			m.page = chatPage
			m.chatInput.Focus()
			m.updateChatViewport()
			return m, nil
		}
		m.busy = true
		runtime, modelName, host, port := m.config.Runtime, m.activeModel, m.config.Host, m.config.Port
		return m, func() tea.Msg {
			cmd, err := startServer(runtime, modelName, host, port)
			return serverStartedMsg{cmd: cmd, err: err, purpose: "chat"}
		}
	case 4:
		if m.server != nil && m.server.running() {
			if err := stopServer(m.server); err != nil {
				m.err = err.Error()
				return m, nil
			}
			m.server = nil
			m.status = "OMA server stopped."
			return m, nil
		}
		if m.activeModel == "" {
			m.status = "Select a model before starting the API server."
			m.page = modelsPage
			return m, loadModels()
		}
		m.busy = true
		runtime, modelName, host, port := m.config.Runtime, m.activeModel, m.config.Host, m.config.Port
		return m, func() tea.Msg {
			cmd, err := startServer(runtime, modelName, host, port)
			if err != nil {
				return serverStartedMsg{err: err}
			}
			return serverStartedMsg{cmd: cmd, purpose: "api"}
		}
	case 5:
		m.busy = true
		source := m.config.Source
		return m, func() tea.Msg {
			runtimePath, err := buildRuntime(source)
			return runtimeOutputMsg{title: "build", output: runtimePath, err: err}
		}
	}
	return m, nil
}

func (m *model) updateChatViewport() {
	var conversation strings.Builder
	for _, message := range m.chat {
		style := userStyle
		label := "You"
		if message.role == "assistant" {
			style = assistantStyle
			label = "Open Mind"
		}
		conversation.WriteString(style.Render(label))
		conversation.WriteByte('\n')
		conversation.WriteString(strings.TrimSpace(message.text))
		conversation.WriteString("\n\n")
	}
	if m.busy {
		conversation.WriteString(assistantStyle.Render("Open Mind"))
		conversation.WriteString("\nThinking...\n\n")
	}
	m.viewport.SetContent(conversation.String())
}

func (m model) buildPrompt() string {
	var prompt strings.Builder
	for _, message := range m.chat {
		if message.role == "user" {
			prompt.WriteString("User: ")
		} else {
			prompt.WriteString("Assistant: ")
		}
		prompt.WriteString(message.text)
		prompt.WriteByte('\n')
	}
	prompt.WriteString("Assistant:")
	return prompt.String()
}

func (m model) View() string {
	if m.width == 0 {
		return "\n  Starting Open Mind...\n"
	}
	if m.width < 64 || m.height < 18 {
		return "Please resize the terminal to at least 64 columns by 18 rows.\n"
	}
	var body string
	switch m.page {
	case modelsPage:
		body = m.modelsView()
	case addPage:
		body = m.addView()
	case chatPage:
		body = m.chatView()
	case detailsPage:
		body = m.detailsView()
	default:
		body = m.homeView()
	}
	header := m.headerView()
	footer := m.footerView()
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m model) headerView() string {
	brand := titleStyle.Render("◉  OPEN MIND")
	label := subtitleStyle.Render("LOCAL MODEL RUNNER")
	modelLabel := "No model selected"
	if m.activeModel != "" {
		modelLabel = accentStyle.Render(m.activeModel)
	}
	line := lipgloss.JoinHorizontal(lipgloss.Center, brand, "   ", label)
	right := mutedStyle.Render("MODEL  ") + modelLabel
	gap := max(1, m.width-lipgloss.Width(line)-lipgloss.Width(right)-6)
	return lipgloss.NewStyle().Width(m.width-4).Padding(1, 2).Render(line + strings.Repeat(" ", gap) + right)
}

func (m model) homeView() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFFFF")).Render("Run models. Keep everything local.")
	subtitle := subtitleStyle.Render("A simple terminal home for your Open Mind models.")
	items := []string{
		"Choose a model",
		"Browse models",
		"Add a model",
		"Chat",
		"Start / stop API server",
		"Build runtime",
	}
	var menu strings.Builder
	menu.WriteString("\n")
	menu.WriteString(title)
	menu.WriteString("\n")
	menu.WriteString(subtitle)
	menu.WriteString("\n\n")
	for i, item := range items {
		key := fmt.Sprintf("%d", i+1)
		label := fmt.Sprintf("  %s   %s", mutedStyle.Render(key), item)
		if i == m.cursor {
			label = selectedStyle.Render("› " + key + "   " + item)
		}
		if i == 3 && m.activeModel == "" {
			label += mutedStyle.Render("  (choose a model first)")
		}
		if i == 4 && m.server != nil && m.server.running() {
			label += accentStyle.Render("  RUNNING")
		}
		menu.WriteString(label + "\n")
	}
	if len(m.models) == 0 {
		menu.WriteString("\n")
		menu.WriteString(mutedStyle.Render("No models installed. Choose “Add a model” to get started."))
	} else {
		menu.WriteString("\n")
		menu.WriteString(mutedStyle.Render(fmt.Sprintf("%d local model(s) installed", len(m.models))))
	}
	return lipgloss.NewStyle().Padding(1, 4).Render(menu.String())
}

func (m model) modelsView() string {
	var content strings.Builder
	content.WriteString(titleStyle.Render("Your models"))
	content.WriteString("\n")
	content.WriteString(subtitleStyle.Render("Enter selects · I inspects · A installs · Esc returns"))
	content.WriteString("\n\n")
	if len(m.models) == 0 {
		content.WriteString(mutedStyle.Render("No .omf files found in your local model library."))
		content.WriteString("\n\n")
		content.WriteString("Press A to add a model, or Esc to go back.")
	} else {
		for i, item := range m.models {
			prefix := "  "
			nameStyle := lipgloss.NewStyle()
			if i == m.cursor {
				prefix = "› "
				nameStyle = selectedStyle
			}
			marker := ""
			if item.Name == m.activeModel {
				marker = accentStyle.Render("  ACTIVE")
			}
			name := nameStyle.Render(item.Name)
			content.WriteString(fmt.Sprintf("%s%-40s %8s%s\n", prefix, name, item.Size, marker))
		}
	}
	return lipgloss.NewStyle().Padding(1, 4).Render(content.String())
}

func (m model) addView() string {
	content := titleStyle.Render("Add a local model")
	content += "\n" + subtitleStyle.Render("Enter the path to an OMF file. The runtime validates it before installation.")
	content += "\n\n" + panelStyle.Width(max(32, m.width-14)).Render(m.pathInput.View())
	if m.busy {
		content += "\n\n" + accentStyle.Render("Installing model...")
	}
	return lipgloss.NewStyle().Padding(1, 4).Render(content)
}

func (m model) detailsView() string {
	content := titleStyle.Render("Model details")
	content += "\n\n" + m.details
	content += "\n\n" + mutedStyle.Render("Press Esc to return to your model library.")
	return lipgloss.NewStyle().Padding(1, 4).Render(content)
}

func (m model) chatView() string {
	header := titleStyle.Render("Chat") + "   " + accentStyle.Render(m.activeModel) + "\n"
	header += subtitleStyle.Render("Enter sends · Esc returns home · Ctrl+L clears conversation")
	chat := m.viewport.View()
	input := panelStyle.Width(max(32, m.width-12)).Render(m.chatInput.View())
	return lipgloss.NewStyle().Padding(0, 3).Render(lipgloss.JoinVertical(lipgloss.Left, header, "", chat, "", input))
}

func (m model) footerView() string {
	status := m.status
	if m.busy {
		status = "Working..."
	}
	if m.err != "" {
		status = errorStyle.Render("Error: " + m.err)
	}
	bar := mutedStyle.Render("↑/↓ navigate  ·  Enter select  ·  Q quit")
	return lipgloss.NewStyle().Width(m.width).Padding(0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, lipgloss.NewStyle().Foreground(lipgloss.Color("#3F3F46")).Render(strings.Repeat("─", max(1, m.width-4))),
			lipgloss.JoinHorizontal(lipgloss.Left, status, strings.Repeat(" ", max(1, m.width-lipgloss.Width(status)-lipgloss.Width(bar)-6)), bar)))
}
