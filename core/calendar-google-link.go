package core

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

const (
	googleTemplateURL = "https://calendar.google.com/calendar/render"
	googleDateLayout  = "20060102T150405Z"
	maxGoogleTabs     = 10
	tabOpenDelay      = 300 * time.Millisecond
)

type GoogleLinkProvider struct{}

func (p *GoogleLinkProvider) Name() string {
	return "Google Calendar (browser)"
}

func (p *GoogleLinkProvider) Connect(ctx context.Context) error {
	return nil
}

func (p *GoogleLinkProvider) Upsert(ctx context.Context, events []calendarEvent) (SyncResult, error) {
	var result SyncResult

	if len(events) > maxGoogleTabs {
		return result, fmt.Errorf("that would open %d browser tabs (max %d); export a .ics file instead", len(events), maxGoogleTabs)
	}

	for i, ev := range events {
		if i > 0 {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(tabOpenDelay):
			}
		}

		if err := openExternal(googleCalendarLink(ev)); err != nil {
			return result, fmt.Errorf("opening browser: %w", err)
		}
		result.Added++
	}

	return result, nil
}

func googleCalendarLink(ev calendarEvent) string {
	q := url.Values{}
	q.Set("action", "TEMPLATE")
	q.Set("text", ev.Title)
	q.Set("dates", ev.Start.UTC().Format(googleDateLayout)+"/"+ev.End.UTC().Format(googleDateLayout))
	q.Set("details", ev.Description)

	return googleTemplateURL + "?" + q.Encode()
}
