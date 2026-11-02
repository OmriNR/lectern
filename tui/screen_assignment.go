package tui

import (
	"errors"
	"fmt"
	"lectern/core"
	"os"
	"path/filepath"
	"slices"
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
	assignmentConfirm
	assignmentExported
	assignmentFailed
)

type assignmentMenuMsg struct{ assignments []core.Assignment }
type assignmentMsg struct{ assingment core.Assignment }
type assignmentExportedMsg struct {
	path   string
	result core.SyncResult
}
type assignmentFilePickedMsg struct {
	path string
	size int64
}
type assignmentPickCanceledMsg struct{}
type assignmentGoogleOpenedMsg struct{ opened int }
type assignmentExportFailedMsg struct{ err error }
type assignemntFailedMsg struct{ err error }
type assignmentUploadFailedMsg struct{ err error }

var (
	assignmentKeyMove     = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "move"))
	assignmentKeyToggle   = key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "select"))
	assignmentKeyAll      = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "all"))
	assignmentKeyUpdate   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "update"))
	assignmentKeyMenu     = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "menu"))
	assignmentKeyOpen     = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open"))
	assignmentKeyRetry    = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "try again"))
	assignmentKeyBack     = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
	assignmentKeyFolder   = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open folder"))
	assignmentKeyExport   = key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "export .ics"))
	assignmentKeyGoogle   = key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "google calendar"))
	assignmentKeyCalendar = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add to calendar"))
	assignmentKeyMoodle   = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open in moodle"))
	assignmentKeyUpload   = key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "upload"))
	assignmentKeyConfirm  = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "upload & submit"))
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

	selected     []bool
	exportFrom   assignmentState
	exportPath   string
	exportResult core.SyncResult
	exportNotice string
	exportErr    error

	picking    bool
	uploadPath string
	uploadErr  error
	uploadSize int64
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

func openAssignmentInMoodleCmd(svc *core.Service, assignment core.Assignment) tea.Cmd {
	return func() tea.Msg {
		if err := svc.OpenInMoodle(assignment); err != nil {
			return courseOpenFailMsg{err}
		}
		return nil
	}
}

func pickFileCmd(svc *core.Service, assignment core.Assignment) tea.Cmd {
	return func() tea.Msg {
		path, err := svc.PickFileFromMachine(assignment.Path)

		if errors.Is(err, core.ErrPickCanceled) {
			return assignmentPickCanceledMsg{}
		}

		if err != nil {
			return assignmentUploadFailedMsg{err}
		}

		size, err := core.CheckUploadFile(path)

		if err != nil {
			return assignmentUploadFailedMsg{err}
		}

		return assignmentFilePickedMsg{path, size}
	}
}

func exportAssignmentsCmd(svc *core.Service, assignments []core.Assignment) tea.Cmd {
	return func() tea.Msg {
		path, result, err := svc.ExportAssignments(assignments)
		if err != nil {
			return assignmentExportFailedMsg{err}
		}
		return assignmentExportedMsg{path: path, result: result}
	}
}

func googleCalendarCmd(svc *core.Service, assignments []core.Assignment) tea.Cmd {
	return func() tea.Msg {
		n, err := svc.AddToGoogleCalendar(assignments)
		if err != nil {
			return assignmentExportFailedMsg{err}
		}
		return assignmentGoogleOpenedMsg{opened: n}
	}
}

func (m *assignmentScreen) toExport(now time.Time) []core.Assignment {
	var picked []core.Assignment
	for i, a := range m.assignments {
		if m.selected[i] && a.IsSchedulable(now) {
			picked = append(picked, a)
		}
	}

	if len(picked) == 0 {
		if a := m.assignments[m.cursor]; a.IsSchedulable(now) {
			picked = append(picked, a)
		}
	}

	return picked
}

func (m *assignmentScreen) startExport(assignments []core.Assignment, export func(*core.Service, []core.Assignment) tea.Cmd) tea.Cmd {
	if len(assignments) == 0 {
		m.exportErr = errors.New("nothing to export: only upcoming, unsubmitted assignments can go on a calendar")
		return nil
	}
	m.exportFrom = m.state
	return export(m.sess.svc, assignments)
}

