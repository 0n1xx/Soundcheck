package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"
	"time"
)

// telegramAPI is the Bot API address; the token is added after "bot".
const telegramAPI = "https://api.telegram.org/bot"

// Telegram sends messages through a Telegram bot.
type Telegram struct {
	token      string
	httpClient *http.Client // for sending messages
	pollClient *http.Client // for getUpdates, which waits up to pollTimeout
}

// pollTimeout is how long Telegram holds a getUpdates request open
// while waiting for new messages (long polling).
const pollTimeout = 25 * time.Second

// NewTelegram creates a Telegram notifier for the bot with this token.
func NewTelegram(token string) *Telegram {
	return &Telegram{
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		// A little longer than pollTimeout, so the wait itself isn't cut off.
		pollClient: &http.Client{Timeout: pollTimeout + 10*time.Second},
	}
}

// Notify sends msg to the user's Telegram chat.
// Users who haven't connected Telegram are skipped quietly.
func (t *Telegram) Notify(ctx context.Context, to Recipient, msg Message) error {
	if to.TelegramChatID == 0 {
		return nil
	}
	return t.SendMessage(ctx, to.TelegramChatID, formatTelegram(msg))
}

// SendMessage sends HTML text to one chat.
func (t *Telegram) SendMessage(ctx context.Context, chatID int64, text string) error {
	body, err := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML", // lets us use <b> and <a> in the text
	})
	if err != nil {
		return err
	}

	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"` // Telegram's error message
	}
	if err := t.call(ctx, t.httpClient, "sendMessage", body, &result); err != nil {
		return err
	}
	if !result.OK {
		return fmt.Errorf("telegram sendMessage: %s", result.Description)
	}
	return nil
}

// call sends one request to the Bot API and decodes the JSON answer into out.
func (t *Telegram) call(ctx context.Context, client *http.Client, method string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		telegramAPI+t.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		// The error text contains the URL with the token, so don't pass it on.
		return fmt.Errorf("telegram %s: request failed", method)
	}
	defer resp.Body.Close()

	return json.NewDecoder(resp.Body).Decode(out)
}

// StartHandler is called for every /start message the bot gets.
// code is what came after /start in the bot link; "" if nothing.
type StartHandler func(ctx context.Context, chatID int64, code string)

// update is one incoming event for the bot. We only need messages.
type update struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		Text string `json:"text"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"` // nil for other kinds of updates
}

// Listen waits for messages to the bot and passes every /start to onStart,
// until ctx is cancelled. It uses long polling: each getUpdates request
// stays open until a message arrives or pollTimeout passes, so the bot
// answers instantly without asking Telegram every second.
func (t *Telegram) Listen(ctx context.Context, onStart StartHandler) {
	log.Println("telegram: listening for /start")

	// offset tells Telegram which updates we've already handled.
	var offset int64
	for {
		updates, err := t.getUpdates(ctx, offset)
		if ctx.Err() != nil {
			log.Println("telegram: stopped")
			return
		}
		if err != nil {
			log.Printf("telegram: %v", err)
			// Wait a bit before trying again, unless we're shutting down.
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}

		for _, u := range updates {
			offset = u.UpdateID + 1 // "I've seen this one, don't send it again"
			if u.Message == nil {
				continue
			}
			// The bot link t.me/<bot>?start=<code> arrives as "/start <code>".
			command, code, _ := strings.Cut(strings.TrimSpace(u.Message.Text), " ")
			if command == "/start" {
				onStart(ctx, u.Message.Chat.ID, strings.TrimSpace(code))
			}
		}
	}
}

// getUpdates asks Telegram for new updates, waiting up to pollTimeout.
func (t *Telegram) getUpdates(ctx context.Context, offset int64) ([]update, error) {
	body, err := json.Marshal(map[string]any{
		"offset":          offset,
		"timeout":         int(pollTimeout.Seconds()),
		"allowed_updates": []string{"message"},
	})
	if err != nil {
		return nil, err
	}

	var result struct {
		OK          bool     `json:"ok"`
		Description string   `json:"description"`
		Result      []update `json:"result"`
	}
	if err := t.call(ctx, t.pollClient, "getUpdates", body, &result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram getUpdates: %s", result.Description)
	}
	return result.Result, nil
}

// formatTelegram turns a Message into Telegram HTML:
// a bold title, the details and a link to the tickets.
func formatTelegram(msg Message) string {
	var b strings.Builder
	b.WriteString("<b>" + html.EscapeString(msg.Title) + "</b>")
	if msg.Text != "" {
		b.WriteString("\n\n" + html.EscapeString(msg.Text))
	}
	if msg.URL != "" {
		b.WriteString("\n\n<a href=\"" + html.EscapeString(msg.URL) + "\">Get tickets</a>")
	}
	return b.String()
}

// Make sure Telegram really is a Notifier; the build fails otherwise.
var _ Notifier = (*Telegram)(nil)
