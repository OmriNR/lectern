package tui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type menuItem struct {
	title, desc    string
	to             screenID
	needsLogin     bool
	needsWorkspace bool
	quit           bool
}

func (i menuItem) Title() string       { return i.title }
func (i menuItem) Description() string { return i.desc }
func (i menuItem) FilterValue() string { return i.title }

type menuScreen struct {
	sess *session
	list list.Model

	cwd         string
	root        string // workspace root; empty when not inside one
	inWorkspace bool
}

func newMenuScreen(s *session) *menuScreen {
	m := &menuScreen{sess: s}
	m.cwd, _ = os.Getwd()
	if root, err := s.svc.WorkspaceRoot(m.cwd); err == nil {
		m.root, m.inWorkspace = root, true
	}

	connect := menuItem{title: "Connect", desc: "Log in to your Moodle site", to: scrConnect}

	if name := s.svc.Username(); name != "" {
		connect = menuItem{title: "Reconnect", desc: "Signed in as " + name, to: scrConnect}
	}

	items := []list.Item{
		connect,
		menuItem{title: "Clone", desc: "Download all your courses", to: scrClone, needsLogin: true},
		menuItem{title: "Courses", desc: "Manage your courses", to: scrCourses, needsLogin: true, needsWorkspace: true},
		menuItem{title: "Assignments", desc: "Manage your assignments", to: scrAssignments, needsLogin: true, needsWorkspace: true},
		menuItem{title: "Sync", desc: "Check new updates in courses and sync them", to: scrSync, needsLogin: true, needsWorkspace: true},
		menuItem{title: "Config", desc: "Show or clear your saved login", to: scrConfig},
		menuItem{title: "Quit", desc: "Exit lectern", quit: true},
	}

	if !m.inWorkspace {
		for i, item := range items {
			if it, ok := item.(menuItem); ok && it.needsWorkspace {
				it.desc += " · needs a courses folder"
				items[i] = it
			}
		}
	}

	// The list's default colors are purple; switch them to the theme.
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(colorOrange).BorderForeground(colorOrange)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(colorMuted).BorderForeground(colorOrange)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(colorText)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(colorMuted)

	l := list.New(items, delegate, s.width, m.listHeight(s.height))
	l.Title = "What would you like to do?"
	l.Styles.Title = l.Styles.Title.UnsetBackground().Foreground(colorText).Bold(true)
	l.SetShowStatusBar(false) // hides "5 items"
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	m.list = l
	return m
}

// notice tells the user whether lectern was started inside a courses folder.
func (m *menuScreen) notice() string {
	if m.inWorkspace {
		return "  " + mutedStyle.Render("Courses folder: "+tildify(m.root))
	}

	return "  " + accentStyle.Render("! ") + "You're not in a courses folder (" + tildify(m.cwd) + ").\n" +
		"    " + mutedStyle.Render("cd into the folder you cloned, or choose Clone to create one.")
}

// listHeight leaves room under the list for the notice and a blank line.
func (m *menuScreen) listHeight(h int) int {
	lines := strings.Count(m.notice(), "\n") + 1
	return max(1, h-lines-1)
}

func (m *menuScreen) Init() tea.Cmd { return nil }

func (m *menuScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, m.listHeight(msg.Height))
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

func (m *menuScreen) View() string {
	return m.list.View() + "\n\n" + m.notice()
}

func (m *menuScreen) ShortHelp() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
	}
}
