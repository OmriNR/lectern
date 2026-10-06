package tui

import (
	"errors"
	"fmt"
	"lectern/core"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type courseState int

const (
	courseChooseMenu courseState = iota
	courseMenu
	courseSection
	courseUpload
	courseFailed
)

type courseChooseMsg struct {
	root    string
	courses []core.CourseReport
}
type courseMenuMsg struct{ course core.CourseReport }
type courseSectionMsg struct{ section core.SectionState }
type courseUploadFileMsg struct{}
type courseFailMsg struct{ err error }
type courseOpenFailMsg struct{ err error }

var (
	courseKeyMove   = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move"))
	courseKeyToggle = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select"))
	courseKeyAll    = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all"))
	courseKeyUpdate = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "update"))
	courseKeyMenu   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "menu"))
	courseKeyOpen   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	courseKeyRetry  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "try again"))
	courseKeyBack   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
)

var (
	_ screen       = (*courseScreen)(nil)
	_ helpProvider = (*courseScreen)(nil)
)

type courseScreen struct {
	sess    *session
	state   courseState
	spinner spinner.Model

	courses         []core.CourseReport
	selectedCourse  core.CourseReport
	selectedSection core.SectionState
	root            string // workspace root; file paths are relative to it
	cursor          int
	openErr         error
	err             error
}

func newCourseScreen(s *session) *courseScreen {
	return &courseScreen{
		sess:    s,
		state:   courseChooseMenu,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

func loadCoursesCmd(svc *core.Service) tea.Cmd {
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			return courseFailMsg{err}
		}
		report, err := svc.LoadStatus(dir)
		if err != nil {
			return courseFailMsg{err}
		}
		return courseChooseMsg{root: report.Root, courses: report.Courses}
	}
}

// openFileCmd opens path in the system's default app for its type, outside
// the terminal.
func openFileCmd(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", path)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", path)
		default:
			cmd = exec.Command("xdg-open", path)
		}

		if err := cmd.Start(); err != nil {
			return courseOpenFailMsg{err}
		}
		go cmd.Wait() // reap the opener without blocking the UI
		return nil
	}
}

func (m *courseScreen) loading() bool {
	return m.state == courseChooseMenu && m.courses == nil
}

// sections returns the selected course's sections in course order.
func (m *courseScreen) sections() []core.SectionState {
	sections := make([]core.SectionState, 0, len(m.selectedCourse.Sections))
	for _, s := range m.selectedCourse.Sections {
		sections = append(sections, s)
	}
	sort.Slice(sections, func(i, j int) bool { return sections[i].Number < sections[j].Number })
	return sections
}

// files returns the selected section's files sorted by name.
func (m *courseScreen) files() []core.FileState {
	files := make([]core.FileState, 0, len(m.selectedSection.Files))
	for _, f := range m.selectedSection.Files {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return files
}

func (m *courseScreen) sectionIndex(id int) int {
	for i, s := range m.sections() {
		if s.ID == id {
			return i
		}
	}
	return 0
}

func (m *courseScreen) courseIndex(id int) int {
	for i, c := range m.courses {
		if c.ID == id {
			return i
		}
	}
	return 0
}

func (m *courseScreen) moveCursor(key string, n int) bool {
	switch key {
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = max(0, min(n-1, m.cursor+1))
	default:
		return false
	}
	return true
}

func (m *courseScreen) handleKey(msg tea.KeyMsg) (screen, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case courseChooseMenu:
		if key == "esc" {
			return m, goTo(scrMenu)
		}
		if m.loading() {
			return m, nil
		}
		if m.moveCursor(key, len(m.courses)) {
			return m, nil
		}
		if key == "enter" && len(m.courses) > 0 {
			course := m.courses[m.cursor]
			return m, func() tea.Msg { return courseMenuMsg{course} }
		}

	case courseMenu:
		sections := m.sections()
		switch key {
		case "esc":
			m.state = courseChooseMenu
			m.cursor = m.courseIndex(m.selectedCourse.ID)
			return m, nil
		case "enter":
			if len(sections) == 0 {
				return m, nil
			}
			section := sections[m.cursor]
			return m, func() tea.Msg { return courseSectionMsg{section} }
		default:
			m.moveCursor(key, len(sections))
		}

	case courseSection:
		files := m.files()
		m.openErr = nil
		switch key {
		case "esc":
			m.state = courseMenu
			m.cursor = m.sectionIndex(m.selectedSection.ID)
			return m, nil
		case "enter":
			if len(files) == 0 {
				return m, nil
			}
			return m, openFileCmd(filepath.Join(m.root, files[m.cursor].Path))
		default:
			m.moveCursor(key, len(files))
		}

	case courseUpload:
		if key == "esc" {
			m.state = courseSection
		}

	case courseFailed:
		switch key {
		case "r":
			m.state = courseChooseMenu
			m.courses, m.err, m.cursor = nil, nil, 0
			return m, m.Init()
		case "enter", "esc":
			return m, goTo(scrMenu)
		}
	}

	return m, nil
}

