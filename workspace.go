package main

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
}

type WorkspaceState struct {
	LastSync	time.Time			`json:"last_sync"`
	Files 		map[int]FileState	`json:"files"`
}

type WorkspaceManager struct {}

func NewWorkspaceManager() *WorkspaceManager {
	return  &WorkspaceManager{}
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

func (w *WorkspaceManager) InitWorkspace(targetDir string, courses []Course) error {

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

	if err := w.SaveState(targetDir, &initialState); err != nil {
		return fmt.Errorf("Failed initial setup: %w", err)
	}

	for _, course := range courses {
		courseFolder := w.SanitizeName(course.DisplayName)
		coursePath := filepath.Join(targetDir, courseFolder)

		if err := os.MkdirAll(coursePath, 0755); err != nil {
			return fmt.Errorf("Failed creating course %s: %w", course.DisplayName, err)
		}

		fmt.Printf("course created %s\n", courseFolder)

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

			fmt.Printf("└── %s\n", secFolder)
		}
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