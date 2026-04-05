package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.com/ajistrying/nbcode2/internal/agent"
	"github.com/ajistrying/nbcode2/internal/git"
	"github.com/ajistrying/nbcode2/internal/provider"
)

// ─── Theme ──────────────────────────────────────────────────────────────────

var theme = struct {
	bg        lipgloss.Color
	fg        lipgloss.Color
	muted     lipgloss.Color
	accent    lipgloss.Color
	border    lipgloss.Color
	user      lipgloss.Color
	err       lipgloss.Color
	toolCall  lipgloss.Color
	statusBg  lipgloss.Color
	statusFg  lipgloss.Color
}{
	bg:       lipgloss.Color("235"),
	fg:       lipgloss.Color("252"),
	muted:    lipgloss.Color("241"),
	accent:   lipgloss.Color("75"),  // soft blue
	border:   lipgloss.Color("238"),
	user:     lipgloss.Color("75"),
	err:      lipgloss.Color("203"), // soft red
	toolCall: lipgloss.Color("243"),
	statusBg: lipgloss.Color("236"),
	statusFg: lipgloss.Color("252"),
}

// ─── Styles ─────────────────────────────────────────────────────────────────

var (
	chatBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.border)

	inputBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(theme.border)

	userLabelStyle = lipgloss.NewStyle().
			Foreground(theme.user).
			Bold(true)

	toolCallStyle = lipgloss.NewStyle().
			Foreground(theme.toolCall).
			Italic(true)

	confirmStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")).
			Bold(true)

	welcomeTitleStyle = lipgloss.NewStyle().
				Foreground(theme.accent).
				Bold(true)

	welcomeMutedStyle = lipgloss.NewStyle().
				Foreground(theme.muted)
)

// ─── Messages ───────────────────────────────────────────────────────────────

type agentResponseMsg struct {
	content string
	err     error
}

// StatusUpdateMsg updates the loading status text.
type StatusUpdateMsg string

// ToolCallMsg notifies the TUI that a tool is being invoked.
type ToolCallMsg struct {
	ToolName string
	Args     json.RawMessage
}

// ConfirmRequestMsg asks the user to approve a tool call.
type ConfirmRequestMsg struct {
	ToolName string
	Args     json.RawMessage
	Respond  chan bool
}

// ─── Model ──────────────────────────────────────────────────────────────────

// Model is the Bubble Tea model for the nbcode TUI.
type Model struct {
	agent    *agent.Agent
	provider provider.Provider
	viewport viewport.Model
	textarea textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer

	messages    []chatMessage
	status      string
	gitInfo     git.Info
	width       int
	height      int
	loading     bool
	confirming  bool
	confirmMsg  string
	confirmChan   chan bool
	ready         bool
	lastWrapWidth int
}

type chatMessage struct {
	role    string
	content string
}

// New creates the TUI model.
func New(a *agent.Agent, p provider.Provider, workDir string) Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.CharLimit = 0

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(theme.accent)

	return Model{
		agent:    a,
		provider: p,
		textarea: ta,
		spinner:  sp,
		gitInfo:  git.GetInfo(workDir),
		messages: make([]chatMessage, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		m.spinner.Tick,
	)
}

// ─── Update ─────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			// Alt+Enter inserts a newline — let textarea handle it
			if msg.Alt {
				m.textarea.InsertRune('\n')
				lines := strings.Count(m.textarea.Value(), "\n") + 1
				if lines > 6 {
					lines = 6
				}
				m.textarea.SetHeight(lines)
				m.recalcLayout()
				return m, nil
			}

			if m.confirming {
				input := strings.TrimSpace(m.textarea.Value())
				approved := strings.HasPrefix(strings.ToLower(input), "y")
				m.confirmChan <- approved
				m.confirming = false
				m.textarea.Reset()
				if !approved {
					m.messages = append(m.messages, chatMessage{
						role:    "system",
						content: "Tool execution denied.",
					})
				}
				m.viewport.SetContent(m.renderMessages())
				m.viewport.GotoBottom()
				return m, nil
			}

			if m.loading {
				return m, nil
			}

			input := strings.TrimSpace(m.textarea.Value())
			if input == "" {
				return m, nil
			}

			if input == "/quit" || input == "/exit" {
				return m, tea.Quit
			}

			m.textarea.Reset()
			m.textarea.SetHeight(1)
			m.messages = append(m.messages, chatMessage{
				role:    "user",
				content: input,
			})
			m.loading = true
			m.viewport.SetContent(m.renderMessages())
			m.viewport.GotoBottom()

			return m, m.sendMessage(input)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.recalcLayout()
		return m, nil

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

	case ToolCallMsg:
		m.messages = append(m.messages, chatMessage{
			role:    "tool",
			content: formatToolCall(msg.ToolName, msg.Args),
		})
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

	// Auto-grow textarea based on content (1–6 lines)
	if !m.loading || m.confirming {
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)

		lines := strings.Count(m.textarea.Value(), "\n") + 1
		if lines < 1 {
			lines = 1
		}
		if lines > 6 {
			lines = 6
		}
		if m.textarea.Height() != lines {
			m.textarea.SetHeight(lines)
			m.recalcLayout()
		}
	}

	return m, tea.Batch(cmds...)
}

