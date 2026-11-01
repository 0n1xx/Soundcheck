package core

import "time"

// User is someone who signed up for Soundcheck.
type User struct {
	ID           int64
	Email        string
	PasswordHash string // empty for people who sign in with Google
	City         string
	CreatedAt    time.Time

	// EmailVerified is true once the user clicked the confirmation link
	// (or signed in with Google, which has already checked the address).
	// Alerts are only emailed to verified addresses.
	EmailVerified bool

	// TelegramChatID is where Telegram alerts go; 0 if not connected.
	TelegramChatID int64

	// Which alerts the user wants (Settings → Notifications).
	NotifyNew     bool // a new show is announced
	Notify24h     bool // a day before tickets go on sale
	Notify15m     bool // 15 minutes before tickets go on sale
	NotifyChanges bool // a show is cancelled, postponed or moved
}
