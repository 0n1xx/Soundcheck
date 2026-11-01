package ticketmaster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// getJSON sends one GET request to Ticketmaster and decodes the JSON answer
// into out. Search, FindArtist and EventsByArtist all go through here, so
// the API key, the status check and the error handling live in one place.
func (c *Client) getJSON(ctx context.Context, endpoint string, params url.Values, out any) error {
	params.Set("apikey", c.apiKey)

	// The context cancels the request if the caller gives up: the worker is
	// shutting down, or the visitor closed the tab while adding an artist.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// A network error from net/http includes the full URL, and our URL
		// contains the API key. Keep only the cause, so the key never ends
		// up in the logs on Railway.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("request ticketmaster: %w", err)
	}
	defer resp.Body.Close()

	// A 4xx/5xx response is not a Go error, so check the status explicitly.
	// 429 means we went over the rate limit.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ticketmaster returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode ticketmaster response: %w", err)
	}
	return nil
}
