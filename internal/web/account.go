package web

import (
	"errors"
	"net/http"
	"strings"

	"github.com/0n1xx/soundcheck/internal/store"
)

// handleSetEmail saves a new email from the Email window in Settings.
func (s *Server) handleSetEmail(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	email := strings.TrimSpace(r.FormValue("email"))

	if !strings.Contains(email, "@") {
		s.renderDashboard(w, r, http.StatusBadRequest, "settings.html", "Enter a valid email address.")
		return
	}

	err := s.db.SetEmail(r.Context(), user.ID, email)
	if errors.Is(err, store.ErrEmailTaken) {
		s.renderDashboard(w, r, http.StatusConflict, "settings.html",
			"Another account already uses this email.")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	saved(w, r)
}

// handleSetPassword changes the password. People who signed up with Google
// have no password yet, so for them the current one isn't asked.
func (s *Server) handleSetPassword(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	current := r.FormValue("current_password")
	next := r.FormValue("new_password")

	if user.PasswordHash != "" && !checkPassword(user.PasswordHash, current) {
		s.renderDashboard(w, r, http.StatusBadRequest, "settings.html", "Your current password is wrong.")
		return
	}
	if err := validatePassword(next, user.Email); err != nil {
		s.renderDashboard(w, r, http.StatusBadRequest, "settings.html", err.Error())
		return
	}

	hash, err := hashPassword(next)
	if err != nil {
		s.serverError(w, err)
		return
	}
	if err := s.db.SetPassword(r.Context(), user.ID, hash); err != nil {
		s.serverError(w, err)
		return
	}

	// Log out every other browser, in case someone else knew the old password.
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.db.DeleteOtherSessions(r.Context(), user.ID, c.Value); err != nil {
			s.serverError(w, err)
			return
		}
	}
	saved(w, r)
}

// handleSetNotifications saves the four checkboxes in the Notifications window.
// A checkbox that isn't ticked isn't sent with the form at all, so
// "is it in the form?" is the same as "is it ticked?".
func (s *Server) handleSetNotifications(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	n := store.Notifications{
		New:     r.FormValue("notify_new") != "",
		Day:     r.FormValue("notify_24h") != "",
		Minutes: r.FormValue("notify_15m") != "",
		Changes: r.FormValue("notify_changes") != "",
	}

	if err := s.db.SetNotifications(r.Context(), user.ID, n); err != nil {
		s.serverError(w, err)
		return
	}
	saved(w, r)
}

// handleDeleteAccount removes the user and everything they saved,
// then logs the browser out.
func (s *Server) handleDeleteAccount(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)

	// The checkbox is required in the HTML, but check it here too:
	// a form can be sent without the browser's checks.
	if r.FormValue("confirm") == "" {
		s.renderDashboard(w, r, http.StatusBadRequest, "settings.html",
			"Tick the box to confirm you want to delete your account.")
		return
	}

	if err := s.db.DeleteUser(r.Context(), user.ID); err != nil {
		s.serverError(w, err)
		return
	}
	clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// saved sends the browser back to Settings with a "Saved." message.
func saved(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/settings?saved=1", http.StatusSeeOther)
}
