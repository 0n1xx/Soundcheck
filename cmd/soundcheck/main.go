// Command soundcheck runs the Soundcheck website and the background worker.
//
// Usage:
//
//	go run ./cmd/soundcheck
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/0n1xx/soundcheck/internal/store"
	"github.com/0n1xx/soundcheck/internal/ticketmaster"
	"github.com/0n1xx/soundcheck/internal/web"
	"github.com/0n1xx/soundcheck/internal/worker"
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

	tmKey := os.Getenv("TICKETMASTER_API_KEY")
	if tmKey == "" {
		log.Fatal("TICKETMASTER_API_KEY is not set")
	}

	// ctx is the "go home" call: it is cancelled when Railway (or Ctrl+C)
	// asks the program to stop. Everything long-running listens to it.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Give the database 10 seconds to answer on startup, then give up.
	startCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := store.New(startCtx, databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Migrate(startCtx); err != nil {
		log.Fatal(err)
	}
	log.Println("Database is ready")

	// One Ticketmaster client, shared by the website and the worker.
	tm := ticketmaster.New(tmKey)

	// The worker runs in its own goroutine, next to the website.
	go worker.New(db, tm).Run(ctx)

	srv, err := web.NewServer(db, tm)
	if err != nil {
		log.Fatal(err)
	}

	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: srv,
	}

	// The website also runs in its own goroutine, so main can wait for "stop".
	go func() {
		log.Printf("Soundcheck is running on http://localhost:%s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	// Wait here until Railway asks us to stop.
	<-ctx.Done()
	log.Println("Shutting down...")

	// Let requests that are in progress finish, but don't wait more than 10 seconds.
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("Bye")
}
