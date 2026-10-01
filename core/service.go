package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// This file holds lectern's operations without any terminal I/O, so both the
// plain subcommands and the interactive TUI can drive them.

var ErrNotConnected = errors.New("not connected. Run 'connect' first")

func (c *CLI) isConnected() bool {
	return c.client != nil && c.client.token != ""
}

// connect logs in to Moodle and persists the session to the global config.
func (c *CLI) connect(username, password string) error {
	if err := c.client.Login(username, password); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	cfg := &GlobalConfig{
		BaseURL:  c.client.Host(),
		Email:    c.client.user.Email,
		Username: c.client.user.Username,
		UserID:   c.client.user.ID,
		Token:    c.client.token,
	}

	if err := SaveConfig(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// resolveCloneTarget turns an optional user-supplied path into an absolute
// target directory, defaulting to the current directory.
func resolveCloneTarget(path string) (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("recognizing current folder: %w", err)
	}

	if path == "" {
		return currentDir, nil
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(currentDir, path), nil
}

func (c *CLI) fetchCourses() ([]Course, error) {
	if !c.isConnected() {
		return nil, ErrNotConnected
	}

	courses, err := c.client.GetUserCourses()
	if err != nil {
		return nil, fmt.Errorf("fetching courses: %w", err)
	}
	return courses, nil
}

// cloneCourses downloads the given courses into targetDir. onEvent may be nil.
func (c *CLI) cloneCourses(targetDir string, courses []Course, onEvent func(CloneEvent)) error {
	if err := c.workspace.InitWorkspace(targetDir, courses, c.client, onEvent); err != nil {
		return fmt.Errorf("setting up workspace: %w", err)
	}
	return nil
}

type CourseReport struct {
	CourseState
	Files []string // sorted base names
}

type StatusReport struct {
	Root       string
	State      *WorkspaceState
	Courses    []CourseReport
	TotalFiles int
}

// loadStatus finds the workspace containing startDir and summarizes it.
func (c *CLI) loadStatus(startDir string) (*StatusReport, error) {
	root, err := c.workspace.FindRoot(startDir)
	if err != nil {
		return nil, fmt.Errorf("couldn't find root: %w", err)
	}

	state, err := c.workspace.LoadState(root)
	if err != nil {
		return nil, fmt.Errorf("loading sync file: %w", err)
	}

	report := &StatusReport{
		Root:       root,
		State:      state,
		TotalFiles: len(state.Files),
	}

	for _, course := range state.Courses {
		var names []string
		for _, f := range state.Files {
			if f.CourseID == course.ID {
				names = append(names, filepath.Base(f.Path))
			}
		}
		sort.Strings(names)

		report.Courses = append(report.Courses, CourseReport{CourseState: course, Files: names})
	}

	return report, nil
}

func (c *CLI) clearConfig() error {
	if err := ClearConfig(); err != nil {
		return fmt.Errorf("clearing config: %w", err)
	}

	c.client.RestoreSession("", nil)
	return nil
}
