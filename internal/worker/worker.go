// Package worker checks Ticketmaster for concerts in the background,
// every few hours, while the website keeps running.
package worker

import (
	"context"
	"log"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/0n1xx/soundcheck/internal/store"
	"github.com/0n1xx/soundcheck/internal/ticketmaster"
)

const (
	// interval is how often the worker checks every artist.
	interval = 3 * time.Hour
	// parallel is how many artists are checked at the same time.
	parallel = 5
	// requestsPerSecond stays under Ticketmaster's limit of 5 per second.
	requestsPerSecond = 4
	// artistTimeout gives up on one artist if Ticketmaster is too slow.
	artistTimeout = 30 * time.Second
)

// Worker finds concerts for every followed artist and saves them.
type Worker struct {
	db      *store.Store
	tm      *ticketmaster.Client
	limiter *rate.Limiter
}

// New creates a worker. Call Run to start it.
func New(db *store.Store, tm *ticketmaster.Client) *Worker {
	return &Worker{
		db: db,
		tm: tm,
		// At most 4 requests per second, one at a time (burst of 1).
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), 1),
	}
}

// Run checks all artists right away, then again every interval,
// until ctx is cancelled (when the program is shutting down).
func (w *Worker) Run(ctx context.Context) {
	log.Printf("worker: started, checking every %s", interval)

	// First check right after start, so a fresh deploy has data quickly.
	w.checkAll(ctx)

	// The alarm clock: a signal every 3 hours.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done(): // "your shift is over"
			log.Println("worker: stopped")
			return
		case <-ticker.C: // "3 hours passed, time for a round"
			w.checkAll(ctx)
		}
	}
}

// checkAll is one round: every followed artist, 5 at a time.
func (w *Worker) checkAll(ctx context.Context) {
	start := time.Now()

	ids, err := w.db.ArtistIDsToCheck(ctx)
	if err != nil {
		log.Printf("worker: list artists: %v", err)
		return
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(parallel) // at most 5 artists at once

	for _, id := range ids {
		g.Go(func() error {
			// One artist failing (e.g. Ticketmaster hiccup) must not stop
			// the others, so log the error and report success to the group.
			if err := w.checkArtist(ctx, id); err != nil {
				log.Printf("worker: artist %s: %v", id, err)
			}
			return nil
		})
	}

	g.Wait() // wait until every artist is done
	log.Printf("worker: checked %d artists in %s", len(ids), time.Since(start).Round(time.Second))
}

// checkArtist asks Ticketmaster for one artist's concerts and saves them.
func (w *Worker) checkArtist(ctx context.Context, artistID string) error {
	// Don't let one slow answer hold up the whole round.
	ctx, cancel := context.WithTimeout(ctx, artistTimeout)
	defer cancel()

	// Wait for our turn at the "checkout": at most 4 requests per second.
	if err := w.limiter.Wait(ctx); err != nil {
		return err
	}

	events, err := w.tm.EventsByArtist(ctx, artistID)
	if err != nil {
		return err
	}

	for _, e := range events {
		change, err := w.db.SaveEvent(ctx, e)
		if err != nil {
			return err
		}

		// For now just log what happened; step 5 turns these into emails.
		switch change {
		case store.EventNew:
			log.Printf("worker: new show %q in %s", e.Name, e.City)
		case store.EventStatusChanged:
			log.Printf("worker: %q is now %s", e.Name, e.Status)
		case store.EventRescheduled:
			log.Printf("worker: %q moved to %s", e.Name, e.StartsAt.Format(time.RFC1123))
		}
	}
	return nil
}
