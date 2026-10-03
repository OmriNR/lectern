package tui

import (
	"errors"
	"fmt"
	"lectern/core"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type statusState int

const (
	statusLoading statusState = iota
	statusReady
	statusNotWorkspace
	statusFailed
)

type statusLoadedMsg struct{ report *core.StatusReport }
type statusFailedMsg struct{ err error }

func loadStatusCmd(svc *core.Service) tea.Cmd {
	return func() tea.Msg {
		dir, err := os.Getwd()
		if err != nil {
			return statusFailedMsg{err}
		}
		report, err := svc.LoadStatus(dir)
		if err != nil {
			return statusFailedMsg{err}
		}
		return statusLoadedMsg{report}
	}
}

// Temporary styles until theme.go exists.
var (
	statusMutedStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B6B6B", Dark: "#8A8A8A"})
	statusTitleStyle = lipgloss.NewStyle().Bold(true)
)

var (
	statusKeyScroll  = key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑/↓", "scroll"))
	statusKeyRefresh = key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	statusKeyClone   = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clone"))
	statusKeyBack    = key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back"))
)

var (
	_ screen       = (*statusScreen)(nil)
	_ helpProvider = (*statusScreen)(nil)
)

type statusScreen struct {
	sess          *session
	state         statusState
	report        *core.StatusReport
	err           error
	dir           string // where lectern was started, for the not-a-workspace message
	spinner       spinner.Model
	vp            viewport.Model
	width, height int
}

func newStatusScreen(s *session) *statusScreen {
	dir, _ := os.Getwd()

	m := &statusScreen{
		sess:    s,
		state:   statusLoading,
		dir:     dir,
		spinner: spinner.New(spinner.WithSpinner(spinner.Dot)),
		vp:      viewport.New(0, 0),
	}
	m.setSize(s.width, s.height)
	return m
}

func (m *statusScreen) Init() tea.Cmd {
	return tea.Batch(loadStatusCmd(m.sess.svc), m.spinner.Tick)
}

// setSize fits the viewport under the summary and re-renders the course list,
// since the number of file columns depends on the width.
func (m *statusScreen) setSize(width, height int) {
	m.width, m.height = width, height
	m.vp.Width = width
	m.vp.Height = max(1, height-lipgloss.Height(m.renderSummary())-1) // -1 for the separator
	if m.report != nil {
		m.vp.SetContent(m.renderCourses(width))
	}
}

func (m *statusScreen) reload() tea.Cmd {
	m.state = statusLoading
	return tea.Batch(loadStatusCmd(m.sess.svc), m.spinner.Tick)
}

func (m *statusScreen) Update(msg tea.Msg) (screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil

	case statusLoadedMsg:
		m.state = statusReady
		m.report = msg.report
		m.err = nil
		m.setSize(m.width, m.height) // the summary exists now, so the viewport height changes
		m.vp.GotoTop()
		return m, nil

	case statusFailedMsg:
		m.report = nil
		m.err = msg.err
		if errors.Is(msg.err, core.ErrNoWorkspace) {
			m.state = statusNotWorkspace
		} else {
			m.state = statusFailed
		}
		return m, nil

	case spinner.TickMsg:
		if m.state != statusLoading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, goTo(scrMenu)
		case "r":
			if m.state != statusLoading {
				return m, m.reload()
			}
			return m, nil
		case "c":
			if m.state == statusNotWorkspace {
				if m.sess.username == "" {
					return m, goTo(scrConnect)
				}
				return m, goTo(scrClone)
			}
		case "g":
			m.vp.GotoTop()
			return m, nil
		case "G":
			m.vp.GotoBottom()
			return m, nil
		}

		// Arrows, j/k, PgUp/PgDn, space, u/d: the viewport handles scrolling.
		if m.state == statusReady {
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *statusScreen) View() string {
	switch m.state {
	case statusLoading:
		return "\n  " + m.spinner.View() + " Reading workspace…"

	case statusNotWorkspace:
		return "\n" +
			"  " + statusTitleStyle.Render("Not a lectern workspace") + "\n\n" +
			"  " + statusMutedStyle.Render(tildify(m.dir)+" isn't inside a cloned folder.") + "\n" +
			"  Run Clone to create one, or start lectern from inside a workspace."

	case statusFailed:
		errText := lipgloss.NewStyle().Width(max(10, m.width-4)).Render(m.err.Error())
		return "\n" +
			"  " + statusTitleStyle.Render("✗ Couldn't read the workspace") + "\n\n" +
			lipgloss.NewStyle().PaddingLeft(2).Render(errText) + "\n\n" +
			"  Press r to try again."
	}

	return lipgloss.JoinVertical(lipgloss.Left, m.renderSummary(), m.separator(), m.vp.View())
}

func (m *statusScreen) ShortHelp() []key.Binding {
	switch m.state {
	case statusReady:
		return []key.Binding{statusKeyScroll, statusKeyRefresh, statusKeyBack}
	case statusNotWorkspace:
		return []key.Binding{statusKeyClone, statusKeyRefresh, statusKeyBack}
	case statusFailed:
		return []key.Binding{statusKeyRefresh, statusKeyBack}
	default:
		return []key.Binding{statusKeyBack}
	}
}

// renderSummary is the fixed block above the course list.
func (m *statusScreen) renderSummary() string {
	if m.report == nil {
		return ""
	}

	r := m.report
	valueWidth := max(10, m.width-16)
	row := func(label, value string) string {
		return "  " + statusMutedStyle.Render(fmt.Sprintf("%-12s", label)) + truncate(value, valueWidth)
	}

	last := r.State.LastSync
	return strings.Join([]string{
		"",
		row("Workspace", tildify(r.Root)),
		row("Last sync", fmt.Sprintf("%s (%s)", last.Format("2006-01-02 15:04"), core.HumanAgo(last))),
		row("Files", fmt.Sprintf("%d across %d courses", r.TotalFiles, len(r.Courses))),
	}, "\n")
}

// separator is a rule under the summary, with the scroll position on the
// right when the course list doesn't fit.
func (m *statusScreen) separator() string {
	width := max(0, m.width-4)
	label := ""
	if m.vp.TotalLineCount() > m.vp.Height {
		label = fmt.Sprintf(" %3.0f%%", m.vp.ScrollPercent()*100)
	}
	rule := strings.Repeat("─", max(0, width-lipgloss.Width(label)))
	return "  " + statusMutedStyle.Render(rule+label)
}

// renderCourses builds the scrollable content. It's called on load and on
// resize only, never from View.
func (m *statusScreen) renderCourses(width int) string {
	if len(m.report.Courses) == 0 {
		return "\n  " + statusMutedStyle.Render("No courses in this workspace.")
	}

	inner := max(10, width-4) // 2 columns of margin on each side
	var b strings.Builder

	for _, c := range m.report.Courses {
		b.WriteString("\n")

		title := c.DisplayName
		if c.ShortName != "" {
			title = c.ShortName + " — " + c.DisplayName
		}
		count := fmt.Sprintf("%d files", len(c.Files))
		titleWidth := max(1, inner-lipgloss.Width(count)-1)
		b.WriteString("  " + statusTitleStyle.Width(titleWidth).Render(truncate(title, titleWidth)) +
			" " + statusMutedStyle.Render(count) + "\n")

		location := c.Path
		if rel, err := filepath.Rel(m.report.Root, c.Path); err == nil {
			location = rel
		}
		info := fmt.Sprintf("%s/ · %d sections", location, c.Sections)
		b.WriteString("    " + statusMutedStyle.Render(truncate(info, inner-2)) + "\n")

		if len(c.Files) == 0 {
			b.WriteString("      " + statusMutedStyle.Render("no files") + "\n")
			continue
		}
		b.WriteString(fileColumns(c.Files, inner-4, "      "))
	}

	return strings.TrimRight(b.String(), "\n")
}

// fileColumns lays names out in as many columns as fit in width. Like ls,
// each column is as wide as its own longest name, so names are only
// truncated when one is wider than the whole line.
func fileColumns(names []string, width int, indent string) string {
	const gap = 3
	width = max(1, width)

	widths := make([]int, len(names))
	for i, n := range names {
		widths[i] = lipgloss.Width(n)
	}

	// Try the most columns first and keep the first layout that fits.
	cols, colWidths := 1, []int{width}
	for c := len(names); c > 1; c-- {
		cw := make([]int, c)
		for i, w := range widths {
			cw[i%c] = max(cw[i%c], w)
		}
		total := gap * (c - 1)
		for _, w := range cw {
			total += w
		}
		if total <= width {
			cols, colWidths = c, cw
			break
		}
	}

	var b strings.Builder
	for i := 0; i < len(names); i += cols {
		var row []string
		for j, n := range names[i:min(i+cols, len(names))] {
			row = append(row, lipgloss.NewStyle().Width(colWidths[j]).Render(truncate(n, colWidths[j])))
		}
		b.WriteString(indent + strings.TrimRight(strings.Join(row, strings.Repeat(" ", gap)), " ") + "\n")
	}
	return b.String()
}

// truncate shortens s to at most width cells, ending with "…" when cut.
func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// tildify replaces the home directory prefix with "~".
func tildify(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home || strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + path[len(home):]
	}
	return path
}
