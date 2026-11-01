package web

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/0n1xx/soundcheck/internal/notify"
	"github.com/0n1xx/soundcheck/internal/store"
)

// handleForgot sends a "reset your password" link. The page that follows
// says the same thing whether or not the email has an account, so the form
// can't be used to find out who signed up.
func (s *Server) handleForgot(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))

	if s.mailer == nil {
		s.render(w, http.StatusServiceUnavailable, "forgot.html", pageData{
			Email: email,
			Error: "Password reset isn't available right now.",
		})
		return
	}
	if !strings.Contains(email, "@") {
		s.render(w, http.StatusBadRequest, "forgot.html", pageData{
			Email: email,
			Error: "Enter a valid email address.",
		})
		return
	}

	user, err := s.db.UserByEmail(r.Context(), email)
	switch {
	case errors.Is(err, store.ErrNotFound):
		// No account: send nothing, but show the same page.
	case err != nil:
		s.serverError(w, err)
		return
	default:
		if err := s.sendReset(r, user.ID, user.Email); err != nil && !errors.Is(err, store.ErrTooSoon) {
			log.Printf("send reset: %v", err)
		}
	}

	s.render(w, http.StatusOK, "forgot.html", pageData{
		Email:  email,
		Notice: "If there's an account for this email, we've sent a link to reset the password. It works for 1 hour.",
	})
}

// sendReset emails a one-time link to the reset page.
func (s *Server) sendReset(r *http.Request, userID int64, email string) error {
	token, err := s.db.CreateResetToken(r.Context(), userID, email)
	if err != nil {
		return err
	}

	link := s.baseURL + "/reset?token=" + url.QueryEscape(token)
	return s.mailer.Notify(r.Context(), notify.Recipient{Email: email}, notify.Message{
		Title:  "Reset your password",
		Text:   "Someone asked to reset the password for " + email + " on Soundcheck.\n\nThe link works for 1 hour. If it wasn't you, ignore this email: your password stays the same.",
		URL:    link,
		Button: "Choose a new password",
		Footer: "You're getting this because a password reset was requested on Soundcheck.",
	})
}

// handleResetForm shows the "choose a new password" form for a link from
// the email. It only checks the link: mail apps sometimes open links on
// their own to scan them, and that must not use the link up.
func (s *Server) handleResetForm(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")

	user, err := s.db.UserByResetToken(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		s.render(w, http.StatusBadRequest, "forgot.html", pageData{
			Error: "This reset link has expired or was already used. Send yourself a new one.",
		})
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	s.render(w, http.StatusOK, "reset.html", pageData{Email: user.Email, Token: token})
}

// handleReset saves the new password, then logs the user in.
func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")

	user, err := s.db.UserByResetToken(r.Context(), token)
	if errors.Is(err, store.ErrNotFound) {
		s.render(w, http.StatusBadRequest, "forgot.html", pageData{
			Error: "This reset link has expired or was already used. Send yourself a new one.",
		})
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	// Same rules as at sign-up.
	if err := validatePassword(password, user.Email); err != nil {
		s.render(w, http.StatusBadRequest, "reset.html", pageData{
			Email: user.Email, Token: token, Error: err.Error(),
		})
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		s.serverError(w, err)
		return
	}

	userID, err := s.db.ResetPassword(r.Context(), token, hash)
	if errors.Is(err, store.ErrNotFound) {
		// Used up between showing the form and sending it.
		s.render(w, http.StatusBadRequest, "forgot.html", pageData{
			Error: "This reset link has expired or was already used. Send yourself a new one.",
		})
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	s.startSession(w, r, userID, "/shows")
}
