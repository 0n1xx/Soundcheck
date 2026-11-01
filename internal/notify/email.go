package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
	"time"
)

// resendAPI is Resend's address for sending one email.
const resendAPI = "https://api.resend.com/emails"

// Email sends alerts as emails through Resend.
type Email struct {
	apiKey     string
	from       string // e.g. "Soundcheck <shows@getsoundcheck.cc>"
	baseURL    string // the site's address, for the "Manage alerts" link
	httpClient *http.Client
}

// NewEmail creates an email notifier. from must use a domain verified in Resend.
func NewEmail(apiKey, from, baseURL string) *Email {
	return &Email{
		apiKey:     apiKey,
		from:       from,
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Notify emails msg to the user.
func (e *Email) Notify(ctx context.Context, to Recipient, msg Message) error {
	if to.Email == "" {
		return nil
	}

	body, err := json.Marshal(map[string]any{
		"from":    e.from,
		"to":      []string{to.Email},
		"subject": msg.Title,
		"html":    e.formatHTML(msg),
		"text":    e.formatText(msg), // for mail apps that don't show HTML
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPI, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()

	// Resend answers 200 with the email's ID, or 4xx/5xx with an error message.
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("resend: %s: %s", resp.Status, bytes.TrimSpace(detail))
	}
	return nil
}

// formatText is the plain-text version of the email.
func (e *Email) formatText(msg Message) string {
	var b strings.Builder
	b.WriteString(msg.Title + "\n\n")
	if msg.Text != "" {
		b.WriteString(msg.Text + "\n\n")
	}
	if msg.URL != "" {
		b.WriteString(buttonLabel(msg) + ": " + msg.URL + "\n\n")
	}
	if msg.Footer != "" {
		b.WriteString(msg.Footer + "\n")
	} else {
		b.WriteString("Manage your alerts: " + e.baseURL + "/settings\n")
	}
	return b.String()
}

// formatHTML is a small email in the site's style: a window with a title bar,
// the details, a blue button and a footer. Email apps support only simple
// HTML, so everything is inline styles and tables, no CSS files.
func (e *Email) formatHTML(msg Message) string {
	esc := html.EscapeString
	// Line breaks in the text become <br> in HTML.
	text := strings.ReplaceAll(esc(msg.Text), "\n", "<br>")

	// The default footer links to Settings so people can turn alerts off.
	footer := `You follow this artist on Soundcheck. <a href="` + esc(e.baseURL) + `/settings" style="color:#888">Manage your alerts</a>`
	if msg.Footer != "" {
		footer = esc(msg.Footer)
	}

	button := ""
	if msg.URL != "" {
		button = `<p style="margin:24px 0 0"><a href="` + esc(msg.URL) + `" style="display:inline-block;padding:10px 18px;color:#ffffff;font-weight:600;text-decoration:none;background:#3a7bd5;border-radius:6px">` + esc(buttonLabel(msg)) + `</a></p>`
	}

	return `<!doctype html>
<html><body style="margin:0;padding:24px;background:#e8e8e8;font-family:-apple-system,'Helvetica Neue',Helvetica,Arial,sans-serif;color:#222">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0"><tr><td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:520px;background:#f6f6f6;border:1px solid #c8c8c8;border-radius:8px;overflow:hidden">
<tr><td style="padding:10px 14px;background:#e4e4e4;border-bottom:1px solid #cfcfcf;font-size:13px;font-weight:600;color:#555;text-align:center">Soundcheck</td></tr>
<tr><td style="padding:24px">
<h1 style="margin:0 0 12px;font-size:20px;line-height:1.3">` + esc(msg.Title) + `</h1>
<p style="margin:0;font-size:15px;line-height:1.5;color:#444">` + text + `</p>` + button + `
</td></tr>
</table>
<p style="margin:16px 0 0;font-size:12px;color:#888">` + footer + `</p>
</td></tr></table>
</body></html>`
}

// buttonLabel is the text of the email's button.
func buttonLabel(msg Message) string {
	if msg.Button != "" {
		return msg.Button
	}
	return "Get tickets"
}

// Make sure Email really is a Notifier; the build fails otherwise.
var _ Notifier = (*Email)(nil)
