// Command soundcheck runs the Soundcheck website.
//
// Usage:
//
//	go run ./cmd/soundcheck
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"

	"github.com/0n1xx/soundcheck/internal/web"
)

func main() {
	// Load .env if it exists; on Railway variables come from the service settings.
	_ = godotenv.Load()

	// Railway tells the app which port to listen on through PORT.
	// Locally there is no PORT, so we fall back to 8080.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv, err := web.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Soundcheck is running on http://localhost:%s", port)

	// ListenAndServe blocks forever: it waits for requests and hands each one to srv.
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
