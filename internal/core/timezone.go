package core

import (
	"sync"
	"time"
)

// locations remembers time zones that were already loaded. LoadLocation
// reads a file from disk every time, and we need the same few zones for
// every concert on every page, so each one is loaded only once.
// sync.Map is safe to use from many goroutines at once (the website and
// the worker), unlike a plain map.
var locations sync.Map // time zone name → *time.Location

// Location returns the time zone with this name, like "America/Toronto".
// Unknown or empty names give UTC.
func Location(name string) *time.Location {
	if loc, ok := locations.Load(name); ok {
		return loc.(*time.Location)
	}

	loc, err := time.LoadLocation(name)
	if err != nil {
		loc = time.UTC
	}
	locations.Store(name, loc)
	return loc
}
