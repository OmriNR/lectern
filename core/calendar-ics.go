package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/emersion/go-ical"
)

const icsFileName = "assignments.ics"

type ICSProvider struct {
	Path string
}

func NewICSProvider() (*ICSProvider, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	return &ICSProvider{Path: filepath.Join(filepath.Dir(configPath), icsFileName)}, nil
}

func (p *ICSProvider) Name() string {
	return "Local calendar (.ics)"
}

func (p *ICSProvider) Connect(ctx context.Context) error {
	return nil
}

func (p *ICSProvider) Upsert(ctx context.Context, events []calendarEvent) (SyncResult, error) {
	var result SyncResult

	previous, err := p.readEvents()
	if err != nil {
		return result, err
	}

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.SetText(ical.PropProductID, "-//lectern//assignments//EN")

	now := time.Now()

	for _, ev := range events {
		old, ok := previous[ev.UID]
		switch {
		case !ok:
			result.Added++
		case old.Start.Equal(ev.Start) && old.End.Equal(ev.End) && old.Title == ev.Title:
			result.Unchanged++
		default:
			result.Updated++
		}

		cal.Children = append(cal.Children, toICalEvent(ev, now).Component)
	}

	return result, p.write(cal)
}

func (p *ICSProvider) Open() error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", p.Path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll.FileProtocolHandler", p.Path)
	default:
		cmd = exec.Command("xdg-open", p.Path)
	}

	return cmd.Start()
}

func toICalEvent(ev calendarEvent, now time.Time) *ical.Event {
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, ev.UID)
	event.Props.SetDateTime(ical.PropDateTimeStamp, now.UTC())
	event.Props.SetDateTime(ical.PropDateTimeStart, ev.Start.UTC())
	event.Props.SetDateTime(ical.PropDateTimeEnd, ev.End.UTC())
	event.Props.SetText(ical.PropSummary, ev.Title)
	event.Props.SetText(ical.PropDescription, ev.Description)

	if u, err := url.Parse(ev.URL); err == nil {
		event.Props.SetURI(ical.PropURL, u)
	}

	for _, before := range ev.Remiders {
		alarm := ical.NewComponent(ical.CompAlarm)
		alarm.Props.SetText(ical.PropAction, "DISPLAY")
		alarm.Props.SetText(ical.PropDescription, ev.Title)

		trigger := ical.NewProp(ical.PropTrigger)
		trigger.SetDuration(-before)
		alarm.Props.Set(trigger)

		event.Children = append(event.Children, alarm)
	}

	return event
}

func (p *ICSProvider) readEvents() (map[string]calendarEvent, error) {
	events := map[string]calendarEvent{}

	f, err := os.Open(p.Path)
	if errors.Is(err, os.ErrNotExist) {
		return events, nil
	}

	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", p.Path, err)
	}
	defer f.Close()

	cal, err := ical.NewDecoder(f).Decode()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", p.Path, err)
	}

	for _, e := range cal.Events() {
		uid, _ := e.Props.Text(ical.PropUID)
		title, _ := e.Props.Text(ical.PropSummary)
		start, _ := e.DateTimeStart(time.Local)
		end, _ := e.DateTimeEnd(time.Local)

		events[uid] = calendarEvent{UID: uid, Title: title, Start: start, End: end}
	}

	return events, nil
}

func (p *ICSProvider) write(cal *ical.Calendar) error {
	tmp, err := os.CreateTemp(filepath.Dir(p.Path), icsFileName+".*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if err := ical.NewEncoder(tmp).Encode(cal); err != nil {
		tmp.Close()
		return fmt.Errorf("writing calendar: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmp.Name(), p.Path)
}