func assignmentsLabel(n int) string {
	if n == 1 {
		return "1 assignment"
	}

	return fmt.Sprintf("%d assignments", n)
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
		m.exportErr, m.exportNotice = nil, ""

		if key == "esc" {
			return m, goTo(scrMenu)
		}

		if m.loading() || len(m.assignments) == 0 {
			return m, nil
		}

		if m.moveCursor(key, len(m.assignments)) {
			return m, nil
		}

		now := time.Now()
		switch key {
		case "enter":
			assignment := m.assignments[m.cursor]
			return m, func() tea.Msg { return assignmentMsg{assingment: assignment} }
		case " ":
			if m.assignments[m.cursor].IsSchedulable(now) {
				m.selected[m.cursor] = !m.selected[m.cursor]
			}
		case "a":
			all := !slices.Contains(m.selected, true)
			for i, a := range m.assignments {
				m.selected[i] = all && a.IsSchedulable(now)
			}
		case "e":
			return m, m.startExport(m.toExport(now), exportAssignmentsCmd)
		case "g":
			return m, m.startExport(m.toExport(now), googleCalendarCmd)
		}
	case assignmentPage:
		m.openErr, m.exportErr, m.uploadErr = nil, nil, nil

		switch key {
		case "esc":
			m.state = assignmentChoose
			m.uploadPath = ""
			m.cursor = m.assignmentIndex(m.selectedAssignment.Info.ID)
			return m, nil
		case "enter":
			if m.selectedAssignment.Path == "" {
				return m, nil
			}
			return m, openFileCmd(m.selectedAssignment.Path)
		case "e":
			return m, m.startExport([]core.Assignment{m.selectedAssignment}, exportAssignmentsCmd)
		case "g":
			return m, m.startExport([]core.Assignment{m.selectedAssignment}, googleCalendarCmd)
		case "o":
			return m, openAssignmentInMoodleCmd(m.sess.svc, m.selectedAssignment)
		case "u":
			if m.picking {
				return m, nil
			}
			if !m.selectedAssignment.Status.LastAttempt.SubmissionsEnabled {
				m.uploadErr = errors.New("this assignment doesn't take submissions through Moodle")
				return m, nil
			}
			if !m.selectedAssignment.Status.LastAttempt.CanEdit {
				m.uploadErr = errors.New("this assignment can't be edited from here. Press o to open it in Moodle")
				return m, nil
			}
			m.picking = true
			return m, pickFileCmd(m.sess.svc, m.selectedAssignment)
		}

	case assignmentConfirm:
		switch key {
		case "esc":
			m.uploadPath = ""
			m.state = assignmentPage
		case "enter":
			m.uploadPath = ""
			m.exportNotice = "Uploading comes in step 4"
			m.state = assignmentPage
		}

	case assignmentExported:
		switch key {
		case "enter":
			m.openErr = nil
			return m, openFileCmd(m.exportPath)
		case "esc":
			m.state, m.openErr = m.exportFrom, nil
		}

	case assignmentFailed:
		switch key {
		case "r":
			m.state = assignmentChoose
			m.assignments, m.selected, m.err, m.cursor = nil, nil, nil, 0
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

		m.selected = make([]bool, len(m.assignments))
		m.cursor = 0
		return m, nil

	case assignmentMsg:
		m.selectedAssignment = msg.assingment
		m.state = assignmentPage
		return m, nil

	case assignmentFilePickedMsg:
		m.picking = false
		m.uploadPath = msg.path
		m.uploadSize = msg.size
		m.state = assignmentConfirm
		return m, nil

	case assignmentPickCanceledMsg:
		m.picking = false
		return m, nil

	case assignmentUploadFailedMsg:
		m.picking = false
		m.uploadErr = msg.err
		return m, nil

	case assignmentExportedMsg:
		m.exportPath, m.exportResult = msg.path, msg.result
		m.state = assignmentExported
		clear(m.selected)
		return m, nil

	case assignmentExportFailedMsg:
		m.exportErr = msg.err
		return m, nil

	case assignemntFailedMsg:
		m.err = msg.err
		m.state = assignmentFailed
		return m, nil

	case assignmentGoogleOpenedMsg:
		m.exportNotice = fmt.Sprintf("Opened %s in your browser. Press Save on each tab.", assignmentsLabel(msg.opened))
		clear(m.selected)
		return m, nil

	case spinner.TickMsg:
		if !m.loading() {
			return m, nil
		}

		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case courseOpenFailMsg:
		if m.state == assignmentPage || m.state == assignmentExported {
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

			box := "    "
			if a.IsSchedulable(now) {
				box = "[ ] "
				if m.selected[i] {
					box = "[✓] "
				}
			}
			items[i] = [2]string{box + a.Info.Name, detail}
		}
		body = titleStyle.Render("Your assignments") + "\n\n" + renderCursorList(items, m.cursor, m.sess)

		if m.exportErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + m.exportErr.Error()
		}

		if m.exportNotice != "" {
			body += "\n\n" + accentStyle.Render("✓ ") + m.exportNotice
		}

	case assignmentPage:
		a := m.selectedAssignment
		body = titleStyle.Render(a.Info.Name) + "\n" +
			mutedStyle.Render(a.CourseName) + "\n\n" +
			m.renderDetails(a, now)

		if m.openErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + "couldn't open it: " + m.openErr.Error()
		}

		if m.picking {
			body += "\n\n" + mutedStyle.Render("Waiting for the file chooser...")
		}

		if m.uploadErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + m.uploadErr.Error()
		}

		if m.exportErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + m.exportErr.Error()
		}

		if m.exportNotice != "" {
			body += "\n\n" + accentStyle.Render("✓ ") + m.exportNotice
		}

	case assignmentExported:
		r := m.exportResult
		body = accentStyle.Render("✓ ") + titleStyle.Render("Exported "+assignmentsLabel(r.Added+r.Updated+r.Unchanged)) + "\n" +
			mutedStyle.Render(tildify(m.exportPath)) + "\n\n" +
			fmt.Sprintf("%d new · %d updated · %d unchanged", r.Added, r.Updated, r.Unchanged) + "\n\n" +
			mutedStyle.Render("Press enter to open it in your calendar app, or import the file manually.")
		if m.openErr != nil {
			body += "\n\n" + accentStyle.Render("✗ ") + "couldn't open the file: " + m.openErr.Error()
		}

	case assignmentConfirm:
		body = titleStyle.Render("Upload and submit?") + "\n\n" + m.renderConfirm(m.selectedAssignment)

	case assignmentFailed:
		if errors.Is(m.err, core.ErrNoWorkspace) {
			dir, _ := os.Getwd()
			body = titleStyle.Render("Not a lectern workspace") + "\n\n" +
				mutedStyle.Render(tildify(dir)+" isn't inside a cloned folder.") + "\n" +
				"Run Clone to create one, or start lectern from inside a workspace."
		} else {
			errText := lipgloss.NewStyle().Width(max(10, m.sess.width-4)).Render(m.err.Error())
			body = accentStyle.Render("✗ ") + titleStyle.Render("Couldn't load your assignments") + "\n\n" + errText
		}
	}

	return lipgloss.NewStyle().Padding(1, 2).Render(body)
}

