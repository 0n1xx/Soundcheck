package ticketmaster

import (
	"encoding/json"
	"fmt"
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
	params.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("request ticketmaster: %w", err)
	}
	// Always close the body, otherwise the connection leaks.
	defer resp.Body.Close()

	// A 4xx/5xx response is not a Go error, so check the status explicitly.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticketmaster returned status %d", resp.StatusCode)
	}

	var data response
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
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

	// Prefer the exact UTC start time; fall back to the date alone
	// for events whose time has not been announced yet.
	if t, err := time.Parse(time.RFC3339, e.Dates.Start.DateTime); err == nil {
		ev.StartsAt = t
	} else if t, err := time.Parse("2006-01-02", e.Dates.Start.LocalDate); err == nil {
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
