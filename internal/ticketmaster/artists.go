package ticketmaster

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode"
)

const attractionsURL = "https://app.ticketmaster.com/discovery/v2/attractions.json"

// ErrArtistNotFound means Ticketmaster has no musician with that name.
var ErrArtistNotFound = errors.New("artist not found on ticketmaster")

// Artist is a musician as Ticketmaster knows them.
type Artist struct {
	ID   string // Ticketmaster's attraction ID, e.g. "K8vZ917Kew0"
	Name string // spelled the way Ticketmaster spells it
}

// attractionsResponse mirrors the attractions search response.
type attractionsResponse struct {
	Embedded struct {
		Attractions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"attractions"`
	} `json:"_embedded"`
}

// FindArtist looks up a musician by name. It only accepts a result whose
// name matches what was typed (ignoring case, spaces and punctuation),
// so typos and made-up names are rejected instead of matching someone else.
func (c *Client) FindArtist(ctx context.Context, name string) (Artist, error) {
	params := url.Values{}
	params.Set("keyword", name)
	params.Set("classificationName", "music")
	params.Set("size", "20")

	var data attractionsResponse
	if err := c.getJSON(ctx, attractionsURL, params, &data); err != nil {
		return Artist{}, err
	}

	// Ticketmaster returns everything that loosely matches the keyword,
	// so pick the one whose name is the same as what the user typed.
	want := simplify(name)
	for _, a := range data.Embedded.Attractions {
		if simplify(a.Name) == want {
			return Artist{ID: a.ID, Name: a.Name}, nil
		}
	}
	return Artist{}, ErrArtistNotFound
}

// simplify keeps only letters and digits, in lower case, so that
// "Tyler, The Creator" and "tyler the creator" compare as equal.
func simplify(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
