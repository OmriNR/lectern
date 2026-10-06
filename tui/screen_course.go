package tui

import (
	"lectern/core"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
)

type courseState int

const (
	courseChooseMenu courseState = iota
	courseMenu
	courseSection
	courseUpload
	courseFailed
)

type courseChooseMsg struct{ courses []core.Course }
type courseMenuMsg struct{ course core.Course }
type courseSectionMsg struct{ section core.Section }
type courseUploadFileMsg struct{}
type courseFailMsg struct{ err error }

var (
	courseKeyMove   = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move"))
	courseKeyToggle = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select"))
	courseKeyAll    = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all"))
	courseKeyUpdate = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "update"))
	courseKeyMenu   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "menu"))
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

	courses         []core.Course
	selectedCourse  core.Course
	selectedSection core.Section
	err             error
}

func newCourseScreen(s *session) *courseScreen {
	return &courseScreen{
		sess:    s,
		state:   courseChooseMenu,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
	}
}

func (m *courseScreen) ShortHelp() []key.Binding {
	switch m.state {
	case courseChooseMenu, courseMenu, courseSection:
		return []key.Binding{courseKeyMove, courseKeyToggle, courseKeyUpdate, courseKeyBack}
	case courseUpload:
		return []key.Binding{courseKeyBack}
	case courseFailed:
		return []key.Binding{courseKeyRetry, courseKeyMenu}
	default:
		return nil
	}
}
