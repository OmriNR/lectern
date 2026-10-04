package tui

import (
	"lectern/core"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type syncState int

const (
	syncChecking syncState = iota
	syncNotifing
	syncUpdating
	syncFailed
	syncDone
	syncAlreadySynced
)

type checkSyncMsg struct{}
type coursesSyncingChoose struct{ courses []core.Course }
type couresesAlreadySyncedMSg struct{}
type coursesSyncingFailedMsg struct{ err error }
type syncFinishedMsg struct{}

type syncScreen struct{ check string }

func newSyncScreen(check string) *syncScreen { return &syncScreen{check: check} }

func (m *syncScreen) Init() tea.Cmd { return nil }

func (p *syncScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		return p, goTo(scrMenu)
	}
	return p, nil
}

func (p *syncScreen) View() string { return p.check + " - coming soon" }

func (p *syncScreen) ShortHelp() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))}
}
