package tui

import (
	"fmt"
	"lectern/core"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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
