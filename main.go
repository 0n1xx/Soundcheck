// Command soundcheck searches Ticketmaster for upcoming concerts.
//
// Usage:
//
//	go run . "Artist Name" City
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// tmResponse mirrors the top level of a Ticketmaster Discovery API response.
// Only the fields we need are declared; everything else is ignored.
type tmResponse struct {
	Embedded struct {
		Events []tmEvent `json:"events"`
	} `json:"_embedded"`
}

// tmEvent is a single event as Ticketmaster returns it.
type tmEvent struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`

	Dates struct {
		Start struct {
			LocalDate string `json:"localDate"` // e.g. "2026-12-29"
			LocalTime string `json:"localTime"` // e.g. "19:30:00"
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

func main() {
	// Load variables from .env if the file exists.
	// On Railway there is no .env, variables come from the service settings,
	// so a missing file is not an error.
	_ = godotenv.Load()

	apiKey := os.Getenv("TICKETMASTER_API_KEY")
	if apiKey == "" {
		log.Fatal("TICKETMASTER_API_KEY is not set")
	}

	// os.Args[0] is the program itself, so we need at least two more arguments.
	if len(os.Args) < 3 {
		fmt.Println(`Usage: go run . "Artist Name" City`)
		os.Exit(1)
	}
	artist := os.Args[1]
	city := os.Args[2]

	events, err := searchEvents(apiKey, artist, city)
	if err != nil {
		log.Fatal(err)
	}

	if len(events) == 0 {
		fmt.Printf("No upcoming concerts for %s in %s\n", artist, city)
		return
	}

	for _, e := range events {
		// Some events have no venue yet; indexing an empty slice would panic.
		venue := "TBA"
		if len(e.Embedded.Venues) > 0 {
			venue = e.Embedded.Venues[0].Name
		}

		fmt.Printf("%s\n", e.Name)
		fmt.Printf("  Venue: %s\n", venue)
		fmt.Printf("  Date:  %s %s\n", e.Dates.Start.LocalDate, e.Dates.Start.LocalTime)
		fmt.Printf("  Link:  %s\n\n", e.URL)
	}
}

// searchEvents returns upcoming music events for an artist in a city.
func searchEvents(apiKey, artist, city string) ([]tmEvent, error) {
	// url.Values escapes special characters, so "Danny Brown" becomes "Danny+Brown".
	params := url.Values{}
	params.Set("keyword", artist)
	params.Set("city", city)
	params.Set("classificationName", "music")
	params.Set("apikey", apiKey)

	fullURL := "https://app.ticketmaster.com/discovery/v2/events.json?" + params.Encode()

	// The default HTTP client has no timeout and could hang forever.
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(fullURL)
	if err != nil {
		return nil, fmt.Errorf("request ticketmaster: %w", err)
	}
	// Always close the body, otherwise the connection leaks.
	defer resp.Body.Close()

	// A 4xx/5xx response is not a Go error, so check the status explicitly.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ticketmaster returned status %d", resp.StatusCode)
	}

	var data tmResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return data.Embedded.Events, nil
}
