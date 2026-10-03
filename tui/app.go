// Package tui is lectern's interactive full-screen interface. A single root
// model owns the program for the whole session and swaps between screens.
package tui

import (
	"lectern/core"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type screenID int

const (
	scrMenu screenID = iota
	scrConnect
	scrClone
	scrStatus
	scrConfig
)

// navigateMsg asks the root model to switch screens.
type navigateMsg struct{ to screenID }

func goTo(id screenID) tea.Cmd {
	return func() tea.Msg { return navigateMsg{to: id} }
}

// screen is a tea.Model whose Update returns a screen, so the root can store
// the result without type assertions.
type screen interface {
	Init() tea.Cmd
	Update(tea.Msg) (screen, tea.Cmd)
	View() string
}

// helpProvider is implemented by screens that want key hints in the footer.
type helpProvider interface {
	ShortHelp() []key.Binding
}

// session is shared by every screen.
type session struct {
	svc           *core.Service
	username      string
	width, height int // space available for the screen body
}

type app struct {
	sess    *session
	current screen
	help    help.Model
}

// Run starts the interactive interface and blocks until the user quits.
func Run(svc *core.Service) error {
	_, err := tea.NewProgram(newApp(svc), tea.WithAltScreen()).Run()
	return err
}

func newApp(svc *core.Service) app {
	sess := &session{svc: svc, username: svc.Username()}
	return app{sess: sess, current: newMenuScreen(sess), help: help.New()}
}

func (a app) Init() tea.Cmd { return a.current.Init() }

func (a app) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

	case tea.WindowSizeMsg:
		a.sess.width = msg.Width
		a.help.Width = msg.Width
		a.sess.height = max(0, msg.Height-lipgloss.Height(a.header())-lipgloss.Height(a.footer()))
		// Screens get the body size, not the whole terminal.
		var cmd tea.Cmd
		a.current, cmd = a.current.Update(tea.WindowSizeMsg{Width: a.sess.width, Height: a.sess.height})
		return a, cmd
	case connectedMsg:
		a.sess.username = msg.username
	case disconnectedMsg:
		a.sess.username = ""
	case navigateMsg:
		a.current = a.newScreen(msg.to)
		return a, a.current.Init()
	}

	var cmd tea.Cmd
	a.current, cmd = a.current.Update(msg)
	return a, cmd
}

// newScreen builds a fresh screen on every visit, so state resets and Init
// runs again. Screens size themselves from the session in their constructor.
func (a app) newScreen(id screenID) screen {
	switch id {
	case scrConnect:
		return newConnectScreen(a.sess)
	case scrClone:
		return newCloneScreen(a.sess)
	case scrStatus:
		return newStatusScreen(a.sess)
	case scrConfig:
		return newConfigScreen(a.sess)
	default:
		return newMenuScreen(a.sess)
	}
}

func (a app) View() string {
	body := lipgloss.NewStyle().Height(a.sess.height).Render(a.current.View())
	return lipgloss.JoinVertical(lipgloss.Left, a.header(), body, a.footer())
}

// header and footer get real styling in the theme step.
func (a app) header() string {
	status := "not connected"
	if name := a.sess.username; name != "" {
		status = "connected · " + name
	}
	return "lectern_   " + status + "\n"
}

func (a app) footer() string {
	if hp, ok := a.current.(helpProvider); ok {
		return a.help.ShortHelpView(hp.ShortHelp())
	}
	return ""
}
