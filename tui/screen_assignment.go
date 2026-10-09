package tui

import (
	"errors"
	"fmt"
	"lectern/core"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type assignmentState int

const (
	assignmentChoose assignmentState = iota
	assignmentPage
	assignmentSubmit
	assignmentFailed
)

type assignmentMenuMsg struct{ assignments []core.Assignment }
type assignmentMsg struct{ assingment core.Assignment }
type assignmentSubmitMsg struct{}
type assignemntFailedMsg struct{ err error }

var (
	assignmentKeyMove   = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move"))
	assignmentKeyToggle = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select"))
	assignmentKeyAll    = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all"))
	assignmentKeyUpdate = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "update"))
	assignmentKeyMenu   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "menu"))
	assignmentKeyOpen   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	assignmentKeyRetry  = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "try again"))
	assignmentKeyBack   = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	assignmentKeyFolder = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open folder"))
	assignmentKeySubmit = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "submit"))
)

var (
	_ screen       = (*assignmentScreen)(nil)
	_ helpProvider = (*assignmentScreen)(nil)
)

type assignmentScreen struct {
	sess    *session
	state   assignmentState
	spinner spinner.Model

	assignments        []core.Assignment
	selectedAssignment core.Assignment
	root               string
	cursor             int
	openErr            error
	err                error
}

func newAssignmentScreen(s *session) *assignmentScreen {
	return &assignmentScreen{
		sess:    s,
		state:   assignmentChoose,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

func loadAssignmentsCmd(svc *core.Service) tea.Cmd {
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			return assignemntFailedMsg{err}
		}
		assignments, err := svc.FetchAssignments(dir)
		if err != nil {
			return assignemntFailedMsg{err}
		}
		return assignmentMenuMsg{
			assignments: assignments,
		}
	}
}

func (m *assignmentScreen) loading() bool {
	return m.state == assignmentChoose && m.assignments == nil
}

func (m *assignmentScreen) assignmentIndex(id int) int {
	for i, a := range m.assignments {
		if a.Info.ID == id {
			return i
		}
	}

	return 0
}

