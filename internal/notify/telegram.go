package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const telegramTimeout = 10 * time.Second

type telegramBot struct {
	token  string
	chatID string
	client *http.Client
}

// NewTelegram creates a Notifier that sends messages via the Telegram Bot API.
func NewTelegram(token, chatID string) Notifier {
	return &telegramBot{
		token:  token,
		chatID: chatID,
		client: &http.Client{Timeout: telegramTimeout},
	}
}

func (t *telegramBot) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, method)
}

func (t *telegramBot) SendText(ctx context.Context, text string) error {
	body, err := json.Marshal(map[string]string{
		"chat_id": t.chatID,
		"text":    text,
	})
	if err != nil {
		return fmt.Errorf("telegram: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL("sendMessage"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return t.doRequest(req)
}

func (t *telegramBot) SendPhoto(ctx context.Context, photo []byte, caption string) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	if err := w.WriteField("chat_id", t.chatID); err != nil {
		return fmt.Errorf("telegram: write chat_id: %w", err)
	}
	if caption != "" {
		if err := w.WriteField("caption", caption); err != nil {
			return fmt.Errorf("telegram: write caption: %w", err)
		}
	}

	part, err := w.CreateFormFile("photo", "image.png")
	if err != nil {
		return fmt.Errorf("telegram: create photo field: %w", err)
	}
	if _, err := part.Write(photo); err != nil {
		return fmt.Errorf("telegram: write photo: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("telegram: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL("sendPhoto"), &buf)
	if err != nil {
		return fmt.Errorf("telegram: new request: %w", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	return t.doRequest(req)
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func (t *telegramBot) doRequest(req *http.Request) error {
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var tgResp telegramResponse
		if json.Unmarshal(respBody, &tgResp) == nil && tgResp.Description != "" {
			return fmt.Errorf("telegram: API error %d: %s", resp.StatusCode, tgResp.Description)
		}
		return fmt.Errorf("telegram: HTTP %d", resp.StatusCode)
	}

	var tgResp telegramResponse
	if json.Unmarshal(respBody, &tgResp) == nil && !tgResp.OK {
		return fmt.Errorf("telegram: API error: %s", tgResp.Description)
	}

	return nil
}
