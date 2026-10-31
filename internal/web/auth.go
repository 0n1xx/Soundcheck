package web

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/0n1xx/soundcheck/internal/core"
	"github.com/0n1xx/soundcheck/internal/store"
)

// sessionCookie is the name of the cookie that keeps a browser logged in.
const sessionCookie = "soundcheck_session"

// userKey is the key under which requireUser stores the logged-in user
// in the request context. An empty struct type can't clash with other keys.
type userKey struct{}

// ---- Handlers ---------------------------------------------------------------

// handleSignup creates an account from the sign-up form and logs the user in.
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	// showError re-renders the form with a message and the typed email.
	showError := func(status int, msg string) {
		s.render(w, status, "index.html", pageData{Email: email, Error: msg})
	}

	if !strings.Contains(email, "@") {
		showError(http.StatusBadRequest, "Enter a valid email address.")
		return
	}
	if err := validatePassword(password, email); err != nil {
		showError(http.StatusBadRequest, err.Error())
		return
	}

	hash, err := hashPassword(password)
	if err != nil {
		s.serverError(w, err)
		return
	}

	user, err := s.db.CreateUser(r.Context(), email, hash)
	if errors.Is(err, store.ErrEmailTaken) {
		showError(http.StatusConflict, "An account with this email already exists. Log in instead.")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	// New users start by adding the artists they follow.
	s.startSession(w, r, user.ID, "/artists")
}

// handleLogin checks the email and password and logs the user in.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	user, err := s.db.UserByEmail(r.Context(), email)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.serverError(w, err)
		return
	}

	// Same message for "no such user" and "wrong password", so the form
	// doesn't reveal which emails have an account.
	if errors.Is(err, store.ErrNotFound) || !checkPassword(user.PasswordHash, password) {
		s.render(w, http.StatusUnauthorized, "login.html", pageData{
			Email: email,
			Error: "Wrong email or password.",
		})
		return
	}

	s.startSession(w, r, user.ID, "/shows")
}

// handleLogout ends the session and sends the visitor to the home page.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := s.db.DeleteSession(r.Context(), c.Value); err != nil {
			s.serverError(w, err)
			return
		}
	}

	// MaxAge -1 tells the browser to delete the cookie right away.
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ---- Middleware -------------------------------------------------------------

// requireUser lets only logged-in users through; everyone else goes to /login.
func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := s.currentUser(r)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Logged in: remember the user for the page and continue.
		ctx := context.WithValue(r.Context(), userKey{}, &user)
		next(w, r.WithContext(ctx))
	}
}

// guestOnly sends logged-in users to their shows instead of the
// sign-up and login pages.
func (s *Server) guestOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.currentUser(r); ok {
			http.Redirect(w, r, "/shows", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// ---- Helpers ----------------------------------------------------------------

// startSession creates a session, puts its token in a cookie and redirects.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64, redirectTo string) {
	token, expiresAt, err := s.db.CreateSession(r.Context(), userID)
	if err != nil {
		s.serverError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,                 // JavaScript can't read it
		Secure:   true,                 // only sent over HTTPS
		SameSite: http.SameSiteLaxMode, // not sent with forms from other sites
	})

	// 303 tells the browser to open the next page with GET,
	// so refreshing it doesn't resubmit the form.
	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

// currentUser returns the logged-in user, if the request has a valid session.
func (s *Server) currentUser(r *http.Request) (core.User, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return core.User{}, false // no cookie at all
	}

	user, err := s.db.UserBySession(r.Context(), c.Value)
	if err != nil {
		return core.User{}, false // unknown or expired session
	}
	return user, true
}

// userFromContext returns the user that requireUser stored, or nil.
func userFromContext(r *http.Request) *core.User {
	user, _ := r.Context().Value(userKey{}).(*core.User)
	return user
}
