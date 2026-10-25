// Package core holds the types and interfaces shared across Soundcheck.
package core

import "time"

// Event is a concert in the shape the rest of the app works with,
// independent of which source it came from.
type Event struct {
	ID       string
	Name     string
	Venue    string
	City     string
	URL      string
	StartsAt time.Time
}
