// Package ticketmaster is a client for the Ticketmaster Discovery API.
package ticketmaster

// response mirrors the top level of a Ticketmaster Discovery API response.
// Only the fields we need are declared; everything else is ignored.
type response struct {
	Embedded struct {
		Events []event `json:"events"`
	} `json:"_embedded"`
}

// event is a single event as Ticketmaster returns it.
type event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`

	Dates struct {
		Start struct {
			LocalDate string `json:"localDate"` // e.g. "2026-12-29"
			LocalTime string `json:"localTime"` // e.g. "19:30:00"
			DateTime  string `json:"dateTime"`  // UTC, e.g. "2026-12-30T00:30:00Z"
		} `json:"start"`
	} `json:"dates"`

	Embedded struct {
		Venues []struct {
			Name string `json:"name"`
			City struct {
				Name string `json:"name"`
			} `json:"city"`
		} `json:"venues"`
	} `json:"_embedded"`
}
