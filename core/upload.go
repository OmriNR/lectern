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

// UploadResult says how far UploadSubmission got once the file was saved.
type UploadResult struct {
	Assignment  Assignment
	Submitted   bool   // the assignment is now submitted (closed)
	NeedsMoodle string // why the user must finish on the website; empty if not
}

// UploadSubmission uploads the file at path as the user's submission for a,
// replacing any files already in it, then submits it for grading when lectern
// can. An error means the file wasn't saved; anything after that is reported
// in the result.
func (s *Service) UploadSubmission(a Assignment, path string) (UploadResult, error) {
	if _, err := CheckUploadFile(path); err != nil {
		return UploadResult{Assignment: a}, err
	}

	itemID, err := s.client.UploadDraftFile(path)
	if err != nil {
		return UploadResult{Assignment: a}, fmt.Errorf("upload: %w", err)
	}

	if err := s.client.SaveFileSubmission(a.Info.ID, itemID); err != nil {
		return UploadResult{Assignment: a}, fmt.Errorf("saving submission: %w", err)
	}

	a = s.refreshStatus(a)
	result := UploadResult{Assignment: a}

	switch {
	case a.Info.SubmissionsDrafts == 0:
		// Without drafts, saving is submitting.
		result.Submitted = true
		return result, nil
	case a.Info.RequireSubmissionStatement == 1:
		result.NeedsMoodle = "this assignment needs you to accept a submission statement"
		return result, nil
	case !a.Status.LastAttempt.CanSubmit:
		result.NeedsMoodle = "Moodle won't let lectern submit this one"
		return result, nil
	}

	if err := s.client.SubmitForGrading(a.Info.ID); err != nil {
		result.NeedsMoodle = "submitting failed: " + err.Error()
		return result, nil
	}

	result.Assignment = s.refreshStatus(a)
	result.Submitted = true
	return result, nil
}

// refreshStatus reloads a's submission status, keeping the old one if the
// reload fails: the upload already happened, so that's not worth an error.
func (s *Service) refreshStatus(a Assignment) Assignment {
	if status, err := s.client.GetSubmissionStatus(a.Info.ID); err == nil {
		a.Status = *status
	}
	return a
}
