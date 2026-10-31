package web

import (
	"net/http"
	"strings"
	"unicode/utf8"
)

const (
	// maxArtists keeps the Ticketmaster checks for one user reasonable.
	maxArtists = 50
	// maxNameLength applies to artist and city names.
	maxNameLength = 100
)

// ---- Handlers ---------------------------------------------------------------
// All of them run behind requireUser, so the user is always logged in here.

// handleAddArtist adds an artist from the form at the top of the Artists page.
func (s *Server) handleAddArtist(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	name := cleanName(r.FormValue("artist"))

	// An empty name means the form was sent blank: just show the page again.
	if name == "" {
		http.Redirect(w, r, "/artists", http.StatusSeeOther)
		return
	}
	if utf8.RuneCountInString(name) > maxNameLength {
		s.renderDashboard(w, r, http.StatusBadRequest, "artists.html", "Artist name is too long.")
		return
	}

	// Check the limit before adding.
	artists, err := s.db.ListArtists(r.Context(), user.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if len(artists) >= maxArtists {
		s.renderDashboard(w, r, http.StatusBadRequest, "artists.html",
			"You can follow up to 50 artists. Remove one to add another.")
		return
	}

	if err := s.db.AddArtist(r.Context(), user.ID, name); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/artists", http.StatusSeeOther)
}

// handleRemoveArtist removes the artist whose "Remove" button was pressed.
func (s *Server) handleRemoveArtist(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	name := r.FormValue("artist") // comes from a hidden field, sent back as it was saved

	if err := s.db.RemoveArtist(r.Context(), user.ID, name); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/artists", http.StatusSeeOther)
}

// handleSetCity saves the city from the Location window on the Settings page.
func (s *Server) handleSetCity(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	city := cleanName(r.FormValue("city"))

	if city == "" {
		s.renderDashboard(w, r, http.StatusBadRequest, "settings.html", "Enter a city.")
		return
	}
	if utf8.RuneCountInString(city) > maxNameLength {
		s.renderDashboard(w, r, http.StatusBadRequest, "settings.html", "City name is too long.")
		return
	}

	if err := s.db.SetCity(r.Context(), user.ID, city); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings", http.StatusSeeOther)
}

// ---- Page data --------------------------------------------------------------

// dashboardData collects what every dashboard page shows:
// the user (email, city) and their artists (list and sidebar counter).
func (s *Server) dashboardData(r *http.Request) (pageData, error) {
	user := userFromContext(r)

	artists, err := s.db.ListArtists(r.Context(), user.ID)
	if err != nil {
		return pageData{}, err
	}
	return pageData{User: user, Artists: artists}, nil
}

// renderDashboard shows a dashboard page with an error message on top.
func (s *Server) renderDashboard(w http.ResponseWriter, r *http.Request, status int, name, msg string) {
	data, err := s.dashboardData(r)
	if err != nil {
		s.serverError(w, err)
		return
	}
	data.Error = msg
	s.render(w, status, name, data)
}

// cleanName trims a name and squeezes repeated spaces:
// "  Danny   Brown " becomes "Danny Brown".
func cleanName(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
