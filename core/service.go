package core

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
)

// This file holds lectern's operations without any terminal I/O, so both the
// plain subcommands and the interactive TUI can drive them.

var (
	ErrNotConnected = errors.New("not connected. Run 'connect' first")
	ErrInvalidLogin = errors.New("wrong username or password")
	ErrUnreachable  = errors.New("can't reach the Moodle site")
)

// Service is lectern's application layer: a Moodle client plus the local
// workspace manager.
type Service struct {
	client    *MoodleClient
	workspace *WorkspaceManager
}

func NewService(client *MoodleClient) *Service {
	return &Service{client: client, workspace: NewWorkspaceManager()}
}

func (s *Service) IsConnected() bool {
	return s.client != nil && s.client.token != ""
}

func (s *Service) Host() string {
	return s.client.Host()
}

func (s *Service) Username() string {
	if !s.IsConnected() || s.client.user == nil {
		return ""
	}
	return s.client.user.Username
}

// connect logs in to Moodle and persists the session to the global config.
func (s *Service) Connect(username, password string) error {
	if err := s.client.Login(username, password); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	cfg := &GlobalConfig{
		BaseURL:  s.client.Host(),
		Email:    s.client.user.Email,
		Username: s.client.user.Username,
		UserID:   s.client.user.ID,
		Token:    s.client.token,
	}

	if err := SaveConfig(cfg); err != nil {
		return loginError(s.client.Host(), err)
	}
	return nil
}

func loginError(host string, err error) error {
	var me *moodleError
	if errors.As(err, &me) && me.ErrorCode == "invalidlogin" {
		return ErrInvalidLogin
	}

	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%w at %s", ErrUnreachable, host)
	}

	return fmt.Errorf("failed to connect: %w", err)
}

func ResolveCloneTarget(path string) (string, error) {
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

func (s *Service) FetchCourses() ([]Course, error) {
	if !s.IsConnected() {
		return nil, ErrNotConnected
	}

	courses, err := s.client.GetUserCourses()
	if err != nil {
		return nil, fmt.Errorf("fetching courses: %w", err)
	}
	return courses, nil
}

// CloneCourses downloads the given courses into targetDir. onEvent may be nil.
func (s *Service) CloneCourses(targetDir string, courses []Course, onEvent func(CloneEvent)) error {
	if err := s.workspace.InitWorkspace(targetDir, courses, s.client, onEvent); err != nil {
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

// LoadStatus finds the workspace containing startDir and summarizes it.
func (s *Service) LoadStatus(startDir string) (*StatusReport, error) {
	root, err := s.workspace.FindRoot(startDir)
	if err != nil {
		return nil, fmt.Errorf("couldn't find root: %w", err)
	}

	state, err := s.workspace.LoadState(root)
	if err != nil {
		return nil, fmt.Errorf("loading sync file: %w", err)
	}

	report := &StatusReport{
		Root:  root,
		State: state,
	}

	for _, course := range state.Courses {
		var names []string
		for _, sec := range course.Sections {
			for _, f := range sec.Files {
				names = append(names, filepath.Base(f.Path))
			}
		}
		sort.Strings(names)

		report.TotalFiles += len(names)
		report.Courses = append(report.Courses, CourseReport{CourseState: course, Files: names})
	}

	sort.Slice(report.Courses, func(i, j int) bool {
		return report.Courses[i].DisplayName < report.Courses[j].DisplayName
	})

	return report, nil
}

type SyncReport struct {
	Root                   string
	State                  *WorkspaceState
	NewCourses             []Course
	CoursesWithNewSections []Course
}

func (s *Service) CheckSync(startDir string) (*SyncReport, error) {
	root, err := s.workspace.FindRoot(startDir)
	if err != nil {
		return nil, fmt.Errorf("couldn't find root: %w", err)
	}

	state, err := s.workspace.LoadState(root)
	if err != nil {
		return nil, fmt.Errorf("loading sync file: %w", err)
	}

	courses, err := s.FetchCourses()
	if err != nil {
		return nil, err
	}

	report := &SyncReport{Root: root, State: state}

	for _, course := range courses {
		if s.workspace.CheckIfCourseNew(state, course) {
			report.NewCourses = append(report.NewCourses, course)
		} else if s.workspace.CheckIfHasNewSections(state, course) {
			report.CoursesWithNewSections = append(report.CoursesWithNewSections, course)
		}
	}

	return report, nil
}

func (s *Service) ClearConfig() error {
	if err := ClearConfig(); err != nil {
		return fmt.Errorf("clearing config: %w", err)
	}

	s.client.RestoreSession("", nil)
	return nil
}
