package ticketmaster

import (
	"context"
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

	var data response
	if err := c.getJSON(ctx, baseURL, params, &data); err != nil {
		return nil, err
	}

	events := make([]core.Event, 0, len(data.Embedded.Events))
	for _, e := range data.Embedded.Events {
		ev := toEvent(e)
		ev.ArtistID = artistID // remember which artist we found it for
		events = append(events, ev)
	}
	return events, nil
}
