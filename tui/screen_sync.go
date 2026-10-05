package tui

import (
	"errors"
	"fmt"
	"lectern/core"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
type coursesAlreadySyncedMsg struct{}
type coursesSyncingFailedMsg struct{ err error }
type syncFinishedMsg struct{}

var (
	syncKeyMove   = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move"))
	syncKeyToggle = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select"))
	syncKeyAll    = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all"))
	syncKeyUpdate = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "update"))
	syncKeyMenu   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "menu"))
	syncKeyRetry  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "try again"))
	syncKeyBack   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
)

var (
	_ screen       = (*syncScreen)(nil)
	_ helpProvider = (*syncScreen)(nil)
)

type syncScreen struct {
	sess    *session
	state   syncState
	spinner spinner.Model

	courses  []core.Course
	selected []bool
	cursor   int
	updated  []core.Course
	err      error
}

func newSyncScreen(s *session) *syncScreen {
	return &syncScreen{
		sess:    s,
		state:   syncChecking,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

func checkSync() tea.Msg { return checkSyncMsg{} }

func (m *syncScreen) Init() tea.Cmd { return checkSync }

func checkSyncCmd(svc *core.Service) tea.Cmd {
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			return coursesSyncingFailedMsg{err}
		}

		report, err := svc.CheckSync(dir)

		if err != nil {
			return coursesSyncingFailedMsg{err}
		}

		var courses []core.Course
		courses = append(courses, report.NewCourses...)
		courses = append(courses, report.CoursesWithNewSections...)
		courses = append(courses, report.CoursesWithNewFiles...)

		if len(courses) == 0 {
			return coursesAlreadySyncedMsg{}
		}

		return coursesSyncingChoose{courses}
	}
}

func updateCoursesCmd(svc *core.Service, courses []core.Course) tea.Cmd {
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			return coursesSyncingFailedMsg{err}
		}

		if err := svc.UpdateCourses(dir, courses); err != nil {
			return coursesSyncingFailedMsg{err}
		}
		return syncFinishedMsg{}
	}
}

func (m *syncScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case checkSyncMsg:
		m.state = syncChecking
		m.courses, m.selected, m.updated = nil, nil, nil
		m.cursor, m.err = 0, nil
		return m, tea.Batch(checkSyncCmd(m.sess.svc), m.spinner.Tick)

	case coursesSyncingChoose:
		if m.state != syncChecking {
			return m, nil
		}

		m.courses = msg.courses
		m.selected = make([]bool, len(msg.courses))
		for i := range m.selected {
			m.selected[i] = true
		}
		m.state = syncNotifing
		return m, nil

	case coursesAlreadySyncedMsg:
		if m.state != syncChecking {
			return m, nil
		}
		m.state = syncAlreadySynced
		return m, nil

	case coursesSyncingFailedMsg:
		if m.state != syncChecking && m.state != syncUpdating {
			return m, nil
		}

		m.err = msg.err
		m.state = syncFailed
		return m, nil

	case syncFinishedMsg:
		if m.state != syncUpdating {
			return m, nil
		}
		m.state = syncDone
		return m, nil

	case spinner.TickMsg:
		if m.state != syncChecking && m.state != syncUpdating {
			return m, nil
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *syncScreen) handleKey(msg tea.KeyMsg) (screen, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case syncChecking:
		if key == "esc" {
			return m, goTo(scrMenu)
		}

	case syncNotifing:
		switch key {
		case "esc":
			return m, goTo(scrMenu)
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(m.courses)-1, m.cursor+1)
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			all := m.selectedCount() != len(m.courses)
			for i := range m.selected {
				m.selected[i] = all
			}
		case "enter":
			m.updated = m.chosen()
			if len(m.updated) == 0 {
				return m, nil
			}
			m.state = syncUpdating
			return m, tea.Batch(updateCoursesCmd(m.sess.svc, m.updated), m.spinner.Tick)
		}
	case syncUpdating:

	case syncDone:
		if key == "enter" || key == "esc" {
			return m, goTo(scrMenu)
		}

	case syncAlreadySynced:
		if key == "enter" || key == "esc" {
			return m, goTo(scrMenu)
		}

	case syncFailed:
		switch key {
		case "r":
			return m, checkSync
		case "enter", "esc":
			return m, goTo(scrMenu)
		}
	}

	return m, nil
}

