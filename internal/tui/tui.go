package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ajistrying/nbcode2/internal/agent"
	"github.com/ajistrying/nbcode2/internal/git"
)

// Styles
var (
	statusBarStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("252")).
			Padding(0, 1)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	assistantMsgStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)
)

// Messages for the Bubble Tea update loop

type agentResponseMsg struct {
	content string
	err     error
}

// StatusUpdateMsg updates the loading status text.
type StatusUpdateMsg string

// ConfirmRequestMsg asks the user to approve a tool call.
type ConfirmRequestMsg struct {
	ToolName string
	Args     json.RawMessage
	Respond  chan bool
}

// Model is the Bubble Tea model for the nbcode TUI.
type Model struct {
	agent    *agent.Agent
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model

	messages    []chatMessage
	status      string
	modelName   string
	gitInfo     git.Info
	width       int
	height      int
	loading     bool
	confirming  bool
	confirmMsg  string
	confirmChan chan bool
	ready       bool
}

type chatMessage struct {
	role    string
	content string
}

// New creates the TUI model.
func New(a *agent.Agent, modelName string, workDir string) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	return Model{
		agent:     a,
		textarea:  ta,
		spinner:   sp,
		modelName: modelName,
		gitInfo:   git.GetInfo(workDir),
		messages:  make([]chatMessage, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			if m.confirming {
				// Handle confirmation response
				input := strings.TrimSpace(m.textarea.Value())
				approved := strings.HasPrefix(strings.ToLower(input), "y")
				m.confirmChan <- approved
				m.confirming = false
				m.textarea.Reset()
				if approved {
					m.confirmMsg = ""
				} else {
					m.messages = append(m.messages, chatMessage{
						role:    "system",
						content: "Tool execution denied.",
					})
				}
				return m, nil
			}

			if m.loading {
				return m, nil // ignore input while loading
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			if input == "/quit" || input == "/exit" {
				return m, tea.Quit
			}

			m.textarea.Reset()
			m.messages = append(m.messages, chatMessage{
				role:    "user",
				content: input,
			})
			m.loading = true

			return m, m.sendMessage(input)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1 // status bar
		inputHeight := 5  // textarea + border
		vpHeight := m.height - headerHeight - inputHeight - 2

		if !m.ready {
			m.viewport = viewport.New(m.width, vpHeight)
			m.ready = true
		} else {
			m.viewport.Width = m.width
			m.viewport.Height = vpHeight
		}
		m.textarea.SetWidth(m.width - 2)
		m.viewport.SetContent(m.renderMessages())

	case agentResponseMsg:
		m.loading = false
		if msg.err != nil {
			m.messages = append(m.messages, chatMessage{
				role:    "error",
				content: fmt.Sprintf("Error: %s", msg.err),
			})
		} else {
			m.messages = append(m.messages, chatMessage{
				role:    "assistant",
				content: msg.content,
			})
		}
		m.viewport.SetContent(m.renderMessages())
		m.viewport.GotoBottom()
		return m, nil

	case StatusUpdateMsg:
		m.status = string(msg)
		return m, nil

	case ConfirmRequestMsg:
		m.confirming = true
		m.confirmChan = msg.Respond
		m.confirmMsg = fmt.Sprintf("Allow %s? %s [y/N] ", msg.ToolName, formatArgs(msg.Args))
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	// Update textarea
	if !m.loading || m.confirming {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	statusBar := m.renderStatusBar()
	chatView := m.viewport.View()

	inputView := m.textarea.View()
	if m.confirming {
		inputView = confirmStyle.Render(m.confirmMsg) + "\n" + m.textarea.View()
	} else if m.loading {
		inputView = m.spinner.View() + " " + m.status
	}

	return fmt.Sprintf("%s\n%s\n%s", statusBar, chatView, inputView)
}

func (m Model) renderStatusBar() string {
	tokens := fmt.Sprintf("tokens: %d", m.agent.TotalTokens())
	model := fmt.Sprintf("model: %s", m.modelName)

	parts := []string{model, tokens}
	if m.gitInfo.IsRepo {
		parts = append(parts, fmt.Sprintf("branch: %s", m.gitInfo.Branch))
		parts = append(parts, fmt.Sprintf("git: %s", m.gitInfo.Status))
	}

	bar := strings.Join(parts, "  |  ")
	return statusBarStyle.Width(m.width).Render(bar)
}

func (m Model) renderMessages() string {
	var sb strings.Builder
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			sb.WriteString(userMsgStyle.Render("You: ") + msg.content + "\n\n")
		case "assistant":
			sb.WriteString(assistantMsgStyle.Render(msg.content) + "\n\n")
		case "error":
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(msg.content) + "\n\n")
		case "system":
			sb.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render(msg.content) + "\n\n")
		}
	}
	return sb.String()
}

func (m Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		response, err := m.agent.Run(input)
		return agentResponseMsg{content: response, err: err}
	}
}

func formatArgs(args json.RawMessage) string {
	var m map[string]any
	if json.Unmarshal(args, &m) == nil {
		if cmd, ok := m["command"]; ok {
			return fmt.Sprintf("(%v)", cmd)
		}
		if path, ok := m["path"]; ok {
			return fmt.Sprintf("(%v)", path)
		}
	}
	s := string(args)
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}
