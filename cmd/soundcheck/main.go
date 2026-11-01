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
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/0n1xx/soundcheck/internal/notify"
	"github.com/0n1xx/soundcheck/internal/store"
	"github.com/0n1xx/soundcheck/internal/ticketmaster"
	"github.com/0n1xx/soundcheck/internal/web"
	"github.com/0n1xx/soundcheck/internal/worker"
)

func main() {
	// Railway colours stderr red; normal logs go to stdout.
	log.SetOutput(os.Stdout)

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

	// The site's public address, for links in alerts and Google sign-in.
	baseURL := strings.TrimRight(os.Getenv("APP_BASE_URL"), "/")

	// Alert channels are optional: each one is on only if it's configured.
	// Every working channel goes into the list the worker sends alerts through.
	var tg *notify.Telegram
	var channels []notify.Notifier
	if token := os.Getenv("TELEGRAM_BOT_TOKEN"); token != "" {
		tg = notify.NewTelegram(token)
		channels = append(channels, tg)
	}
	if key, from := os.Getenv("RESEND_API_KEY"), os.Getenv("EMAIL_FROM"); key != "" && from != "" {
		channels = append(channels, notify.NewEmail(key, from, baseURL))
	}

	// One worker: it runs in the background, and the website can ask it
	// to check a newly added artist right away.
	wk := worker.New(db, tm, channels...)
	go wk.Run(ctx)

	// Google sign-in needs the site's public address for the way back.
	google := web.GoogleConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  baseURL + "/auth/google/callback",
	}
	if google.ClientID != "" && baseURL == "" {
		log.Println("APP_BASE_URL is not set, so Google sign-in is off")
		google = web.GoogleConfig{}
	}

	srv, err := web.NewServer(web.Config{
		DB:           db,
		Ticketmaster: tm,
		Worker:       wk,
		Telegram:     tg,
		BotName:      os.Getenv("TELEGRAM_BOT_USERNAME"),
		Google:       google,
	})
	if err != nil {
		log.Fatal(err)
	}

	// The bot listens for /start in its own goroutine and hands each
	// one to the website, which knows the users.
	if tg != nil {
		go tg.Listen(ctx, srv.TelegramStart)
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
