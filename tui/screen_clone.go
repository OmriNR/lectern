package tui

import (
	"lectern/core"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type cloneState int

const (
	cloneChoosingDir cloneState = iota
	cloneFetching
	cloneConfirm
	cloneDone
	cloneFailed
)

type coursesFethcedMsg struct{ courses []core.Course }
type fetchFailedMsg struct{ err error }
type cloneEventMsg core.CloneEvent
type cloneFinishedMsg struct{ err error }

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