func (m *assignmentScreen) renderDetails(a core.Assignment, now time.Time) string {
	const dateFormat = "Mon 2 Jan 2006 15:04"
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

	add("Moodle", mutedStyle.Render(a.MoodleURL(m.sess.svc.Host())))

	lines := make([]string, len(rows))
	for i, r := range rows {
		lines[i] = labelStyle.Render(r[0]) + r[1]
	}
	return strings.Join(lines, "\n")
}

func (m *assignmentScreen) renderConfirm(a core.Assignment) string {
	labelStyle := mutedStyle.Width(12)

	lines := []string{
		labelStyle.Render("File") + filepath.Base(m.uploadPath),
		labelStyle.Render("Size") + core.HumanSize(m.uploadSize),
		labelStyle.Render("Assignment") + a.Info.Name + mutedStyle.Render(" · "+a.CourseName),
	}

	if sub := a.Submission(); sub != nil {
		if files := sub.Files(); len(files) > 0 {
			names := make([]string, len(files))
			for i, f := range files {
				names[i] = f.FileName
			}
			warning := fmt.Sprintf("Replaces %s already submitted: %s", filesLabel(len(files)),
				truncate(strings.Join(names, ", "), max(10, m.sess.width-50)))
			lines = append(lines, labelStyle.Render("Note")+accentStyle.Render(warning))
		}
	}

	if a.Info.SubmissionsDrafts == 1 {
		lines = append(lines, "", mutedStyle.Render("Submitting closes the assignment. You can't change it afterwards unless your lecturer reopens it."))
	}

	return strings.Join(lines, "\n")
}

func (m *assignmentScreen) ShortHelp() []key.Binding {
	switch m.state {
	case assignmentChoose:
		return []key.Binding{assignmentKeyMove, assignmentKeyOpen, assignmentKeyToggle, assignmentKeyAll, assignmentKeyExport, assignmentKeyGoogle, assignmentKeyBack}
	case assignmentPage:
		return []key.Binding{assignmentKeyFolder, assignmentKeyMoodle, assignmentKeyUpload, assignmentKeyExport, assignmentKeyGoogle, assignmentKeyBack}
	case assignmentExported:
		return []key.Binding{assignmentKeyCalendar, assignmentKeyBack}
	case assignmentConfirm:
		return []key.Binding{assignmentKeyConfirm, assignmentKeyBack}
	case assignmentFailed:
		return []key.Binding{assignmentKeyRetry, assignmentKeyMenu}
	default:
		return nil
	}
}
