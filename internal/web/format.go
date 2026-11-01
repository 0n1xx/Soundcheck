package web

import (
	"html/template"
	"time"

	"github.com/0n1xx/soundcheck/internal/core"
)

// templateFuncs are helpers the page templates can call, e.g. {{showDate .}}.
// Templates can't do real logic, so the work happens here in Go.
var templateFuncs = template.FuncMap{
	"showDate":   showDate,
	"showStatus": showStatus,
}

// ticketStatus is what the Tickets column shows: a label and a CSS modifier.
type ticketStatus struct {
	Label string
	Class string // "", "status--soon" or "status--off"
}

// venueTime converts t to the venue's local time.
// If the time zone is unknown, UTC is used.
func venueTime(t time.Time, timezone string) time.Time {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return t.UTC()
	}
	return t.In(loc)
}

// showDate formats a concert's start in the venue's local time:
// "Thu, Feb 11 · 20:00", with the year added if it isn't this year.
func showDate(e core.Event) string {
	if e.StartsAt.IsZero() {
		return "Date TBA"
	}
	t := venueTime(e.StartsAt, e.Timezone)

	layout := "Mon, Jan 2"
	if t.Year() != time.Now().Year() {
		layout += ", 2006"
	}
	// Midnight means only the date was announced (see ticketmaster.toEvent).
	if t.Hour() == 0 && t.Minute() == 0 {
		return t.Format(layout)
	}
	return t.Format(layout + " · 15:04")
}

// showStatus decides what the Tickets column says and in which colour.
func showStatus(e core.Event) ticketStatus {
	switch e.Status {
	case "cancelled":
		return ticketStatus{Label: "Cancelled", Class: "status--off"}
	case "offsale":
		return ticketStatus{Label: "Off sale", Class: "status--off"}
	case "postponed":
		return ticketStatus{Label: "Postponed", Class: "status--soon"}
	case "rescheduled":
		return ticketStatus{Label: "Rescheduled", Class: "status--soon"}
	}

	// Sales haven't started yet: say when, in the venue's local time.
	if e.OnSaleAt.After(time.Now()) {
		t := venueTime(e.OnSaleAt, e.Timezone)
		return ticketStatus{Label: "On sale " + t.Format("Mon, Jan 2 · 15:04"), Class: "status--soon"}
	}
	return ticketStatus{Label: "On sale now"}
}
