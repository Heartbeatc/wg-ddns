package notify

import (
	"bytes"
	"context"
	"os"
	"testing"

	"wg-ddns/internal/model"
)

func TestNopNotifier(t *testing.T) {
	n := &nopNotifier{}
	if err := n.SendText(context.Background(), "test"); err != nil {
		t.Fatalf("nopNotifier.SendText() error = %v", err)
	}
	if err := n.SendPhoto(context.Background(), []byte("png"), "cap"); err != nil {
		t.Fatalf("nopNotifier.SendPhoto() error = %v", err)
	}
}

func TestIsNop(t *testing.T) {
	n := &nopNotifier{}
	if !IsNop(n) {
		t.Fatal("IsNop(nopNotifier) = false, want true")
	}
	tg := NewTelegram("tok", "123")
	if IsNop(tg) {
		t.Fatal("IsNop(telegramBot) = true, want false")
	}
}

func TestFromConfigDisabled(t *testing.T) {
	cfg := model.Notifications{Enabled: false}
	n := FromConfig(cfg, nil)
	if !IsNop(n) {
		t.Fatal("FromConfig(disabled) should return nop notifier")
	}
}

func TestFromConfigMissingToken(t *testing.T) {
	var buf bytes.Buffer
	cfg := model.Notifications{
		Enabled: true,
		Telegram: model.TelegramConfig{
			ChatID: "123",
		},
	}
	n := FromConfig(cfg, &buf)
	if !IsNop(n) {
		t.Fatal("FromConfig(no token) should return nop notifier")
	}
	if buf.Len() == 0 {
		t.Fatal("FromConfig(no token) should write a warning")
	}
}

func TestFromConfigMissingChatID(t *testing.T) {
	var buf bytes.Buffer
	cfg := model.Notifications{
		Enabled: true,
		Telegram: model.TelegramConfig{
			BotToken: "tok",
		},
	}
	n := FromConfig(cfg, &buf)
	if !IsNop(n) {
		t.Fatal("FromConfig(no chat_id) should return nop notifier")
	}
}

func TestFromConfigComplete(t *testing.T) {
	cfg := model.Notifications{
		Enabled: true,
		Telegram: model.TelegramConfig{
			BotToken: "123:ABC",
			ChatID:   "456",
		},
	}
	n := FromConfig(cfg, nil)
	if IsNop(n) {
		t.Fatal("FromConfig(complete) should return telegram notifier")
	}
}

func TestFromConfigEnvToken(t *testing.T) {
	os.Setenv("TEST_TG_TOKEN_1234", "from-env")
	defer os.Unsetenv("TEST_TG_TOKEN_1234")

	cfg := model.Notifications{
		Enabled: true,
		Telegram: model.TelegramConfig{
			BotTokenEnv: "TEST_TG_TOKEN_1234",
			ChatID:      "456",
		},
	}
	n := FromConfig(cfg, nil)
	if IsNop(n) {
		t.Fatal("FromConfig(env token) should return telegram notifier")
	}
}

func TestFireNop(t *testing.T) {
	var buf bytes.Buffer
	n := &nopNotifier{}
	Fire(&buf, n, "test message")
	if buf.Len() != 0 {
		t.Fatal("Fire with nop should produce no output")
	}
}
