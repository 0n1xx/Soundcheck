package web

import (
	"context"
	"errors"
	"html"
	"log"
	"net/http"
	"net/url"

	"github.com/0n1xx/soundcheck/internal/store"
)

// handleConnectTelegram makes a one-time code and sends the browser to the
// bot: t.me/<bot>?start=<code>. Pressing Start in Telegram sends the code
// back to us (see TelegramStart), and that's how we know whose chat it is.
func (s *Server) handleConnectTelegram(w http.ResponseWriter, r *http.Request) {
	if s.tg == nil || s.botName == "" {
		s.renderDashboard(w, r, http.StatusServiceUnavailable, "settings.html",
			"Telegram isn't available right now.")
		return
	}

	user := userFromContext(r)
	code, err := s.db.CreateTelegramLink(r.Context(), user.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}

	link := "https://t.me/" + url.PathEscape(s.botName) + "?start=" + url.QueryEscape(code)
	http.Redirect(w, r, link, http.StatusSeeOther)
}

// handleDisconnectTelegram stops Telegram alerts for the logged-in user.
func (s *Server) handleDisconnectTelegram(w http.ResponseWriter, r *http.Request) {
	user := userFromContext(r)
	if err := s.db.DisconnectTelegram(r.Context(), user.ID); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, "/settings#telegram", http.StatusSeeOther)
}

// TelegramStart handles "/start <code>" sent to the bot. main.go passes it
// to the bot's Listen, so it runs in the bot's goroutine, not in a request.
func (s *Server) TelegramStart(ctx context.Context, chatID int64, code string) {
	var reply string

	if code == "" {
		// Someone found the bot on their own, without the link from the site.
		reply = "Hi! To get alerts here, open Soundcheck → Settings and press <b>Connect Telegram</b>."
	} else {
		email, err := s.db.ConnectTelegram(ctx, code, chatID)
		switch {
		case errors.Is(err, store.ErrNotFound):
			reply = "This link has expired. Open Soundcheck → Settings and press <b>Connect Telegram</b> again."
		case err != nil:
			log.Printf("telegram start: %v", err)
			reply = "Something went wrong. Please try again in a minute."
		default:
			log.Println("telegram: a user connected their chat")
			reply = "✅ <b>Soundcheck is connected</b> to " + html.EscapeString(email) +
				".\n\nYou'll get a message here when your artists announce a show."
		}
	}

	if err := s.tg.SendMessage(ctx, chatID, reply); err != nil {
		log.Printf("telegram reply: %v", err)
	}
}
