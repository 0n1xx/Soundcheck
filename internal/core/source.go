package core

// Source is anything that can find concerts for an artist in a city.
// Ticketmaster is the first implementation; others can be added later
// without changing the rest of the app.
type Source interface {
	Search(artist, city string) ([]Event, error)
}
