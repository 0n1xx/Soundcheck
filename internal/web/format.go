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

// showDate formats a concert's start in the venue's local time.
func showDate(e core.Event) string {
	return e.DateText()
}

// showStatus decides what the Tickets column says and in which colour.
func showStatus(e core.Event) ticketStatus {
	switch e.Status {
	case "cancelled":
		return ticketStatus{Label: "Cancelled", Class: "status--off"}
	case "postponed":
		return ticketStatus{Label: "Postponed", Class: "status--soon"}
	case "rescheduled":
		return ticketStatus{Label: "Rescheduled", Class: "status--soon"}
	}

	// Sales haven't started yet: say when, in the venue's local time.
	// This comes before "offsale", because Ticketmaster also uses
	// "offsale" for shows whose tickets aren't on sale *yet*.
	if e.OnSaleAt.After(time.Now()) {
		t := e.VenueTime(e.OnSaleAt)
		return ticketStatus{Label: "On sale " + t.Format("Mon, Jan 2 · 15:04"), Class: "status--soon"}
	}
	if e.Status == "offsale" {
		return ticketStatus{Label: "Off sale", Class: "status--off"}
	}
	return ticketStatus{Label: "On sale now"}
}
