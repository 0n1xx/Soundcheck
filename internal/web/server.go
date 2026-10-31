// Package web serves the Soundcheck website: HTML pages, forms and static files.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"

	"github.com/0n1xx/soundcheck/internal/core"
	"github.com/0n1xx/soundcheck/internal/store"
)

// The HTML and CSS are built into the program itself, so the binary
// that runs on Railway doesn't need the files next to it.
//
//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

// Server is the website: parsed pages, the database and the router.
type Server struct {
	mux       *http.ServeMux
	templates *template.Template
	db        *store.Store
}

// pageData is everything a page template can show.
type pageData struct {
	User  *core.User // the logged-in user, or nil for visitors
	Email string     // keeps what was typed in a form after an error
	Error string     // message shown above a form
}

// NewServer loads the pages and registers every route.
func NewServer(db *store.Store) (*Server, error) {
	templates, err := template.ParseFS(templateFiles, "templates/*.html")
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
		db:        db,
	}

	// CSS and other files are sent as they are.
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	// Pages for visitors. Logged-in users are sent to their shows instead.
	s.mux.HandleFunc("GET /{$}", s.guestOnly(s.page("index.html")))
	s.mux.HandleFunc("GET /login", s.guestOnly(s.page("login.html")))
	s.mux.HandleFunc("GET /privacy", s.page("privacy.html"))

	// Forms.
	s.mux.HandleFunc("POST /signup", s.handleSignup)
	s.mux.HandleFunc("POST /login", s.handleLogin)
	s.mux.HandleFunc("POST /logout", s.handleLogout)

	// Dashboard pages: only for logged-in users.
	s.mux.HandleFunc("GET /shows", s.requireUser(s.page("shows.html")))
	s.mux.HandleFunc("GET /artists", s.requireUser(s.page("artists.html")))
	s.mux.HandleFunc("GET /settings", s.requireUser(s.page("settings.html")))

	// Anything else gets the "page not found" page.
	s.mux.HandleFunc("/", s.notFound)

	return s, nil
}

// ServeHTTP makes Server usable by http.ListenAndServe:
// every request is handed to the router.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// page returns a handler that shows one HTML page.
func (s *Server) page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.render(w, http.StatusOK, name, pageData{User: userFromContext(r)})
	}
}

// notFound shows the 404 page with the right status code.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusNotFound, "not-found.html", pageData{})
}

// render writes a page to the browser, filling it with data.
func (s *Server) render(w http.ResponseWriter, status int, name string, data pageData) {
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
