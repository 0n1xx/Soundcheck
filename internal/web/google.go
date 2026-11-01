package web

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/0n1xx/soundcheck/internal/store"
)

// Google's OAuth addresses.
const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"
)

// stateCookie holds a random value for the trip to Google and back.
// Google returns it unchanged; if it doesn't match, the request didn't
// start on our site and is refused.
const stateCookie = "soundcheck_oauth_state"

// GoogleConfig is the OAuth client created in Google Cloud Console.
// An empty ClientID turns Google sign-in off.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string // where Google sends people back: .../auth/google/callback
}

// googleAccount is what we learn about the person from Google.
type googleAccount struct {
	Sub           string `json:"sub"` // Google's ID for the account; never changes
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// handleGoogleStart sends the browser to Google's "Choose an account" page.
func (s *Server) handleGoogleStart(w http.ResponseWriter, r *http.Request) {
	if s.google.ClientID == "" {
		s.render(w, http.StatusServiceUnavailable, "login.html",
			pageData{Error: "Google sign-in isn't available right now."})
		return
	}

	// A random value only this browser knows, checked when Google sends it back.
	state := rand.Text()
	http.SetCookie(w, &http.Cookie{
		Name:     stateCookie,
		Value:    state,
		Path:     "/auth/google",
		MaxAge:   600, // 10 minutes to pick an account
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode, // still sent when Google redirects back
	})

	q := url.Values{
		"client_id":     {s.google.ClientID},
		"redirect_uri":  {s.google.RedirectURL},
		"response_type": {"code"},         // "give me a one-time code"
		"scope":         {"openid email"}, // we only ask for the email address
		"state":         {state},
		"prompt":        {"select_account"}, // always show the account picker
	}
	http.Redirect(w, r, googleAuthURL+"?"+q.Encode(), http.StatusSeeOther)
}

// handleGoogleCallback is where Google sends the browser back with a code.
// The code is swapped for the person's email, then they're logged in.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	showError := func(status int, msg string) {
		s.render(w, status, "login.html", pageData{Error: msg})
	}

	// 1. The state must match the cookie we set before sending them to Google.
	c, err := r.Cookie(stateCookie)
	if err != nil || c.Value == "" || c.Value != r.FormValue("state") {
		showError(http.StatusBadRequest, "Signing in with Google took too long. Please try again.")
		return
	}
	// Used up: delete the cookie.
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Path: "/auth/google", MaxAge: -1,
		HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode,
	})

	// 2. The person pressed Cancel on Google's page.
	if r.FormValue("error") != "" {
		showError(http.StatusBadRequest, "Google sign-in was cancelled.")
		return
	}

	// 3. Swap the code for the account's email.
	account, err := s.googleAccount(r.Context(), r.FormValue("code"))
	if err != nil {
		log.Printf("google sign-in: %v", err)
		showError(http.StatusBadGateway, "Couldn't sign in with Google. Please try again.")
		return
	}
	if !account.EmailVerified {
		showError(http.StatusBadRequest, "Your Google email address isn't verified.")
		return
	}

	// 4. Find or create the user, then log them in.
	user, created, err := s.db.SignInWithGoogle(r.Context(), account.Sub, account.Email)
	if errors.Is(err, store.ErrEmailTaken) {
		showError(http.StatusConflict, "This email is already linked to a different Google account.")
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}

	// New users start by adding artists, like after a normal sign-up.
	next := "/shows"
	if created {
		next = "/artists"
	}
	s.startSession(w, r, user.ID, next)
}

// googleAccount swaps the one-time code for an access token, then asks
// Google whose account it is. Both requests go from our server to Google
// directly, so the client secret never reaches the browser.
func (s *Server) googleAccount(ctx context.Context, code string) (googleAccount, error) {
	// a. Code → access token.
	form := url.Values{
		"code":          {code},
		"client_id":     {s.google.ClientID},
		"client_secret": {s.google.ClientSecret},
		"redirect_uri":  {s.google.RedirectURL},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return googleAccount{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var token struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"` // e.g. "invalid_grant" for an old code
	}
	if err := s.googleJSON(req, &token); err != nil {
		return googleAccount{}, fmt.Errorf("token: %w", err)
	}
	if token.AccessToken == "" {
		return googleAccount{}, fmt.Errorf("token: %s", token.Error)
	}

	// b. Access token → who is this?
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return googleAccount{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	var account googleAccount
	if err := s.googleJSON(req, &account); err != nil {
		return googleAccount{}, fmt.Errorf("userinfo: %w", err)
	}
	if account.Sub == "" || account.Email == "" {
		return googleAccount{}, errors.New("userinfo: no account ID or email")
	}
	return account, nil
}

// googleJSON sends a request to Google and decodes the JSON answer.
func (s *Server) googleJSON(req *http.Request, out any) error {
	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Google's answers are small; never read more than 1 MB.
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