func (m *courseScreen) Init() tea.Cmd {
	return tea.Batch(loadCoursesCmd(m.sess.svc), m.spinner.Tick)
}

func (m *courseScreen) renderList(items [][2]string) string {
	rows := max(1, m.sess.height-10)
	start, end := 0, len(items)

	if len(items) > rows {
		start = min(max(0, m.cursor-rows/2), len(items)-rows)
		end = start + rows
	}

	nameWidth := max(10, min(40, m.sess.width-20))
	nameStyle := lipgloss.NewStyle().Width(nameWidth)

	var lines []string
	if start > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... %d more", start)))
	}

	for i := start; i < end; i++ {
		pointer, style := "  ", nameStyle.Foreground(colorText)
		if i == m.cursor {
			pointer, style = accentStyle.Render("> "), nameStyle.Foreground(colorOrange)
		}
		lines = append(lines, pointer+style.Render(truncate(items[i][0], nameWidth))+" "+mutedStyle.Render(items[i][1]))
	}

	if hidden := len(items) - end; hidden > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... %d more", hidden)))
	}

	return strings.Join(lines, "\n")
}

func sectionTitle(s core.SectionState) string {
	if name := strings.TrimSpace(s.Name); name != "" {
		return name
	}
	return s.Folder
}

func (m *courseScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case courseChooseMsg:
		if !m.loading() {
			return m, nil
		}
		m.root = msg.root
		m.courses = msg.courses
		m.cursor = 0
		return m, nil

	case courseMenuMsg:
		m.selectedCourse = msg.course
		m.state = courseMenu
		m.cursor = 0
		return m, nil

	case courseSectionMsg:
		m.selectedSection = msg.section
		m.state = courseSection
		m.cursor = 0
		return m, nil

	case courseUploadFileMsg:
		m.state = courseUpload
		return m, nil

	case courseFailMsg:
		m.err = msg.err
		m.state = courseFailed
		return m, nil

	case spinner.TickMsg:
		if !m.loading() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case courseOpenFailMsg:
		if m.state == courseSection {
			m.openErr = msg.err
		}
		return m, nil
	}

	return m, nil
}

func (m *courseScreen) View() string {
	var body string

	switch m.state {
	case courseChooseMenu:
		if m.loading() {
			body = strings.TrimSpace(m.spinner.View()) + " Loading your courses..."
			break
		}

		if len(m.courses) == 0 {
			body = titleStyle.Render("No courses yet") + "\n" +
				mutedStyle.Render("Run Clone or Sync to download your courses.")
			break
		}
		items := make([][2]string, len(m.courses))
		for i, c := range m.courses {
			items[i] = [2]string{c.DisplayName, c.ShortName}
		}
		body = titleStyle.Render("Choose a course") + "\n\n" + m.renderList(items)

	case courseMenu:
		sections := m.sections()
		items := make([][2]string, len(sections))
		for i, s := range sections {
			items[i] = [2]string{sectionTitle(s), filesLabel(len(s.Files))}
		}
		body = titleStyle.Render(m.selectedCourse.DisplayName) + "\n" +
			mutedStyle.Render(m.selectedCourse.ShortName) + "\n\n"
		if len(items) == 0 {
			body += mutedStyle.Render("This course has no sections.")
		} else {
			body += m.renderList(items)
		}

	case courseSection:
		files := m.files()
		items := make([][2]string, len(files))
		for i, f := range files {
			items[i] = [2]string{f.Name, ""}
		}
		body = titleStyle.Render(sectionTitle(m.selectedSection)) + "\n" +
			mutedStyle.Render(m.selectedCourse.DisplayName) + "\n\n"
		if len(items) == 0 {
			body += mutedStyle.Render("This section has no files.")
		} else {
			body += m.renderList(items)
		}
		if m.openErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + "Couldn't open the file: " + m.openErr.Error()
		}

	case courseUpload:
		body = titleStyle.Render("Upload") + "\n\n" +
			mutedStyle.Render("Uploading isn't available yet.")

	case courseFailed:
		if errors.Is(m.err, core.ErrNoWorkspace) {
			dir, _ := os.Getwd()
			body = titleStyle.Render("Not a lectern workspace") + "\n\n" +
				mutedStyle.Render(tildify(dir)+" isn't inside a cloned folder.") + "\n" +
				"Run Clone to create one, or start lectern from inside a workspace."
		} else {
			errText := lipgloss.NewStyle().Width(max(10, m.sess.width-4)).Render(m.err.Error())
			body = accentStyle.Render("✗ ") + titleStyle.Render("Couldn't load your courses") + "\n\n" + errText
		}
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *courseScreen) ShortHelp() []key.Binding {
	switch m.state {
	case courseChooseMenu, courseMenu:
		return []key.Binding{courseKeyMove, courseKeyToggle, courseKeyUpdate, courseKeyBack}
	case courseSection:
		return []key.Binding{courseKeyMove, courseKeyOpen, courseKeyBack}
	case courseUpload:
		return []key.Binding{courseKeyBack}
	case courseFailed:
		return []key.Binding{courseKeyRetry, courseKeyMenu}
	default:
		return nil
	}
}
