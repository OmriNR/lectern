package core

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

const eventLength = time.Hour

var defaultReminders = []time.Duration{24 * time.Hour, time.Hour}

type calendarEvent struct {
	UID         string
	Title       string
	Description string
	URL         string
	Start       time.Time
	End         time.Time
	Remiders    []time.Duration
}

type SyncResult struct {
	Added     int
	Updated   int
	Unchanged int
}

type CalendarProvider interface {
	Name() string
	Connect(ctx context.Context) error
	Upsert(ctx context.Context, events []calendarEvent) (SyncResult, error)
}

func (a Assignment) IsSchedulable(now time.Time) bool {
	due := a.DueDate()

	if due.IsZero() || !due.After(now) {
		return false
	}

	switch a.State(now) {
	case AssignmentSubmitted, AssignmentSubmittedLate, AssignmentGraded:
		return false
	}

	return true
}

func (a Assignment) Event(host string) calendarEvent {
	due := a.DueDate()
	link := fmt.Sprintf("%s/mod/assign/view.php?id=%d", host, a.Info.CMID)

	return calendarEvent{
		UID:         fmt.Sprintf("assign-%d@%s", a.Info.CMID, siteName(host)),
		Title:       fmt.Sprintf("[%s] %s", a.CourseName, a.Info.Name),
		Description: fmt.Sprintf("Course: %s\nDue: %s\n%s", a.CourseName, due.Format("Mon 02 Jan 15:04"), link),
		URL:         link,
		Start:       due.Add(-eventLength),
		End:         due,
		Remiders:    defaultReminders,
	}
}

func AssignmentEvents(host string, assignments []Assignment, now time.Time) []calendarEvent {
	var events []calendarEvent

	for _, a := range assignments {
		if a.IsSchedulable(now) {
			events = append(events, a.Event(host))
		}
	}

	return events
}

func siteName(host string) string {
	if u, err := url.Parse(host); err == nil && u.Host != "" {
		return u.Host
	}
	return host
}
