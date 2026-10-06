package tui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type menuItem struct {
	title, desc string
	to          screenID
	needsLogin  bool
	quit        bool
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type menuScreen struct {
	sess *session
	list list.Model
}

func newMenuScreen(s *session) *menuScreen {
	connect := menuItem{title: "Connect", desc: "Log in to your Moodle site", to: scrConnect}

	if name := s.svc.Username(); name != "" {
		connect = menuItem{title: "Reconnect", desc: "Signed in as " + name, to: scrConnect}
	}

	items := []list.Item{
		connect,
		menuItem{title: "Clone", desc: "Download all your courses", to: scrClone, needsLogin: true},
		menuItem{title: "Courses", desc: "Manage your courses", to: scrCourses},
		menuItem{title: "Sync", desc: "Check new updates in coureses and sync them", to: scrSync, needsLogin: true},
		menuItem{title: "Config", desc: "Show or clear your saved login", to: scrConfig},
		menuItem{title: "Quit", desc: "Exit lectern", quit: true},
	}

	// The list's default colors are purple; switch them to the theme.
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(colorOrange).BorderForeground(colorOrange)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(colorMuted).BorderForeground(colorOrange)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(colorText)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(colorMuted)

	l := list.New(items, delegate, s.width, s.height)
	l.Title = "What would you like to do?"
	l.Styles.Title = l.Styles.Title.UnsetBackground().Foreground(colorText).Bold(true)
	l.SetShowStatusBar(false) // hides "5 items"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	return &menuScreen{sess: s, list: l}
}

func (m *menuScreen) Init() tea.Cmd { return nil }

func (m *menuScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			return m, tea.Quit
		case "enter":
			it, ok := m.list.SelectedItem().(menuItem)
			if !ok {
				return m, nil
			}
			if it.quit {
				return m, tea.Quit
			}
			if it.needsLogin && !m.sess.svc.IsConnected() {
				return m, goTo(scrConnect)
			}
			return m, goTo(it.to)
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *menuScreen) View() string { return m.list.View() }

func (m *menuScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}
