package web

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLength = 8
	// bcrypt only looks at the first 72 bytes, so longer passwords
	// are rejected instead of being silently cut.
	maxPasswordBytes = 72
)

// commonPasswords are among the most leaked passwords. Shorter ones are
// already rejected by the length rule, so only 8+ characters are listed.
var commonPasswords = map[string]bool{
	"password": true, "password1": true, "password123": true, "passw0rd": true,
	"p@ssw0rd": true, "qwertyuiop": true, "qwerty123": true, "qwerty12": true,
	"1q2w3e4r": true, "1q2w3e4r5t": true, "1qaz2wsx": true, "qazwsxedc": true,
	"asdfghjk": true, "zxcvbnm1": true, "iloveyou": true, "iloveyou1": true,
	"sunshine": true, "princess": true, "football": true, "baseball": true,
	"welcome1": true, "welcome123": true, "admin123": true, "letmein1": true,
	"trustno1": true, "superman": true, "starwars": true, "whatever": true,
	"dragon12": true, "monkey12": true, "abc12345": true, "michael1": true,
	"jennifer": true, "computer": true, "internet": true, "soundcheck": true,
	"soundcheck1": true, "concert1": true, "music123": true,
}

// validatePassword checks a new password and explains what's wrong with it.
// The messages are shown to the user as they are.
func validatePassword(password, email string) error {
	if utf8.RuneCountInString(password) < minPasswordLength {
		return errors.New("Password must be at least 8 characters.")
	}
	if len(password) > maxPasswordBytes {
		return errors.New("Password is too long. Use 72 characters or fewer.")
	}

	lower := strings.ToLower(password)

	if commonPasswords[lower] {
		return errors.New("This password is too common. Pick something less predictable.")
	}
	if uniqueRunes(lower) < 4 {
		return errors.New("Password has too many repeated characters.")
	}
	if isSequence(lower) {
		return errors.New("Password can't be a simple sequence like 12345678 or abcdefgh.")
	}

	// "vlad@gmail.com" shouldn't be able to use "vlad2026".
	name, _, _ := strings.Cut(strings.ToLower(email), "@")
	if len(name) >= 3 && strings.Contains(lower, name) {
		return errors.New("Password shouldn't contain your email address.")
	}

	if !hasLetterAndDigit(password) {
		return errors.New("Password must include at least one letter and one number.")
	}
	return nil
}

// hashPassword turns a valid password into a bcrypt hash that is safe to store.
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// checkPassword reports whether a password matches a stored hash.
func checkPassword(hash, password string) bool {
	if hash == "" {
		return false // Google-only account, no password set
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// uniqueRunes counts different characters: "abababab" has only 2.
func uniqueRunes(s string) int {
	seen := map[rune]bool{}
	for _, r := range s {
		seen[r] = true
	}
	return len(seen)
}

// isSequence reports whether every character is one step up or down
// from the previous one, like "12345678" or "hgfedcba".
func isSequence(s string) bool {
	runes := []rune(s)
	if len(runes) < 2 {
		return false
	}

	step := runes[1] - runes[0]
	if step != 1 && step != -1 {
		return false
	}
	for i := 2; i < len(runes); i++ {
		if runes[i]-runes[i-1] != step {
			return false
		}
	}
	return true
}

// hasLetterAndDigit reports whether s has at least one letter and one digit.
func hasLetterAndDigit(s string) bool {
	var letter, digit bool
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	return letter && digit
}
