package tui

import (
	"errors"
	"lectern/core"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type connectState int

const (
	connEditing connectState = iota
	connSubmiting
	connDone
)

const (
	fieldUsername = iota
	fieldPassword
)

type connectedMsg struct{ username string }

type connectFailedMsg struct{ err error }

type connectDoneMsg struct{}

func connectCmd(svc *core.Service, username, password string) tea.Cmd {
	return func() tea.Msg {
		if err := svc.Connect(username, password); err != nil {
			return connectFailedMsg{err}
		}
		return connectedMsg{username: svc.Username()}
	}
}

var (
	connectKeyNext   = key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next field"))
	connectKeySubmit = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "connect"))
	connectKeyBack   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
)

type connectScreen struct {
	sess    *session
	inputs  []textinput.Model
	focus   int
	spinner spinner.Model
	state   connectState
	err     error
}

func newConnectScreen(s *session) *connectScreen {
	user := textinput.New()
	user.Prompt = "┃ "
	user.Placeholder = "username"
	user.CharLimit = 100
	user.SetValue(s.svc.Username())

	pass := textinput.New()
	pass.Prompt = "┃ "
	pass.Placeholder = "password"
	pass.CharLimit = 200
	pass.EchoMode = textinput.EchoPassword
	pass.EchoCharacter = '•'

	m := &connectScreen{
		sess:    s,
		inputs:  []textinput.Model{user, pass},
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
	m.setWidth(s.width)

	if s.username != "" {
		m.focus = fieldPassword
	}

	return m
}

func (m *connectScreen) setWidth(w int) {
	width := max(10, min(w-6, 40))
	for i := range m.inputs {
		m.inputs[i].Width = width
	}
}

func (m *connectScreen) Init() tea.Cmd {
	return m.focusInput(m.focus)
}

func (m *connectScreen) focusInput(i int) tea.Cmd {
	m.focus = i
	for j := range m.inputs {
		if j != i {
			m.inputs[j].Blur()
		}
	}

	return m.inputs[i].Focus()
}

func (m *connectScreen) submit() tea.Cmd {
	username := strings.TrimSpace(m.inputs[fieldUsername].Value())
	password := m.inputs[fieldPassword].Value()

	switch {
	case username == "":
		m.err = errors.New("enter your username")
		return m.focusInput(fieldUsername)

	case password == "":
		m.err = errors.New("enter your password")
		return m.focusInput(fieldPassword)
	}

	m.state = connSubmiting
	m.err = nil

	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	return tea.Batch(connectCmd(m.sess.svc, username, password), m.spinner.Tick)
}

func (m *connectScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setWidth(msg.Width)
		return m, nil

	case connectedMsg:
		m.state = connDone
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg { return connectDoneMsg{} })

	case connectDoneMsg:
		return m, goTo(scrMenu)

	case connectFailedMsg:
		m.state = connEditing
		m.err = msg.err
		m.inputs[fieldPassword].Reset()
		return m, m.focusInput(fieldPassword)

	case spinner.TickMsg:
		if m.state != connSubmiting {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		if msg.String() == "esc" {
			return m, goTo(scrMenu)
		}

		if m.state != connEditing {
			return m, nil
		}

		switch msg.String() {
		case "tab", "down":
			return m, m.focusInput((m.focus + 1) % len(m.inputs))
		case "shift+tab", "up":
			return m, m.focusInput((m.focus + len(m.inputs) - 1) % len(m.inputs))
		case "enter":
			if m.focus == fieldUsername {
				return m, m.focusInput(fieldPassword)
			}
			return m, m.submit()
		}
	}

	var cmd tea.Cmd
	m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
	return m, cmd
}

func (m *connectScreen) View() string {
	var b strings.Builder

	b.WriteString("Connect to Moodel\n")
	b.WriteString(m.sess.svc.Host() + "\n\n")
	b.WriteString("Username\n" + m.inputs[fieldUsername].View() + "\n")
	b.WriteString("Password\n" + m.inputs[fieldPassword].View() + "\n\n")

	switch {
	case m.state == connSubmiting:
		b.WriteString(m.spinner.View() + "Connecting...")
	case m.state == connDone:
		b.WriteString("✓ connected as " + m.sess.username)
	case m.err != nil:
		b.WriteString("✗ " + m.err.Error())
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(b.String())
}

func (m *connectScreen) ShortHelp() []key.Binding {
	if m.state != connEditing {
		return []key.Binding{connectKeyBack}
	}

	return []key.Binding{connectKeyNext, connectKeySubmit, connectKeyBack}
}