func (m *assignmentScreen) moveCursor(key string, n int) bool {
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

func dueLabel(due, now time.Time) string {
	if due.IsZero() {
		return "no deadline"
	}

	d := due.Sub(now)
	switch {
	case d < 0:
		return "was due " + core.HumanAgo(due)
	case d < time.Hour:
		return fmt.Sprintf("due in %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("due in %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("due in %dd", int(d.Hours()/24))
	}
}

func statusLabel(a core.Assignment, now time.Time) string {
	state := a.State(now)

	switch state {
	case core.AssignmentGraded:
		return "graded " + a.Grade()
	case core.AssignmentSubmittedLate:
		return fmt.Sprintf("submitted %s late", core.HumanDuration(a.LateBy()))
	case core.AssignmentDraft:
		if a.IsPastDue(now) {
			return "draft, never submitted"
		}
	}
	return state.String()
}

func needsAttention(a core.Assignment, now time.Time) bool {
	switch a.State(now) {
	case core.AssignmentOverDue, core.AssignmentMissed:
		return true
	case core.AssignmentDraft:
		return a.IsPastDue(now)
	}
	return false
}

func (m *assignmentScreen) handlekey(msg tea.KeyMsg) (screen, tea.Cmd) {
	key := msg.String()

	switch m.state {
	case assignmentChoose:
		if key == "esc" {
			return m, goTo(scrMenu)
		}

		if m.loading() {
			return m, nil
		}

		if m.moveCursor(key, len(m.assignments)) {
			return m, nil
		}

		if key == "enter" && len(m.assignments) > 0 {
			assignment := m.assignments[m.cursor]
			return m, func() tea.Msg { return assignmentMsg{assingment: assignment} }
		}

	case assignmentPage:
		m.openErr = nil

		switch key {
		case "esc":
			m.state = assignmentChoose
			m.cursor = m.assignmentIndex(m.selectedAssignment.Info.ID)
			return m, nil
		case "enter":
			if m.selectedAssignment.Path == "" {
				return m, nil
			}
			return m, openFileCmd(m.selectedAssignment.Path)
		case "s":
			return m, func() tea.Msg { return assignmentSubmitMsg{} }
		}

	case assignmentSubmit:
		if key == "esc" {
			m.state = assignmentPage
		}

	case assignmentFailed:
		switch key {
		case "r":
			m.state = assignmentChoose
			m.assignments, m.err, m.cursor = nil, nil, 0
			return m, m.Init()
		case "enter", "esc":
			return m, goTo(scrMenu)
		}
	}

	return m, nil
}

func (m *assignmentScreen) Init() tea.Cmd {
	return tea.Batch(loadAssignmentsCmd(m.sess.svc), m.spinner.Tick)
}

func (m *assignmentScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handlekey(msg)

	case assignmentMenuMsg:
		if !m.loading() {
			return m, nil
		}

		m.assignments = msg.assignments

		if m.assignments == nil {
			m.assignments = []core.Assignment{}
		}

		m.cursor = 0
		return m, nil

	case assignmentMsg:
		m.selectedAssignment = msg.assingment
		m.state = assignmentPage
		return m, nil

	case assignmentSubmitMsg:
		m.state = assignmentSubmit
		return m, nil

	case assignemntFailedMsg:
		m.err = msg.err
		m.state = assignmentFailed
		return m, nil

	case spinner.TickMsg:
		if !m.loading() {
			return m, nil
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case courseOpenFailMsg:
		if m.state == assignmentPage {
			m.openErr = msg.err
		}
		return m, nil
	}

	return m, nil
}

func (m *assignmentScreen) View() string {
	var body string
	now := time.Now()

	switch m.state {
	case assignmentChoose:
		if m.loading() {
			body = strings.TrimSpace(m.spinner.View()) + " Loading your assignments..."
			break
		}

		if len(m.assignments) == 0 {
			body = titleStyle.Render("No assignments") + "\n" +
				mutedStyle.Render("Your courses don't have any assignments yet.")
			break
		}

		items := make([][2]string, len(m.assignments))

		for i, a := range m.assignments {
			detail := a.CourseName + " · " + dueLabel(a.DueDate(), now) + " · " + statusLabel(a, now)

			if needsAttention(a, now) {
				detail = accentStyle.Render("! ") + detail
			}
			items[i] = [2]string{a.Info.Name, detail}
		}
		body = titleStyle.Render("Your assignments") + "\n\n" + renderCursorList(items, m.cursor, m.sess)

	case assignmentPage:
		a := m.selectedAssignment
		body = titleStyle.Render(a.Info.Name) + "\n" +
			mutedStyle.Render(a.CourseName) + "\n\n" +
			m.renderDetails(a, now)
		if m.openErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + "couldn't open the folder: " + m.openErr.Error()
		}

	case assignmentSubmit:
		body = titleStyle.Render("Submit") + "\n\n" +
			mutedStyle.Render("SUbmitting isn't available yet.")

	case assignmentFailed:
		if errors.Is(m.err, core.ErrNoWorkspace) {
			dir, _ := os.Getwd()
			body = titleStyle.Render("Not a lectern workspace") + "\n\n" +
				mutedStyle.Render(tildify(dir)+" isn't inside a cloned folder.") + "\n" +
				"Run CLone to create one, or start lectern from isnide a workspace."
		} else {
			errText := lipgloss.NewStyle().Width(max(10, m.sess.width-4)).Render(m.err.Error())
			body = accentStyle.Render("✗ ") + titleStyle.Render("Couldn't load your assignments") + "\n\n" + errText
		}
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *assignmentScreen) renderDetails(a core.Assignment, now time.Time) string {
	const dateFormat = "Mon 2 Jan 2006m 15:04"
	labelStyle := mutedStyle.Width(12)

	var rows [][2]string
	add := func(label, value string) { rows = append(rows, [2]string{label, value}) }

	status := statusLabel(a, now)
	if needsAttention(a, now) {
		status = accentStyle.Render(status)
	}

	add("Status", status)

	if due := a.DueDate(); !due.IsZero() {
		value := due.Format(dateFormat) + " " + mutedStyle.Render(dueLabel(due, now))
		if ext := a.Status.LastAttempt.ExtensionDueDate; ext != nil && *ext > 0 {
			value += mutedStyle.Render(" (extension)")
		}
		add("Due", value)
	} else {
		add("Due", mutedStyle.Render("no deadline"))
	}

	if cutoff := a.CutOffDate(); !cutoff.IsZero() {
		add("Closes", cutoff.Format(dateFormat))
	}

	if opens := a.OpenAt(); !opens.IsZero() && now.Before(opens) {
		add("Opens", opens.Format(dateFormat))
	}

	if sub := a.Submission(); sub != nil && sub.Status != "new" {
		files := sub.Files()
		names := make([]string, len(files))

		for i, f := range files {
			names[i] = f.FileName
		}

		value := filesLabel(len(files))
		if len(names) > 0 {
			value += mutedStyle.Render(" (" + truncate(strings.Join(names, ", "), max(10, m.sess.width-40)) + ")")
		}
		add("submitted", value)
	}

	if grade := a.Grade(); grade != "" {
		add("Grade", grade)
	} else if a.Status.LastAttempt.GradingStatus == "graded" {
		add("Grade", mutedStyle.Render("graded, not released yet"))
	}

	if a.Path != "" {
		add("Folder", tildify(a.Path))
	} else {
		add("Folder", mutedStyle.Render("not synced yet. Run Sync to download it."))
	}

	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = labelStyle.Render(r[0]) + r[1]
	}
	return strings.Join(lines, "\n")
}

func (m *assignmentScreen) ShortHelp() []key.Binding {
	switch m.state {
	case assignmentChoose:
		return []key.Binding{assignmentKeyMove, assignmentKeyOpen, assignmentKeyBack}
	case assignmentPage:
		return []key.Binding{assignmentKeyFolder, assignmentKeySubmit, assignmentKeyBack}
	case assignmentSubmit:
		return []key.Binding{assignmentKeyBack}
	case assignmentFailed:
		return []key.Binding{assignmentKeyRetry, assignmentKeyMenu}
	default:
		return nil
	}
}
