package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	MetaDirName   = ".moodle"
	StateFIleName = "state.json"

	// StateVersion is bumped whenever the state.json layout changes.
	StateVersion = 2
)

type FileState struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	TimeModified int64  `json:"time_modified"`
	ModuleID     int64  `json:"module_id"`
	Path         string `json:"path"`
}

type SectionState struct {
	ID     int                  `json:"id"`
	Number int64                `json:"number"`
	Name   string               `json:"name"`
	Folder string               `json:"folder"`
	Files  map[string]FileState `json:"files"`
}

type CourseState struct {
	ID          int                     `json:"id"`
	ShortName   string                  `json:"short_name"`
	DisplayName string                  `json:"display_name"`
	Path        string                  `json:"path"`
	Sections    map[string]SectionState `json:"sections"`
}

type WorkspaceState struct {
	LastSync time.Time           `json:"last_sync"`
	Courses  map[int]CourseState `json:"courses"`
	Version  int64               `json:"version"`
}

type WorkspaceManager struct{}

func NewWorkspaceManager() *WorkspaceManager {
	return &WorkspaceManager{}
}

// FileDownloader fetches the raw content of a Moodle file URL. MoodleClient
// satisfies this interface.
type FileDownloader interface {
	DownloadFile(fileURL string) ([]byte, error)
}

// FileKey builds the stable key a file is stored under in SectionState.Files.
// Moodle has no single file ID here, and one module can hold several files,
// so the key combines the module ID with the file's path and name.
func FileKey(file SectionFile) string {
	return fmt.Sprintf("%d%s%s", file.ModuleID, file.Content.FilePath, file.Content.FileName)
}

func (w *WorkspaceManager) SanitizeName(name string) string {
	name = strings.TrimSpace(name)

	invalidChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	clean := invalidChars.ReplaceAllString(name, "_")
	clean = strings.TrimRight(clean, ". ")

	if clean == "" {
		return "unamed"
	}

	return clean
}

func (w *WorkspaceManager) InitWorkspace(targetDir string, courses []Course, downloader FileDownloader) error {

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("Failed creating dest folder: %w", err)
	}

	metaPath := filepath.Join(targetDir, MetaDirName)
	if err := os.MkdirAll(metaPath, 0755); err != nil {
		return fmt.Errorf("Failed create meta-data folder: %w", err)
	}

	initialWorkspace := WorkspaceState{
		LastSync: time.Now(),
		Courses:  make(map[int]CourseState),
		Version:  StateVersion,
	}

	for _, course := range courses {
		// Paths in the state are relative to targetDir, so the workspace
		// keeps working if its root folder is moved or renamed.
		courseFolder := w.SanitizeName(course.DisplayName)

		if err := os.MkdirAll(filepath.Join(targetDir, courseFolder), 0755); err != nil {
			return fmt.Errorf("Failed creating course %s: %w", course.DisplayName, err)
		}

		fmt.Printf("course created %s\n", courseFolder)

		courseState := CourseState{
			ID:          course.ID,
			ShortName:   course.ShortName,
			DisplayName: course.DisplayName,
			Path:        courseFolder,
			Sections:    make(map[string]SectionState),
		}

		for i, sec := range course.Sections {
			secTitle := strings.TrimSpace(sec.Name)
			if secTitle == "" {
				secTitle = fmt.Sprintf("section_%d", i+1)
			}

			secFolder := fmt.Sprintf("%02d_%s", i+1, w.SanitizeName(secTitle))
			secRelPath := filepath.Join(courseFolder, secFolder)

			if err := os.MkdirAll(filepath.Join(targetDir, secRelPath), 0755); err != nil {
				return fmt.Errorf("Failed creating section %s: %w", secFolder, err)
			}

			fmt.Printf("└── %s\n", secFolder)

			sectionState := SectionState{
				ID:     sec.ID,
				Number: int64(sec.Section),
				Name:   sec.Name,
				Folder: secFolder,
				Files:  make(map[string]FileState),
			}

			for _, file := range sec.GetFiles() {
				fileName := w.SanitizeName(file.Content.FileName)
				fileRelPath := filepath.Join(secRelPath, fileName)

				data, err := downloader.DownloadFile(file.Content.FileURL)
				if err != nil {
					return fmt.Errorf("Failed downloading file %s: %w", file.Content.FileName, err)
				}

				if err := os.WriteFile(filepath.Join(targetDir, fileRelPath), data, 0644); err != nil {
					return fmt.Errorf("Failed writing file %s: %w", file.Content.FileName, err)
				}

				sectionState.Files[FileKey(file)] = FileState{
					Name:         file.Content.FileName,
					TimeModified: file.Content.TimeModified,
					ModuleID:     int64(file.ModuleID),
					Path:         fileRelPath,
				}

				fmt.Printf("    - %s\n", fileName)
			}

			courseState.Sections[strconv.Itoa(sec.ID)] = sectionState
		}

		initialWorkspace.Courses[courseState.ID] = courseState
	}

	if err := w.SaveState(targetDir, &initialWorkspace); err != nil {
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
			return "", errors.New("Moodle workspace is not recognized (No .moodle folder)")
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
		return nil, err
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
