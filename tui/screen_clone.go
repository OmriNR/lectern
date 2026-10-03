package tui

import (
	"fmt"
	"lectern/core"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type cloneState int

const (
	cloneChoosingDir cloneState = iota
	cloneFetching
	cloneConfirm
	cloneCloning
	cloneDone
	cloneFailed
)

type coursesFetchedMsg struct{ courses []core.Course }
type fetchFailedMsg struct{ err error }
type cloneEventMsg core.CloneEvent
type cloneFinishedMsg struct{ err error }

var (
	cloneKeyNext   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "next"))
	cloneKeyStart  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "start"))
	cloneKeyMenu   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "menu"))
	cloneKeyRetry  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "try again"))
	cloneKeyBack   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	cloneKeyFolder = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "change folder"))
)

var (
	_ screen       = (*cloneScreen)(nil)
	_ helpProvider = (*cloneScreen)(nil)
)

type cloneCourse struct {
	name        string
	done, total int
}

type cloneScreen struct {
	sess    *session
	state   cloneState
	input   textinput.Model
	spinner spinner.Model
	bar     progress.Model

	dir     string
	courses []core.Course
	list    []cloneCourse
	active  int
	started bool
	current string
	done    int
	total   int
	err     error

	events chan tea.Msg
}

func newCloneScreen(s *session) *cloneScreen {
	dir, _ := core.ResolveCloneTarget("")

	input := textinput.New()
	input.Prompt = "┃ "
	input.Placeholder = "folder to clone into"
	input.CharLimit = 4096
	input.SetValue(dir)
	input.CursorEnd()

	m := &cloneScreen{
		sess:    s,
		state:   cloneChoosingDir,
		input:   input,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		bar:     progress.New(progress.WithSolidFill("#E0892B"), progress.WithoutPercentage()),
	}

	m.setWidth(s.width)
	return m
}

func (m *cloneScreen) setWidth(w int) {
	m.input.Width = max(10, min(w-6, 60))
	m.bar.Width = max(10, min(w-24, 60))
}

func (m *cloneScreen) Init() tea.Cmd {
	return m.input.Focus()
}

func (m *cloneScreen) View() string {
	var body string

	switch m.state {
	case cloneChoosingDir:
		body = titleStyle.Render("Clone your courses") + "\n\n" +
			"where should the courses go?\n" +
			m.input.View()
		if m.err != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + m.err.Error()
		}
	case cloneFetching:
		body = strings.TrimSpace(m.spinner.View()) + " Fetching your courses from " + m.sess.svc.Host() + "..."
	case cloneConfirm:
		body = titleStyle.Render(fmt.Sprintf("Found %d courses · %s", len(m.list), filesLabel(m.total))) + "\n\n" +
			m.renderChecklist() + "\n\n" +
			"Into " + tildify(m.dir) + "\n" +
			mutedStyle.Render("Press enter to start.")
	case cloneCloning:
		body = "Cloning into " + tildify(m.dir) + "\n\n" +
			m.renderProgress() + "\n\n" +
			m.renderChecklist() + "\n\n" +
			mutedStyle.Render("↓ "+truncate(m.current, max(10, m.sess.width-8)))

	case cloneDone:
		body = accentStyle.Render("✓ ") +
			titleStyle.Render(fmt.Sprintf("Cloned %d courses · %s", len(m.list), filesLabel(m.total))) + "\n" +
			mutedStyle.Render("into "+tildify(m.dir)) + "\n\n" +
			m.renderChecklist()

	case cloneFailed:
		errText := lipgloss.NewStyle().Width(max(10, m.sess.width-4)).Render(m.err.Error())
		body = accentStyle.Render("✗ ") + titleStyle.Render("Clone failed") + "\n\n" + errText
		if len(m.list) > 0 { // empty when fetching failed
			body += "\n\n" + m.renderChecklist()
		}
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *cloneScreen) renderProgress() string {
	percent := 1.0
	if m.total > 0 {
		percent = float64(m.done) / float64(m.total)
	}
	return m.bar.ViewAs(percent) + " " +
		mutedStyle.Render(fmt.Sprintf("%d / %d files %3.0f%%", m.done, m.total, percent*100))
}

func (m *cloneScreen) renderChecklist() string {
	rows := max(1, m.sess.height-10)
	start, end := 0, len(m.list)
	if len(m.list) > rows {
		if m.state == cloneCloning || m.state == cloneFailed {
			start = min(max(0, m.active-rows/2), len(m.list)-rows)
		}
		end = start + end
	}

	nameWidth := max(10, min(40, m.sess.width-20))
	nameStyle := lipgloss.NewStyle().Width(nameWidth)

	var lines []string
	for i := start; i < end; i++ {
		icon, count := m.courseStatus(i)
		lines = append(lines, icon+" "+nameStyle.Render(truncate(m.list[i].name, nameWidth))+" "+mutedStyle.Render(count))
	}
	if hidden := len(m.list) - (end - start); hidden > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... %d more", hidden)))
	}
	return strings.Join(lines, "\n")
}

