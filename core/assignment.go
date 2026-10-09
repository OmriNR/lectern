package core

import "time"

type AssignmentStatus int

const (
	AssignmentOpen AssignmentStatus = iota
	AssignmentNotOpen
	AssignmentDraft
	AssignmentOverDue
	AssignmentMissed
	AssignmentSubmitted
	AssignmentSubmittedLate
	AssignmentGraded
	AssignmentNoSubmission
)

func (s AssignmentStatus) String() string {
	switch s {
	case AssignmentNotOpen:
		return "not open yet"
	case AssignmentDraft:
		return "draft"
	case AssignmentOverDue:
		return "overdue"
	case AssignmentMissed:
		return "missed"
	case AssignmentSubmitted:
		return "submitted"
	case AssignmentSubmittedLate:
		return "submitted late"
	case AssignmentGraded:
		return "graded"
	case AssignmentNoSubmission:
		return "no online submission"
	default:
		return "open"
	}
}

type Assignment struct {
	CourseID   int
	CourseName string
	Path       string
	Info       AssignmentInfo
	Status     SubmissionStatus
}

func unixTime(ts int64) time.Time {
	if ts == 0 {
		return time.Time{}
	}

	return time.Unix(ts, 0)
}

func (a Assignment) DueDate() time.Time {
	if ext := a.Status.LastAttempt.ExtensionDueDate; ext != nil && *ext > 0 {
		return unixTime(*ext)
	}

	return unixTime(a.Info.DueDate)
}

func (a Assignment) CutOffDate() time.Time {
	return unixTime(a.Info.CutOffDate)
}

func (a Assignment) OpenAt() time.Time {
	return unixTime(a.Info.AllowSubmissionsFromDate)
}

func (a Assignment) Submission() *Submission {
	if a.Info.TeamSubmission == 1 {
		return a.Status.LastAttempt.TeamSubmission
	}

	return a.Status.LastAttempt.Submission
}

func (a Assignment) SubmissionState() string {
	if sub := a.Submission(); sub != nil {
		return sub.Status
	}

	return "new"
}

func (a Assignment) LateBy() time.Duration {
	sub := a.Submission()
	due := a.DueDate()

	if sub == nil || sub.Status != "submitted" || due.IsZero() {
		return 0
	}

	if late := unixTime(sub.TimeModified).Sub(due); late > 0 {
		return late
	}

	return 0
}

func (a Assignment) IsPastDue(now time.Time) bool {
	due := a.DueDate()
	return !due.IsZero() && now.After(due)
}

func (a Assignment) Grade() string {
	if a.Status.Feedback == nil {
		return ""
	}

	return a.Status.Feedback.GradeForDisplay
}

func (a Assignment) State(now time.Time) AssignmentStatus {
	if a.Status.Feedback != nil {
		return AssignmentGraded
	}

	if !a.Status.LastAttempt.SubmissionsEnabled {
		return AssignmentNoSubmission
	}

	switch a.SubmissionState() {
	case "submitted":
		if a.LateBy() > 0 {
			return AssignmentSubmittedLate
		}
		return AssignmentSubmitted

	case "draft":
		return AssignmentDraft
	}

	cutoff := a.CutOffDate()
	pastCutoff := !cutoff.IsZero() && now.After(cutoff)

	if !a.Status.LastAttempt.CanEdit && (a.IsPastDue(now) || pastCutoff) {
		return AssignmentMissed
	}

	if a.IsPastDue(now) {
		return AssignmentOverDue
	}

	if opens := a.OpenAt(); !opens.IsZero() && now.Before(opens) {
		return AssignmentNotOpen
	}

	return AssignmentOpen
}
