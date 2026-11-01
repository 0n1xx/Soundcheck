package ticketmaster

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/0n1xx/soundcheck/internal/core"
)

const baseURL = "https://app.ticketmaster.com/discovery/v2/events.json"

// Client talks to the Ticketmaster Discovery API.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// New creates a Client with a sensible request timeout.
func New(apiKey string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Search returns upcoming music events for an artist in a city.
// It implements core.Source.
func (c *Client) Search(artist, city string) ([]core.Event, error) {
	// url.Values escapes special characters, so "Danny Brown" becomes "Danny+Brown".
	params := url.Values{}
	params.Set("keyword", artist)
	params.Set("city", city)
	params.Set("classificationName", "music")

	// core.Source has no context (it's used by the small command-line tool),
	// so this request simply runs until it finishes or times out.
	var data response
	if err := c.getJSON(context.Background(), baseURL, params, &data); err != nil {
		return nil, err
	}

	events := make([]core.Event, 0, len(data.Embedded.Events))
	for _, e := range data.Embedded.Events {
		events = append(events, toEvent(e))
	}
	return events, nil
}

// toEvent converts a Ticketmaster event into the app's core.Event.
func toEvent(e event) core.Event {
	ev := core.Event{
		ID:   e.ID,
		Name: e.Name,
		URL:  e.URL,
	}

	if len(e.Embedded.Venues) > 0 {
		ev.Venue = e.Embedded.Venues[0].Name
		ev.City = e.Embedded.Venues[0].City.Name
	}

	// The venue's time zone, used to show dates as local time.
	ev.Timezone = e.Dates.Timezone
	loc := core.Location(ev.Timezone)

	// Prefer the exact UTC start time; fall back to the date alone
	// (midnight at the venue) for events whose time isn't announced yet.
	if t, err := time.Parse(time.RFC3339, e.Dates.Start.DateTime); err == nil {
		ev.StartsAt = t
	} else if t, err := time.ParseInLocation("2006-01-02", e.Dates.Start.LocalDate, loc); err == nil {
		ev.StartsAt = t
	}

	ev.Status = e.Dates.Status.Code
	if t, err := time.Parse(time.RFC3339, e.Sales.Public.StartDateTime); err == nil {
		ev.OnSaleAt = t
	}

	return ev
}

// Compile-time check that *Client satisfies core.Source.
var _ core.Source = (*Client)(nil)
