package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const MaxUploadSize = 5 * 1024 * 1024

func CheckUploadFile(path string) (int64, error) {
	info, err := os.Stat(path)

	if err != nil {
		return 0, err
	}

	if info.IsDir() {
		return 0, errors.New("that's a folder, pick a file")
	}

	if info.Size() > MaxUploadSize {
		return info.Size(), fmt.Errorf("%s is %s, the limit is %s",
			filepath.Base(path),
			HumanSize(info.Size()), HumanSize(MaxUploadSize))
	}

	return info.Size(), nil
}

// UploadSubmission uploads the file at path as the user's submission for a,
// replacing any files already in it, and returns a with its refreshed status.
func (s *Service) UploadSubmission(a Assignment, path string) (Assignment, error) {
	if _, err := CheckUploadFile(path); err != nil {
		return a, err
	}

	itemID, err := s.client.UploadDraftFile(path)
	if err != nil {
		return a, fmt.Errorf("upload: %w", err)
	}

	if err := s.client.SaveFileSubmission(a.Info.ID, itemID); err != nil {
		return a, fmt.Errorf("saving submission: %w", err)
	}

	status, err := s.client.GetSubmissionStatus(a.Info.ID)
	if err != nil {
		return a, fmt.Errorf("uploaded, but couldn't refresh the status: %w", err)
	}
	a.Status = *status

	return a, nil
}