func (m *cloneScreen) courseStatus(i int) (icon, count string) {
	c := m.list[i]
	progress := fmt.Sprintf("%d / %d", c.done, c.total)

	switch {
	case m.state == cloneConfirm:
		return mutedStyle.Render("·"), filesLabel(c.total)
	case m.state == cloneDone, i < m.active:
		return accentStyle.Render("✓"), filesLabel(c.total)
	case i == m.active && m.started && m.state == cloneCloning:
		return accentStyle.Render(strings.TrimSpace(m.spinner.View())), progress
	case i == m.active && m.started && m.state == cloneFailed:
		return accentStyle.Render("✗"), progress
	case m.state == cloneFailed:
		return mutedStyle.Render("·"), "not started"
	default:
		return mutedStyle.Render("·"), filesLabel(c.total)
	}
}

func filesLabel(n int) string {
	switch n {
	case 0:
		return "no files"
	case 1:
		return "1 files"
	default:
		return fmt.Sprintf("%d files", n)
	}
}

func fetchCoursesCmd(svc *core.Service) tea.Cmd {
	return func() tea.Msg {
		courses, err := svc.FetchCourses()
		if err != nil {
			return fetchFailedMsg{err}
		}
		return coursesFetchedMsg{courses}
	}
}

func startClone(svc *core.Service, dir string, courses []core.Course) chan tea.Msg {
	ch := make(chan tea.Msg, 64)
	go func() {
		err := svc.CloneCourses(dir, courses, func(e core.CloneEvent) {
			ch <- cloneEventMsg(e)
		})
		ch <- cloneFinishedMsg{err}
		close(ch)
	}()
	return ch
}

// waitForClone waits for the next message from the clone goroutine. Return it
// again after every cloneEventMsg, or no more events will arrive.
func waitForClone(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// reset goes back to choosing a folder, keeping what was typed.
func (m *cloneScreen) reset() {
	m.state = cloneChoosingDir
	m.courses, m.list = nil, nil
	m.active, m.started = 0, false
	m.done, m.total, m.current = 0, 0, ""
	m.err = nil
}

func (m *cloneScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case coursesFetchedMsg:
		if m.state != cloneFetching { // the user pressed esc while fetching
			return m, nil
		}
		m.courses = msg.courses
		for _, c := range msg.courses {
			n := 0
			for _, sec := range c.Sections {
				n += len(sec.GetFiles())
			}
			m.list = append(m.list, cloneCourse{name: c.DisplayName, total: n})
			m.total += n
		}
		m.state = cloneConfirm
		return m, nil

	case fetchFailedMsg:
		if m.state != cloneFetching {
			return m, nil
		}
		m.err = msg.err
		m.state = cloneFailed
		return m, nil

	case cloneEventMsg:
		switch msg.Kind {
		case core.CloneCourseCreated:
			if m.started {
				m.active++
			}
			m.started = true
		case core.CloneFileDownloaded:
			if m.active < len(m.list) {
				m.list[m.active].done++
			}
			m.done = msg.FilesDone
			m.current = msg.Section + "/" + msg.File
		}
		return m, waitForClone(m.events) // without this the screen freezes after the first event

	case cloneFinishedMsg:
		m.events = nil
		if msg.err != nil {
			m.err = msg.err
			m.state = cloneFailed
		} else {
			m.state = cloneDone
		}
		return m, nil

	case spinner.TickMsg:
		if m.state != cloneFetching && m.state != cloneCloning {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	// Cursor blinks for the folder input.
	if m.state == cloneChoosingDir {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *cloneScreen) handleKey(msg tea.KeyMsg) (screen, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case cloneChoosingDir:
		switch key {
		case "esc":
			return m, goTo(scrMenu)
		case "enter":
			dir, err := core.ResolveCloneTarget(strings.TrimSpace(m.input.Value()))
			if err != nil {
				m.err = err
				return m, nil
			}
			m.dir, m.err = dir, nil
			m.state = cloneFetching
			m.input.Blur()
			return m, tea.Batch(fetchCoursesCmd(m.sess.svc), m.spinner.Tick)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd

	case cloneFetching:
		if key == "esc" {
			m.reset()
			return m, m.input.Focus()
		}

	case cloneConfirm:
		switch key {
		case "esc":
			m.reset()
			return m, m.input.Focus()
		case "enter":
			m.state = cloneCloning
			m.events = startClone(m.sess.svc, m.dir, m.courses)
			return m, tea.Batch(waitForClone(m.events), m.spinner.Tick)
		}

	case cloneCloning:
		// A running clone can't be cancelled yet, so keys do nothing.
		// ctrl+c still quits, because app handles it.

	case cloneDone:
		if key == "enter" || key == "esc" {
			return m, goTo(scrMenu)
		}

	case cloneFailed:
		switch key {
		case "r":
			m.reset()
			return m, m.input.Focus()
		case "enter", "esc":
			return m, goTo(scrMenu)
		}
	}

	return m, nil
}
func (m *cloneScreen) ShortHelp() []key.Binding {
	switch m.state {
	case cloneChoosingDir:
		return []key.Binding{cloneKeyNext, cloneKeyBack}
	case cloneFetching:
		return []key.Binding{cloneKeyBack}
	case cloneConfirm:
		return []key.Binding{cloneKeyStart, cloneKeyFolder}
	case cloneDone:
		return []key.Binding{cloneKeyMenu}
	case cloneFailed:
		return []key.Binding{cloneKeyRetry, cloneKeyMenu}
	default: // cloning: nothing to press
		return nil
	}
}
