// Package web serves the Soundcheck website: HTML pages and static files.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
)

// The HTML and CSS are built into the program itself, so the binary
// that runs on Railway doesn't need the files next to it.
//
//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static
var staticFiles embed.FS

// Server is the website: parsed pages plus the router that picks one per URL.
type Server struct {
	mux       *http.ServeMux
	templates *template.Template
}

// NewServer loads the pages and registers every route.
func NewServer() (*Server, error) {
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
	}

	// CSS and other files are sent as they are.
	s.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	// Pages. {$} means "exactly /", otherwise "/" would match every URL.
	s.mux.HandleFunc("GET /{$}", s.page("index.html"))
	s.mux.HandleFunc("GET /login", s.page("login.html"))
	s.mux.HandleFunc("GET /privacy", s.page("privacy.html"))
	s.mux.HandleFunc("GET /shows", s.page("shows.html"))
	s.mux.HandleFunc("GET /artists", s.page("artists.html"))
	s.mux.HandleFunc("GET /settings", s.page("settings.html"))

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
		s.render(w, http.StatusOK, name)
	}
}

// notFound shows the 404 page with the right status code.
func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusNotFound, "not-found.html")
}

// render writes a page to the browser.
func (s *Server) render(w http.ResponseWriter, status int, name string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	if err := s.templates.ExecuteTemplate(w, name, nil); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}
