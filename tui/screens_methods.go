package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

func renderCursorList(items [][2]string, cursor int, sess *session) string {
	rows := max(1, sess.height-10)
	start, end := 0, len(items)

	if len(items) > rows {
		start = min(max(0, cursor-rows/2), len(items)-rows)
		end = start + rows
	}

	nameWidth := max(10, min(40, sess.width-20))
	nameStyle := lipgloss.NewStyle().Width(nameWidth)

	var lines []string
	if start > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf(" ...%d more", start)))
	}

	for i := start; i < end; i++ {
		pointer, style := " ", nameStyle.Foreground(colorText)
		if i == cursor {
			pointer, style = accentStyle.Render("> "), nameStyle.Foreground(colorOrange)
		}
		lines = append(lines, pointer+style.Render(truncate(items[i][0], nameWidth))+" "+mutedStyle.Render(items[i][1]))
	}

	if hidden := len(items) - end; hidden > 0 {
		lines = append(lines, mutedStyle.Render(fmt.Sprintf(" ... %d more", hidden)))
	}

	return strings.Join(lines, "\n")
}
