// Package worker checks Ticketmaster for concerts in the background,
// every few hours, while the website keeps running.
package worker

import (
	"context"
	"log"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/0n1xx/soundcheck/internal/core"
	"github.com/0n1xx/soundcheck/internal/notify"
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
	// reminderInterval is how often the worker looks for ticket sales
	// starting soon. It only reads our database, so every minute is cheap.
	reminderInterval = time.Minute
)

// Worker finds concerts for every followed artist and saves them.
type Worker struct {
	db        *store.Store
	tm        *ticketmaster.Client
	limiter   *rate.Limiter
	notifiers []notify.Notifier // every channel alerts go out through
}

// New creates a worker that sends alerts through the given channels
// (Telegram today, email later). Call Run to start it.
func New(db *store.Store, tm *ticketmaster.Client, notifiers ...notify.Notifier) *Worker {
	return &Worker{
		db:        db,
		tm:        tm,
		notifiers: notifiers,
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

	// Two alarm clocks: Ticketmaster every 3 hours, reminders every minute.
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	reminders := time.NewTicker(reminderInterval)
	defer reminders.Stop()

	for {
		select {
		case <-ctx.Done(): // "your shift is over"
			log.Println("worker: stopped")
			return
		case <-ticker.C: // "3 hours passed, time for a round"
			w.checkAll(ctx)
		case <-reminders.C: // "a minute passed, any sales starting soon?"
			w.remindOnSale(ctx)
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

	// Housekeeping after each round: old sessions, links and concerts.
	if err := w.db.Cleanup(ctx); err != nil {
		log.Printf("worker: %v", err)
	}
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

		// Tell followers about anything important that happened.
		switch change {
		case store.EventNew:
			log.Printf("worker: new show %q in %s", e.Name, e.City)
			w.alertNewShow(ctx, e)
		case store.EventStatusChanged:
			log.Printf("worker: %q is now %s", e.Name, e.Status)
			w.alertChange(ctx, e, e.Status)
		case store.EventRescheduled:
			log.Printf("worker: %q moved to %s", e.Name, e.StartsAt.Format(time.RFC1123))
			w.alertChange(ctx, e, "moved")
		}
	}
	return nil
}

// CheckArtist checks one artist right now. The website calls it when
// a user adds a new artist, so their shows appear without waiting for
// the next round. It shares the rate limiter with the background rounds.
func (w *Worker) CheckArtist(ctx context.Context, artistID string) error {
	return w.checkArtist(ctx, artistID)
}

// alertNewShow tells the artist's followers in that city about a new show.
func (w *Worker) alertNewShow(ctx context.Context, e core.Event) {
	users, err := w.db.NewShowFollowers(ctx, e.ID)
	if err != nil {
		log.Printf("worker: followers of %s: %v", e.ID, err)
		return
	}

	text := e.Place() + "\n" + e.DateText()
	if e.OnSaleAt.After(time.Now()) {
		text += "\n\nTickets on sale " + e.VenueTime(e.OnSaleAt).Format("Mon, Jan 2 · 15:04")
	}
	msg := notify.Message{Title: "New show: " + e.Name, Text: text, URL: e.URL}

	for _, u := range users {
		w.send(ctx, u, e.ID, "new", msg)
	}
}

// alertChange tells followers in the concert's city that it was cancelled,
// postponed or moved to a new date. Other changes, like "offsale" becoming
// "onsale", aren't worth a message and are ignored.
func (w *Worker) alertChange(ctx context.Context, e core.Event, change string) {
	var title, kind, note string
	switch change {
	case "cancelled":
		title, kind = "Cancelled: ", "cancelled"
		note = "Ticketmaster will contact ticket holders about refunds."
	case "postponed":
		title, kind = "Postponed: ", "postponed"
		note = "A new date hasn't been announced yet."
	case "rescheduled", "moved":
		title = "New date: "
		// The new date is part of the kind: if the show moves again,
		// that's a different alert and people hear about it again.
		kind = "moved " + e.StartsAt.UTC().Format(time.RFC3339)
	default:
		return
	}

	users, err := w.db.EventFollowers(ctx, e.ID)
	if err != nil {
		log.Printf("worker: %v", err)
		return
	}

	text := e.Place() + "\n" + e.DateText()
	if note != "" {
		text += "\n\n" + note
	}
	msg := notify.Message{Title: title + e.Name, Text: text, URL: e.URL}

	for _, u := range users {
		w.send(ctx, u, e.ID, kind, msg)
	}
}

// send delivers one alert to one user through every channel, at most once.
func (w *Worker) send(ctx context.Context, u core.User, eventID, kind string, msg notify.Message) {
	// Take the "ticket" first: if it's already taken, this alert went out before.
	first, err := w.db.MarkAlertSent(ctx, u.ID, eventID, kind)
	if err != nil {
		log.Printf("worker: %v", err)
		return
	}
	if !first {
		return
	}

	to := notify.Recipient{Email: u.Email, TelegramChatID: u.TelegramChatID}
	failures := 0
	for _, n := range w.notifiers {
		if err := n.Notify(ctx, to, msg); err != nil {
			log.Printf("worker: %s alert to user %d: %v", kind, u.ID, err)
			failures++
		}
	}

	// Every channel failed, so nothing reached the user: give the ticket back
	// so a later run can retry. If at least one channel worked, keep it, or
	// the retry would send that channel's message a second time.
	if len(w.notifiers) > 0 && failures == len(w.notifiers) {
		if err := w.db.UnmarkAlert(ctx, u.ID, eventID, kind); err != nil {
			log.Printf("worker: %v", err)
		}
		return
	}
	log.Printf("worker: sent %s alert about %s to user %d", kind, eventID, u.ID)
}

// remindOnSale sends "tickets on sale soon" reminders: one about a day
// before public sales start, and one 15 minutes before.
func (w *Worker) remindOnSale(ctx context.Context) {
	now := time.Now()
	// Up to a day ahead, but not the last hour: that's the 15-minute one's job.
	w.remind(ctx, "onsale_24h", now.Add(time.Hour), now.Add(24*time.Hour), "Tickets go on sale soon")
	w.remind(ctx, "onsale_15m", now, now.Add(15*time.Minute), "Tickets on sale in 15 minutes")
}

// remind sends one kind of on-sale reminder for sales starting between from and to.
func (w *Worker) remind(ctx context.Context, kind string, from, to time.Time, title string) {
	reminders, err := w.db.OnSaleReminders(ctx, from, to, kind)
	if err != nil {
		log.Printf("worker: %v", err)
		return
	}

	for _, r := range reminders {
		e := r.Event
		msg := notify.Message{
			Title: title + ": " + e.Name,
			Text: e.Place() + "\n" + e.DateText() +
				"\n\nSale starts " + e.VenueTime(e.OnSaleAt).Format("Mon, Jan 2 · 15:04"),
			URL: e.URL,
		}
		w.send(ctx, r.User, e.ID, kind, msg)
	}
}
