// Package core holds the types and interfaces shared across Soundcheck.
package core

import "time"

// Event is a concert in the shape the rest of the app works with,
// independent of which source it came from.
type Event struct {
	ID       string
	ArtistID string // Ticketmaster ID of the artist this concert was found for
	Name     string
	Venue    string
	City     string
	URL      string
	StartsAt time.Time // zero if the date isn't announced yet
	OnSaleAt time.Time // when public ticket sales start; zero if unknown
	Status   string    // "onsale", "offsale", "cancelled", "rescheduled", "postponed"...
	Timezone string    // the venue's time zone, e.g. "America/Toronto"; "" if unknown
}

// VenueTime converts t to the venue's local time.
// If the time zone is unknown, UTC is used.
func (e Event) VenueTime(t time.Time) time.Time {
	return t.In(Location(e.Timezone))
}

// DateText is the start in the venue's local time, e.g. "Thu, Feb 11 · 20:00",
// with the year added if it isn't this year. Used by the website and alerts.
func (e Event) DateText() string {
	if e.StartsAt.IsZero() {
		return "Date TBA"
	}
	t := e.VenueTime(e.StartsAt)

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

// Place is "Venue, City", or whichever of the two is known.
func (e Event) Place() string {
	switch {
	case e.Venue != "" && e.City != "":
		return e.Venue + ", " + e.City
	case e.Venue != "":
		return e.Venue
	default:
		return e.City
	}
}
