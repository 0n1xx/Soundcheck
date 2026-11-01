package web

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/0n1xx/soundcheck/internal/notify"
	"github.com/0n1xx/soundcheck/internal/store"
)

// sendVerification emails a one-time link that confirms the address.
// Without an email channel (no Resend key) there's nothing to send.
func (s *Server) sendVerification(ctx context.Context, userID int64, email string) error {
	if s.mailer == nil {
		return nil
	}

	token, err := s.db.CreateEmailToken(ctx, userID, email)
	if err != nil {
		return err
	}

	link := s.baseURL + "/verify-email?token=" + url.QueryEscape(token)
	return s.mailer.Notify(ctx, notify.Recipient{Email: email}, notify.Message{
		Title:  "Confirm your email",
		Text:   "Confirm " + email + " to start getting concert alerts by email.\n\nThe link works for 24 hours.",
		URL:    link,
		Button: "Confirm email",
		Footer: "You're getting this because this address was used on Soundcheck. If it wasn't you, ignore this email.",
	})
}

// handleVerifyEmail is where the link in the confirmation email leads.
// It works without being logged in, e.g. when opened on a phone.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	userID, err := s.db.VerifyEmail(r.Context(), r.FormValue("token"))
	if errors.Is(err, store.ErrNotFound) {
		s.render(w, http.StatusBadRequest, "login.html", pageData{
			Error: "This confirmation link has expired or was already used. Log in and send a new one from Settings.",
		})
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	// Clicking the link proves they own the address, so log them straight
	// in. New users start by adding the artists they follow.
	s.startSession(w, r, userID, "/artists")
}

// showCheckEmail sends a confirmation link and shows the "Check your inbox"
// page. A link sent less than a minute ago is not resent, but the page is
// still shown, so the "Send again" button can't be used to flood an inbox.
func (s *Server) showCheckEmail(w http.ResponseWriter, r *http.Request, userID int64, email, notice string) {
	err := s.sendVerification(r.Context(), userID, email)
	if err != nil && !errors.Is(err, store.ErrTooSoon) {
		log.Printf("send verification: %v", err)
	}
	s.render(w, http.StatusOK, "check-email.html", pageData{Email: email, Notice: notice})
}

// handleResendSignup is the "Send the link again" button on the
// "Check your inbox" page. It looks the same whether or not the email has
// an account, so it can't be used to find out who signed up.
func (s *Server) handleResendSignup(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))

	user, err := s.db.UserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, err)
		return
	}
	if err == nil && !user.EmailVerified {
		s.showCheckEmail(w, r, user.ID, user.Email, "We sent another link.")
		return
	}
	s.render(w, http.StatusOK, "check-email.html", pageData{Email: email, Notice: "We sent another link."})
}

// handleResendVerification sends a new confirmation link from Settings.
func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if user.EmailVerified {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}

	err := s.sendVerification(r.Context(), user.ID, user.Email)
	if errors.Is(err, store.ErrTooSoon) {
		s.renderDashboard(w, r, http.StatusTooManyRequests, "settings.html",
			"We just sent you a link. Check your inbox, or try again in a minute.")
		return
	}
	if err != nil {
		log.Printf("resend verification: %v", err)
		s.renderDashboard(w, r, http.StatusBadGateway, "settings.html",
			"Couldn't send the email right now. Please try again later.")
		return
	}
	http.Redirect(w, r, "/settings?sent=1", http.StatusSeeOther)
}
