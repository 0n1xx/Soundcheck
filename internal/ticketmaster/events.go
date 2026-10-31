package ticketmaster

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/0n1xx/soundcheck/internal/core"
)

// EventsByArtist returns every upcoming concert of one artist, in any city.
// The worker calls it for each artist; filtering by city happens later,
// when a page shows concerts to a particular user.
func (c *Client) EventsByArtist(ctx context.Context, artistID string) ([]core.Event, error) {
	params := url.Values{}
	params.Set("attractionId", artistID) // exactly this artist, not a text search
	params.Set("classificationName", "music")
	params.Set("sort", "date,asc")
	params.Set("size", "100") // more than enough upcoming shows for one artist
	params.Set("apikey", c.apiKey)

	// NewRequestWithContext ties the request to ctx: if the worker is told
	// to stop, or ctx has a timeout, the request is cancelled instead of hanging.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request ticketmaster: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticketmaster returned status %d", resp.StatusCode)
	}

	var data response
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}

	events := make([]core.Event, 0, len(data.Embedded.Events))
	for _, e := range data.Embedded.Events {
		ev := toEvent(e)
		ev.ArtistID = artistID // remember which artist we found it for
		events = append(events, ev)
	}
	return events, nil
}
