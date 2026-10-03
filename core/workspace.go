package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	MetaDirName = ".moodle"
	StateFIleName = "state.json"
)

type FileState struct {
	ID 				int 	`json:"id"`
	Name 			string 	`json:"name"`
	TimeModified	int64	`json:"time_modified"`
	Path 			string	`json:"path"`
	CourseID		int		`json:"course_id"`
}

type CourseState struct {
	ID          int    `json:"id"`
	ShortName   string `json:"short_name"`
	DisplayName string `json:"display_name"`
	Path        string `json:"path"`
	Sections    int    `json:"sections"`
}

type WorkspaceState struct {
	LastSync	time.Time			`json:"last_sync"`
	Files 		map[int]FileState	`json:"files"`
	Courses		[]CourseState		`json:"courses"`
}

// ErrNoWorkspace means no .moodle folder was found in the directory or any of
// its parents.
var ErrNoWorkspace = errors.New("not a lectern workspace (no .moodle folder)")

type WorkspaceManager struct {}

func NewWorkspaceManager() *WorkspaceManager {
	return  &WorkspaceManager{}
}

// FileDownloader fetches the raw content of a Moodle file URL. MoodleClient
// satisfies this interface.
type FileDownloader interface {
	DownloadFile(fileURL string) ([]byte, error)
}

type CloneEventKind int

const (
	CloneCourseCreated CloneEventKind = iota
	CloneSectionCreated
	CloneFileDownloaded
)

// CloneEvent reports InitWorkspace progress. FilesDone/FilesTotal let callers
// render a progress bar.
type CloneEvent struct {
	Kind       CloneEventKind
	Course     string
	Section    string
	File       string
	FilesDone  int
	FilesTotal int
}

func (w *WorkspaceManager) SanitizeName (name string) string {
	name = strings.TrimSpace(name)

	invalidChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	clean := invalidChars.ReplaceAllString(name, "_")
	clean = strings.TrimRight(clean, ". ")

	if clean == "" {
		return "unamed"
	}

	return clean
}

func (w *WorkspaceManager) InitWorkspace(targetDir string, courses []Course, downloader FileDownloader, onEvent func(CloneEvent)) error {
	if onEvent == nil {
		onEvent = func(CloneEvent) {}
	}

	filesTotal := 0
	for _, course := range courses {
		for _, sec := range course.Sections {
			filesTotal += len(sec.GetFiles())
		}
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("Failed creating dest folder: %w", err)
	}

	metaPath := filepath.Join(targetDir, MetaDirName)
	if err := os.MkdirAll(metaPath, 0755); err != nil {
		return fmt.Errorf("Failed create meta-data folder: %w", err)
	}

	initialState := WorkspaceState {
		LastSync: time.Now(),
		Files: make(map[int]FileState),
	}

	nextFileID := 1

	for _, course := range courses {
		courseFolder := w.SanitizeName(course.DisplayName)
		coursePath := filepath.Join(targetDir, courseFolder)

		if err := os.MkdirAll(coursePath, 0755); err != nil {
			return fmt.Errorf("Failed creating course %s: %w", course.DisplayName, err)
		}

		onEvent(CloneEvent{Kind: CloneCourseCreated, Course: courseFolder, FilesDone: nextFileID - 1, FilesTotal: filesTotal})

		courseState := CourseState{
			ID:          course.ID,
			ShortName:   course.ShortName,
			DisplayName: course.DisplayName,
			Path:        coursePath,
		}

		for i, sec := range course.Sections {
			secTitle := strings.TrimSpace(sec.Name)
			if secTitle == "" {
				secTitle = fmt.Sprintf("section_%d", i+1)
			}

			secFolder := fmt.Sprintf("%02d_%s", i+1, w.SanitizeName(secTitle))
			secPath := filepath.Join(coursePath, secFolder)

			if err := os.MkdirAll(secPath, 0755); err != nil {
				return fmt.Errorf("Failed creating section %s: %w", secFolder, err)
			}

			onEvent(CloneEvent{Kind: CloneSectionCreated, Course: courseFolder, Section: secFolder, FilesDone: nextFileID - 1, FilesTotal: filesTotal})

			courseState.Sections++

			for _, file := range sec.GetFiles() {
				fileName := w.SanitizeName(file.FileName)
				filePath := filepath.Join(secPath, fileName)

				data, err := downloader.DownloadFile(file.FileURL)
				if err != nil {
					return fmt.Errorf("Failed downloading file %s: %w", file.FileName, err)
				}

				if err := os.WriteFile(filePath, data, 0644); err != nil {
					return fmt.Errorf("Failed writing file %s: %w", file.FileName, err)
				}

				initialState.Files[nextFileID] = FileState{
					ID:           nextFileID,
					Name:         file.FileName,
					TimeModified: file.TimeModified,
					Path:         filePath,
					CourseID:     course.ID,
				}
				nextFileID++

				onEvent(CloneEvent{Kind: CloneFileDownloaded, Course: courseFolder, Section: secFolder, File: fileName, FilesDone: nextFileID - 1, FilesTotal: filesTotal})
			}
		}

		initialState.Courses = append(initialState.Courses, courseState)
	}

	if err := w.SaveState(targetDir, &initialState); err != nil {
		return fmt.Errorf("Failed saving workspace state: %w", err)
	}

	return nil
}

func (w *WorkspaceManager) FindRoot(startDIr string) (string, error) {
	curr := startDIr

	for {
		metaPath := filepath.Join(curr, MetaDirName)
		info, err := os.Stat(metaPath)

		if err == nil && info.IsDir() {
			return curr, nil
		}

		parent := filepath.Dir(curr)
		if parent == curr {
			return "", ErrNoWorkspace
		}
		curr = parent
	}
}

func (w *WorkspaceManager) LoadState(rootDIr string) (*WorkspaceState, error) {
	statePath := filepath.Join(rootDIr, MetaDirName, StateFIleName)
	data, err := os.ReadFile(statePath)

	if err != nil {
		return nil, err
	}

	var state WorkspaceState

	if err := json.Unmarshal(data, &state); err != nil {
		return  nil, err
	}

	return &state, nil
}

func (w *WorkspaceManager) SaveState(rootDIr string, state *WorkspaceState) error {
	statePath := filepath.Join(rootDIr, MetaDirName, StateFIleName)
	data, err := json.MarshalIndent(state, "", " ")

	if err != nil {
		return err
	}

	return os.WriteFile(statePath, data, 0644)
}