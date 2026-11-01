// Package web serves the Soundcheck website: HTML pages, forms and static files.
package web

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/0n1xx/soundcheck/internal/core"
	"github.com/0n1xx/soundcheck/internal/notify"
	"github.com/0n1xx/soundcheck/internal/store"
	"github.com/0n1xx/soundcheck/internal/ticketmaster"
	"github.com/0n1xx/soundcheck/internal/worker"
)

// The HTML and CSS are built into the program itself, so the binary
// that runs on Railway doesn't need the files next to it.
//
//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

// Config is everything the website needs. Optional parts can stay empty.
type Config struct {
	DB           *store.Store
	Ticketmaster *ticketmaster.Client
	Worker       *worker.Worker
	Telegram     *notify.Telegram // nil: Telegram is off
	BotName      string           // the bot's username, for t.me links
	Google       GoogleConfig     // empty ClientID: Google sign-in is off
}

// Server is the website: parsed pages, the database, Ticketmaster and the router.
type Server struct {
	mux       *http.ServeMux
	templates *template.Template
	db        *store.Store
	tm        *ticketmaster.Client // checks that artists exist
	worker    *worker.Worker       // checks a new artist's shows right away
	tg        *notify.Telegram     // the Telegram bot; nil if not configured
	botName   string               // the bot's username, for t.me links
	google    GoogleConfig         // Google sign-in
	http      *http.Client         // for requests to Google
}

// pageData is everything a page template can show.
type pageData struct {
	User    *core.User    // the logged-in user, or nil for visitors
	Artists []core.Artist // artists the logged-in user follows
	Shows   []core.Event  // upcoming shows of those artists in the user's city
	Email   string        // keeps what was typed in a form after an error
	Error   string        // message shown above a form
	Notice  string        // "Saved." after a settings form
	Page    string        // which page is shown, e.g. "artists.html"; set by render
}

// NewServer loads the pages and registers every route.
func NewServer(cfg Config) (*Server, error) {
	// Helpers like showDate must be registered before the pages are parsed,
	// otherwise Go won't know what {{showDate .}} means.
	templates, err := template.New("").Funcs(templateFuncs).ParseFS(templateFiles, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return nil, fmt.Errorf("load static files: %w", err)
	}

	s := &Server{
		mux:       http.NewServeMux(),
		templates: templates,
		db:        cfg.DB,
		tm:        cfg.Ticketmaster,
		worker:    cfg.Worker,
		tg:        cfg.Telegram,
		botName:   cfg.BotName,
		google:    cfg.Google,
		http:      &http.Client{Timeout: 10 * time.Second},
	}

	// CSS and other files are sent as they are.
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	// Pages for visitors. Logged-in users are sent to their shows instead.
	s.mux.HandleFunc("GET /{$}", s.guestOnly(s.page("index.html")))
	s.mux.HandleFunc("GET /login", s.guestOnly(s.page("login.html")))
	s.mux.HandleFunc("GET /privacy", s.page("privacy.html"))

	// Sign up, log in and log out (auth.go).
	s.mux.HandleFunc("POST /signup", s.handleSignup)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)

	// Sign in with Google (google.go).
	s.mux.HandleFunc("GET /auth/google", s.guestOnly(s.handleGoogleStart))
	s.mux.HandleFunc("GET /auth/google/callback", s.handleGoogleCallback)

	// Dashboard pages: only for logged-in users.
	s.mux.HandleFunc("GET /shows", s.requireUser(s.page("shows.html")))
	s.mux.HandleFunc("GET /artists", s.requireUser(s.page("artists.html")))
	s.mux.HandleFunc("GET /settings", s.requireUser(s.page("settings.html")))

	// Dashboard forms (dashboard.go): also only for logged-in users.
	s.mux.HandleFunc("POST /artists/add", s.requireUser(s.handleAddArtist))
	s.mux.HandleFunc("POST /artists/remove", s.requireUser(s.handleRemoveArtist))
	s.mux.HandleFunc("POST /settings/city", s.requireUser(s.handleSetCity))

	// Account settings (account.go).
	s.mux.HandleFunc("POST /settings/email", s.requireUser(s.handleSetEmail))
	s.mux.HandleFunc("POST /settings/password", s.requireUser(s.handleSetPassword))
	s.mux.HandleFunc("POST /settings/notifications", s.requireUser(s.handleSetNotifications))
	s.mux.HandleFunc("POST /account/delete", s.requireUser(s.handleDeleteAccount))

	// Telegram (telegram.go).
	s.mux.HandleFunc("POST /settings/telegram/connect", s.requireUser(s.handleConnectTelegram))
	s.mux.HandleFunc("POST /settings/telegram/disconnect", s.requireUser(s.handleDisconnectTelegram))

	// Anything else gets the "page not found" page.
	s.mux.HandleFunc("/", s.notFound)

	return s, nil
}

// ServeHTTP makes Server usable by http.ListenAndServe:
// every request is handed to the router.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A few standard safety headers on every response.
	h := w.Header()
	// Browsers must use the Content-Type we send, not guess one.
	h.Set("X-Content-Type-Options", "nosniff")
	// Other sites can't show our pages inside a frame (clickjacking).
	h.Set("X-Frame-Options", "DENY")
	// Links to other sites don't reveal the full address of the page.
	h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
	// Browsers remember to always use HTTPS for this site for a year.
	h.Set("Strict-Transport-Security", "max-age=31536000")

	s.mux.ServeHTTP(w, r)
}

// page returns a handler that shows one HTML page.
// Dashboard pages also get the user's artists from the database.
func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Visitors: nobody is logged in, so there's nothing to load.
		if userFromContext(r) == nil {
			s.render(w, http.StatusOK, name, pageData{})
			return
		}

		data, err := s.dashboardData(r)
		if err != nil {
			s.serverError(w, err)
			return
		}
		// Settings forms come back with ?saved=1 after a successful save.
		if r.URL.Query().Has("saved") {
			data.Notice = "Saved."
		}
		s.render(w, http.StatusOK, name, data)
	}
}

// notFound shows the 404 page with the right status code. Logged-in users
// keep their own sidebar, so a mistyped link doesn't look like a logout.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	data := pageData{}
	if user, ok := s.currentUser(r); ok {
		// The same thing requireUser does: remember who this is for dashboardData.
		r = r.WithContext(context.WithValue(r.Context(), userKey{}, &user))
		if d, err := s.dashboardData(r); err == nil {
			data = d
		}
		// If loading fails, the visitor version of the page is still fine.
	}
	s.render(w, http.StatusNotFound, "not-found.html", data)
}

// render writes a page to the browser, filling it with data.
func (s *Server) render(w http.ResponseWriter, status int, name string, data pageData) {
	// Lets shared templates (like the sidebar) know which page they're on.
	data.Page = name

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

// serverError logs the real problem and shows the visitor a short message.
func (s *Server) serverError(w http.ResponseWriter, err error) {
	log.Printf("server error: %v", err)
	http.Error(w, "Something went wrong. Please try again.", http.StatusInternalServerError)
}
