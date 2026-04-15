package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeTransport is a pure in-memory RoundTripper that captures the request
// and returns a preconfigured response. No network listener is used.
type fakeTransport struct {
	lastReq    *http.Request
	lastBody   []byte
	statusCode int
	respBody   string
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	f.lastReq = req
	if req.Body != nil {
		f.lastBody, _ = io.ReadAll(req.Body)
		req.Body.Close()
	}
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader(f.respBody)),
		Header:     make(http.Header),
	}, nil
}

func TestTelegramSendText(t *testing.T) {
	ft := &fakeTransport{
		statusCode: 200,
		respBody:   `{"ok":true}`,
	}
	bot := &telegramBot{
		token:  "test-token",
		chatID: "12345",
		client: &http.Client{Transport: ft},
	}

	err := bot.SendText(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	if ft.lastReq == nil {
		t.Fatal("no request captured")
	}
	if !strings.HasSuffix(ft.lastReq.URL.Path, "/sendMessage") {
		t.Errorf("path = %q, want suffix /sendMessage", ft.lastReq.URL.Path)
	}
	if ft.lastReq.Method != "POST" {
		t.Errorf("method = %q, want POST", ft.lastReq.Method)
	}
	if ct := ft.lastReq.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(ft.lastBody, &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["chat_id"] != "12345" {
		t.Errorf("chat_id = %q, want 12345", body["chat_id"])
	}
	if body["text"] != "hello world" {
		t.Errorf("text = %q, want hello world", body["text"])
	}
}

func TestTelegramSendTextAPIError(t *testing.T) {
	ft := &fakeTransport{
		statusCode: 401,
		respBody:   `{"ok":false,"description":"Unauthorized"}`,
	}
	bot := &telegramBot{
		token:  "bad-token",
		chatID: "12345",
		client: &http.Client{Transport: ft},
	}

	err := bot.SendText(context.Background(), "test")
	if err == nil {
		t.Fatal("SendText() expected error for 401")
	}
	if !strings.Contains(err.Error(), "Unauthorized") {
		t.Errorf("error should contain Unauthorized, got: %v", err)
	}
}

func TestTelegramSendTextOKFalse(t *testing.T) {
	ft := &fakeTransport{
		statusCode: 200,
		respBody:   `{"ok":false,"description":"chat not found"}`,
	}
	bot := &telegramBot{
		token:  "tok",
		chatID: "999",
		client: &http.Client{Transport: ft},
	}

	err := bot.SendText(context.Background(), "test")
	if err == nil {
		t.Fatal("SendText() expected error for ok=false")
	}
	if !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("error should contain 'chat not found', got: %v", err)
	}
}

func TestTelegramSendPhoto(t *testing.T) {
	ft := &fakeTransport{
		statusCode: 200,
		respBody:   `{"ok":true}`,
	}
	bot := &telegramBot{
		token:  "test-token",
		chatID: "12345",
		client: &http.Client{Transport: ft},
	}

	err := bot.SendPhoto(context.Background(), []byte("fake-png-data"), "my caption")
	if err != nil {
		t.Fatalf("SendPhoto() error = %v", err)
	}

	if ft.lastReq == nil {
		t.Fatal("no request captured")
	}
	if !strings.HasSuffix(ft.lastReq.URL.Path, "/sendPhoto") {
		t.Errorf("path = %q, want suffix /sendPhoto", ft.lastReq.URL.Path)
	}
	ct := ft.lastReq.Header.Get("Content-Type")
	if !strings.Contains(ct, "multipart/form-data") {
		t.Errorf("content-type = %q, want multipart/form-data", ct)
	}

	if !bytes.Contains(ft.lastBody, []byte("fake-png-data")) {
		t.Error("body should contain the photo data")
	}
	if !bytes.Contains(ft.lastBody, []byte("my caption")) {
		t.Error("body should contain the caption")
	}
}

func TestTelegramSendPhotoAPIError(t *testing.T) {
	ft := &fakeTransport{
		statusCode: 400,
		respBody:   `{"ok":false,"description":"Bad Request: wrong file identifier"}`,
	}
	bot := &telegramBot{
		token:  "tok",
		chatID: "123",
		client: &http.Client{Transport: ft},
	}

	err := bot.SendPhoto(context.Background(), []byte("x"), "cap")
	if err == nil {
		t.Fatal("SendPhoto() expected error for 400")
	}
	if !strings.Contains(err.Error(), "wrong file identifier") {
		t.Errorf("error should mention API description, got: %v", err)
	}
}
