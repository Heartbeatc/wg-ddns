package notify

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"wg-ddns/internal/model"
)

// Notifier sends messages to an external channel.
// Implementations must be safe to call even when misconfigured;
// errors are always returned to the caller so the app layer can
// decide whether to log or ignore them.
type Notifier interface {
	SendText(ctx context.Context, text string) error
	SendPhoto(ctx context.Context, photo []byte, caption string) error
}

// FromConfig returns a Notifier based on the project's notification settings.
// If notifications are disabled or the config is incomplete, a no-op
// notifier is returned and a warning is written to w (if non-nil).
func FromConfig(cfg model.Notifications, w io.Writer) Notifier {
	if !cfg.Enabled {
		return &nopNotifier{}
	}

	token := strings.TrimSpace(cfg.Telegram.BotToken)
	if token == "" && cfg.Telegram.BotTokenEnv != "" {
		token = strings.TrimSpace(os.Getenv(cfg.Telegram.BotTokenEnv))
	}
	chatID := strings.TrimSpace(cfg.Telegram.ChatID)

	if token == "" || chatID == "" {
		if w != nil {
			fmt.Fprintln(w, "通知已启用但 Telegram 配置不完整（缺少 bot_token 或 chat_id），跳过通知。")
		}
		return &nopNotifier{}
	}

	return NewTelegram(token, chatID)
}

// IsNop returns true if n is a no-op notifier (notifications disabled or
// misconfigured). Callers can use this to skip expensive enrichment work.
func IsNop(n Notifier) bool {
	_, ok := n.(*nopNotifier)
	return ok
}

type nopNotifier struct{}

func (n *nopNotifier) SendText(ctx context.Context, text string) error                   { return nil }
func (n *nopNotifier) SendPhoto(ctx context.Context, photo []byte, caption string) error { return nil }

// Fire sends a text notification via notif. If sending fails, the error is
// logged to w but never propagated — notifications must not block the main flow.
func Fire(w io.Writer, notif Notifier, msg string) {
	if IsNop(notif) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), telegramTimeout)
	defer cancel()
	if err := notif.SendText(ctx, msg); err != nil {
		fmt.Fprintf(w, "通知发送失败: %v\n", err)
	}
}
