package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramProvider sends messages via the Telegram Bot API.
type TelegramProvider struct{}

// Send posts a MarkdownV2 message to a Telegram chat.
// Required config fields: bot_token, chat_id
func (p *TelegramProvider) Send(ctx context.Context, cfg map[string]string, e Event) error {
	token, err := RequiredField(cfg, "bot_token")
	if err != nil {
		return err
	}
	chatID, err := RequiredField(cfg, "chat_id")
	if err != nil {
		return err
	}

	icon := "✅"
	if e.Status == 0 {
		icon = "🔴"
	}

	text := fmt.Sprintf(
		"%s *%s* is *%s*\n\nURL: `%s`\nLatency: %dms\nMessage: %s\nTime: %s",
		icon,
		escapeTelegramMD(e.MonitorName),
		e.StatusText(),
		escapeTelegramMD(e.MonitorURL),
		e.LatencyMs,
		escapeTelegramMD(e.Message),
		time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	)

	payload := map[string]string{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram: marshal: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: API returned %d", resp.StatusCode)
	}
	return nil
}

// escapeTelegramMD escapes special characters for Telegram Markdown.
func escapeTelegramMD(s string) string {
	replacer := []struct{ from, to string }{
		{"_", "\\_"}, {"*", "\\*"}, {"`", "\\`"}, {"[", "\\["},
	}
	for _, r := range replacer {
		out := ""
		for _, c := range s {
			if string(c) == r.from {
				out += r.to
			} else {
				out += string(c)
			}
		}
		s = out
	}
	return s
}
