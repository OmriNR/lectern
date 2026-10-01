package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type placeholderScreen struct{ name string }

func newPlaceholderScreen(name string) *placeholderScreen { return &placeholderScreen{name: name} }

func (p *placeholderScreen) Init() tea.Cmd { return nil }

func (p *placeholderScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok && k.String() == "esc" {
		return p, goTo(scrMenu)
	}
	return p, nil
}

func (p *placeholderScreen) View() string { return p.name + " - coming soon" }

func (p *placeholderScreen) ShortHelp() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))}
}
