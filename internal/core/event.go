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
}
