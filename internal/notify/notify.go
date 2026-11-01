// Package notify delivers alerts to users: Telegram today, email later.
package notify

import "context"

// Message is one alert, written once and delivered through any channel.
type Message struct {
	Title string // e.g. "New show: 2hollis in Toronto"
	Text  string // the details, plain text
	URL   string // where to buy tickets; may be empty
}

// Recipient says where one user can be reached.
type Recipient struct {
	Email          string
	TelegramChatID int64 // 0 if the user hasn't connected Telegram
}

// Notifier delivers a message to a user. Each channel (Telegram, email)
// is its own Notifier; the worker doesn't care which one it talks to.
type Notifier interface {
	Notify(ctx context.Context, to Recipient, msg Message) error
}