// ─── View ───────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Status bar — full width, no border
	statusBar := m.renderStatusBar()

	cw := m.width - 2

	// Chat viewport with border
	chatContent := m.viewport.View()
	chat := chatBorderStyle.
		Width(cw).
		Height(m.viewport.Height).
		Render(chatContent)

	// Input area with border
	var inputContent string
	if m.confirming {
		inputContent = confirmStyle.Render(m.confirmMsg) + "\n" + m.textarea.View()
	} else if m.loading {
		inputContent = m.spinner.View() + " " + m.status
	} else {
		inputContent = m.textarea.View()
	}
	input := inputBorderStyle.
		Width(cw).
		Render(inputContent)

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, chat, input)
}

// ─── Layout ─────────────────────────────────────────────────────────────────

func (m *Model) contentWidth() int {
	// Border takes 1 char on each side
	w := m.width - 2
	if w < 10 {
		w = 10
	}
	return w
}

func (m *Model) recalcLayout() {
	if m.width == 0 || m.height == 0 {
		return
	}

	cw := m.contentWidth()

	statusHeight := 1
	// input border (2) + textarea height
	inputChrome := 2 // top+bottom border
	taHeight := m.textarea.Height()
	if m.loading {
		taHeight = 1
	}
	inputHeight := inputChrome + taHeight

	chatChrome := 2 // top+bottom border
	vpHeight := m.height - statusHeight - inputHeight - chatChrome
	if vpHeight < 3 {
		vpHeight = 3
	}

	if !m.ready {
		m.viewport = viewport.New(cw, vpHeight)
		m.ready = true
	} else {
		m.viewport.Width = cw
		m.viewport.Height = vpHeight
	}
	m.textarea.SetWidth(cw - 2) // a bit of inner padding

	// Only rebuild glamour renderer when width actually changes
	wrapWidth := cw - 4
	if m.renderer == nil || m.lastWrapWidth != wrapWidth {
		m.renderer, _ = glamour.NewTermRenderer(
			glamour.WithStylePath("dark"),
			glamour.WithWordWrap(wrapWidth),
		)
		m.lastWrapWidth = wrapWidth
	}

	m.viewport.SetContent(m.renderMessages())
}

// ─── Status Bar ─────────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	maxCtx := m.provider.MaxContextTokens()
	tokens := m.agent.TotalTokens()
	var ctxStr string
	if maxCtx > 0 {
		pct := float64(tokens) / float64(maxCtx) * 100
		ctxStr = fmt.Sprintf("ctx %.0f%%", pct)
	} else {
		ctxStr = fmt.Sprintf("ctx %d", tokens)
	}

	parts := []string{m.provider.Model(), ctxStr}
	if m.gitInfo.IsRepo {
		parts = append(parts, m.gitInfo.StatusCompact())
	}

	bar := " " + strings.Join(parts, "  │  ")
	return lipgloss.NewStyle().
		Background(theme.statusBg).
		Foreground(theme.statusFg).
		Width(m.width).
		MaxWidth(m.width).
		Render(bar)
}

// ─── Message Rendering ──────────────────────────────────────────────────────

func (m Model) renderMessages() string {
	if len(m.messages) == 0 {
		return m.renderWelcome()
	}

	var sb strings.Builder
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			sb.WriteString(userLabelStyle.Render("You") + "\n")
			sb.WriteString(msg.content + "\n\n")

		case "assistant":
			sb.WriteString(userLabelStyle.Render("nbcode") + "\n")
			rendered, err := m.renderer.Render(msg.content)
			if err != nil {
				sb.WriteString(msg.content + "\n\n")
			} else {
				sb.WriteString(rendered + "\n")
			}

		case "tool":
			sb.WriteString(toolCallStyle.Render(msg.content) + "\n")

		case "error":
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.err).Render(msg.content) + "\n\n")

		case "system":
			sb.WriteString(lipgloss.NewStyle().Foreground(theme.muted).Italic(true).Render(msg.content) + "\n\n")
		}
	}
	return sb.String()
}

func (m Model) renderWelcome() string {
	title := welcomeTitleStyle.Render("nbcode")
	hints := welcomeMutedStyle.Render(
		"enter send  •  alt+enter newline  •  ctrl+c quit",
	)

	block := lipgloss.JoinVertical(lipgloss.Center, "", title, "", hints, "")
	return lipgloss.Place(m.viewport.Width, m.viewport.Height, lipgloss.Center, lipgloss.Center, block)
}

// ─── Commands & Helpers ─────────────────────────────────────────────────────

func (m Model) sendMessage(input string) tea.Cmd {
	return func() tea.Msg {
		response, err := m.agent.Run(input)
		return agentResponseMsg{content: response, err: err}
	}
}

func formatToolCall(name string, args json.RawMessage) string {
	detail := formatArgs(args)
	return fmt.Sprintf("  ── %s %s", name, detail)
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
