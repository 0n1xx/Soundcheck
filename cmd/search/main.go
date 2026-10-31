// Command soundcheck searches for upcoming concerts.
//
// Usage:
//
//	go run ./cmd/search "Artist Name" City
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	"github.com/0n1xx/soundcheck/internal/core"
	"github.com/0n1xx/soundcheck/internal/ticketmaster"
)

func main() {
	// Load .env if it exists; on Railway variables come from the service settings.
	_ = godotenv.Load()

	apiKey := os.Getenv("TICKETMASTER_API_KEY")
	if apiKey == "" {
		log.Fatal("TICKETMASTER_API_KEY is not set")
	}

	if len(os.Args) < 3 {
		fmt.Println(`Usage: go run ./cmd/search "Artist Name" City`)
		os.Exit(1)
	}
	artist, city := os.Args[1], os.Args[2]

	// main only knows about the Source interface, not about Ticketmaster details.
	var source core.Source = ticketmaster.New(apiKey)

	events, err := source.Search(artist, city)
	if err != nil {
		log.Fatal(err)
	}

	if len(events) == 0 {
		fmt.Printf("No upcoming concerts for %s in %s\n", artist, city)
		return
	}

	for _, e := range events {
		venue := e.Venue
		if venue == "" {
			venue = "TBA"
		}

		fmt.Printf("%s\n", e.Name)
		fmt.Printf("  Venue: %s, %s\n", venue, e.City)
		fmt.Printf("  Date:  %s\n", e.StartsAt.Local().Format("Mon, Jan 2 2006 · 15:04"))
		fmt.Printf("  Link:  %s\n\n", e.URL)
	}
}