func (m *syncScreen) chosen() []core.Course {
	var courses []core.Course
	for i, c := range m.courses {
		if m.selected[i] {
			courses = append(courses, c)
		}
	}
	return courses
}

func (m *syncScreen) selectedCount() int {
	n := 0
	for _, s := range m.selected {
		if s {
			n++
		}
	}
	return n
}

func (m *syncScreen) View() string {
	var body string

	switch m.state {
	case syncChecking:
		body = strings.TrimSpace(m.spinner.View()) + " Checking your courses on " + m.sess.svc.Host() + "..."

	case syncNotifing:
		body = titleStyle.Render(fmt.Sprintf("%s out of sync", coursesLabel(len(m.courses)))) + "\n\n" +
			m.renderChoices() + "\n\n" +
			mutedStyle.Render(fmt.Sprintf("%d selected · press enter to update.", m.selectedCount()))

	case syncUpdating:
		body = strings.TrimSpace(m.spinner.View()) + " Upddating " + coursesLabel(len(m.updated)) + "..." + "\n\n" +
			m.renderUpdated(accentStyle.Render(strings.TrimSpace(m.spinner.View())))

	case syncDone:
		body = accentStyle.Render("✓ ") + titleStyle.Render("Updated "+coursesLabel(len(m.updated))) + "\n\n" +
			m.renderUpdated(accentStyle.Render("✓"))

	case syncAlreadySynced:
		body = accentStyle.Render("✓ ") + titleStyle.Render("Everything is up to date") + "\n" +
			mutedStyle.Render("No new courses, sections or files on "+m.sess.svc.Host())

	case syncFailed:
		if errors.Is(m.err, core.ErrNoWorkspace) {
			dir, _ := os.Getwd()
			body = titleStyle.Render("Not a lectern workspace") + "\n\n" +
				mutedStyle.Render(tildify(dir)+" isn't inside a cloned folder.") + "\n" +
				"Run Clone to create one, or start lectern from inside a workspace."
		} else {
			errText := lipgloss.NewStyle().Width(max(10, m.sess.width-4)).Render(m.err.Error())
			body = accentStyle.Render("✗ ") + titleStyle.Render("sync failed") + "\n\n" + errText
		}
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *syncScreen) renderChoices() string {
	rows := max(1, m.sess.height-10)
	start, end := 0, len(m.courses)

	if len(m.courses) > rows {
		start = min(max(0, m.cursor-rows/2), len(m.courses)-rows)
		end += start + rows
	}

	nameWidth := max(10, min(40, m.sess.width-20))
	nameStyle := lipgloss.NewStyle().Width(nameWidth)

	var lines []string
	if start > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf(" ... %d more", start)))
	}

	for i := start; i < end; i++ {
		c := m.courses[i]

		pointer, style := " ", nameStyle.Foreground(colorText)
		if i == m.cursor {
			pointer, style = accentStyle.Render("> "), nameStyle.Foreground(colorOrange)
		}

		box := mutedStyle.Render("[ ]")
		if m.selected[i] {
			box = accentStyle.Render("[✓]")
		}

		lines = append(lines, pointer+box+" "+style.Render(truncate(c.DisplayName, nameWidth))+" "+mutedStyle.Render(c.ShortName))
	}

	if hidden := len(m.courses) - end; hidden > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... %d more", hidden)))
	}

	return strings.Join(lines, "\n")
}

func (m *syncScreen) renderUpdated(icon string) string {
	rows := max(1, m.sess.height-8)
	nameWidth := max(10, min(40, m.sess.width-20))

	var lines []string
	for i, c := range m.updated {
		if i == rows {
			lines = append(lines, icon+" "+truncate(c.DisplayName, nameWidth))
		}
	}

	return strings.Join(lines, "\n")
}

func coursesLabel(n int) string {
	if n == 1 {
		return "1 course"
	}
	return fmt.Sprintf("%d courses", n)
}

func (m *syncScreen) ShortHelp() []key.Binding {
	switch m.state {
	case syncChecking:
		return []key.Binding{syncKeyBack}
	case syncNotifing:
		return []key.Binding{syncKeyMove, syncKeyToggle, syncKeyAll, syncKeyUpdate, syncKeyBack}
	case syncDone, syncAlreadySynced:
		return []key.Binding{syncKeyMenu}
	case syncFailed:
		return []key.Binding{syncKeyRetry, syncKeyMenu}
	default: // updating: nothing to press
		return nil
	}
}
