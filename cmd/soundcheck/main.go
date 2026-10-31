// Command soundcheck runs the Soundcheck website.
//
// Usage:
//
//	go run ./cmd/soundcheck
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/0n1xx/soundcheck/internal/store"
	"github.com/0n1xx/soundcheck/internal/web"
)

func main() {
	// Load .env if it exists; on Railway variables come from the service settings.
	_ = godotenv.Load()

	// Railway tells the app which port to listen on through PORT.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}

	// Give the database 10 seconds to answer on startup, then give up.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("Database is ready")

	srv, err := web.NewServer()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Soundcheck is running on http://localhost:%s", port)

	// ListenAndServe blocks forever: it waits for requests and hands each one to srv.
	log.Fatal(http.ListenAndServe(":"+port, srv))
}
