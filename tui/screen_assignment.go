package tui

import (
	"lectern/core"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
)

type assignmentState int

const (
	assignmentChoose assignmentState = iota
	assignmentPage
	assignmentSubmit
	assignmentFailed
)

type assignmentMenuMsg struct{ assignments core.Assignment }
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
