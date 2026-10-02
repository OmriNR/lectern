package tui

import (
	"fmt"
	"lectern/core"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type configState int

const (
	stateNotLoggedIn configState = iota
	stateLoggedIn
	stateLoggingOut
	stateResult
)

// disconnectedMsg is also handled by app, which clears the session username.
type disconnectedMsg struct{}
type disconnectedFailedMsg struct{ err error }

func logOutCmd(svc *core.Service) tea.Cmd {
	return func() tea.Msg {
		if err := svc.ClearConfig(); err != nil {
			return disconnectedFailedMsg{err}
		}
		return disconnectedMsg{}
	}
}

var (
	configKeyYes  = key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "log out"))
	configKeyNo   = key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "back"))
	configKeyBack = key.NewBinding(key.WithKeys("enter"), key.WithHelp("any key", "back to menu"))
)

// Compile-time checks that configScreen fits where app uses it.
var (
	_ screen       = (*configScreen)(nil)
	_ helpProvider = (*configScreen)(nil)
)

type configScreen struct {
	state     configState
	sess      *session
	spinner   spinner.Model
	statusMsg string
}

func newConfigScreen(s *session) *configScreen {
	spinner := spinner.New(spinner.WithSpinner(spinner.Dot))
	spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	initialState := stateNotLoggedIn
	if s.username != "" {
		initialState = stateLoggedIn
	}

	m := &configScreen{
		sess:    s,
		state:   initialState,
		spinner: spinner,
	}

	return m
}

func (m *configScreen) Init() tea.Cmd {
	return nil
}

func (m *configScreen) View() string {
	switch m.state {
	case stateNotLoggedIn:
		return "\n You are not logged in.\n"
	case stateLoggedIn:
		return fmt.Sprintf(
			"\n Logged in as %s\n\n Do you want to log out? (y/n)\n",
			lipgloss.NewStyle().Bold(true).Render(m.sess.username),
		)
	case stateLoggingOut:
		return fmt.Sprintf("\n %s Logging out, please wait...\n", m.spinner.View())
	case stateResult:
		return fmt.Sprintf("\n %s\n", m.statusMsg)

	}

	return ""
}

func (m *configScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.state {
		case stateNotLoggedIn, stateResult:
			return m, goTo(scrMenu)
		case stateLoggedIn:
			switch msg.String() {
			case "y", "Y":
				m.state = stateLoggingOut
				return m, tea.Batch(
					m.spinner.Tick,
					logOutCmd(m.sess.svc),
				)
			case "n", "N", "esc":
				return m, goTo(scrMenu)
			}
		}
	case spinner.TickMsg:
		if m.state == stateLoggingOut {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	case disconnectedMsg:
		m.state = stateResult
		m.statusMsg = "✓ Logged out."
		return m, nil
	case disconnectedFailedMsg:
		m.state = stateResult
		m.statusMsg = fmt.Sprintf("✗ Logout failed: %v", msg.err)
		return m, nil
	}

	return m, nil
}

func (m *configScreen) ShortHelp() []key.Binding {
	switch m.state {
	case stateLoggedIn:
		return []key.Binding{configKeyYes, configKeyNo}
	case stateLoggingOut:
		return nil
	default:
		return []key.Binding{configKeyBack}
	}
}
