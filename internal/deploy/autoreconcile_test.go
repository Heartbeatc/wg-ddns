package deploy

import (
	"strings"
	"testing"

	"wg-ddns/internal/model"
)

func TestReconcileScriptHasDNSDriftDetection(t *testing.T) {
	checks := []struct {
		label   string
		pattern string
	}{
		{"detects IP change", `IP_CHANGED=true`},
		{"detects DNS drift", `DNS_DRIFT=true`},
		{"checks record content", `REC_CONTENT`},
		{"checks record TTL", `REC_TTL`},
		{"checks record proxied", `REC_PROXIED`},
		{"exits when no change and no drift", `"$IP_CHANGED" = "false" ] && [ "$DNS_DRIFT" = "false" ]`},
		{"only restarts WG on IP change", `"$IP_CHANGED" = "true" ] && [ -n "$EXIT_SSH_HOST"`},
		{"saves state only when all succeed", `"$FAILED" ] && [ -n "$UPDATED"`},
		{"tracks drifted domains", `DRIFTED=`},
		{"logs trigger reason", `TRIGGER=`},
		{"marks missing record", `missing`},
		{"notification includes trigger", `触发原因: $TRIGGER`},
	}

	for _, c := range checks {
		if !strings.Contains(reconcileScript, c.pattern) {
			t.Errorf("script missing %s: expected to find %q", c.label, c.pattern)
		}
	}
}

func TestReconcileScriptIPUnchangedDNSOK(t *testing.T) {
	// When IP equals LAST_IP and DNS_DRIFT is false, script should exit 0
	if !strings.Contains(reconcileScript, `exit 0`) {
		t.Error("script should contain early exit 0 for no-op case")
	}
	if !strings.Contains(reconcileScript, `"$IP_CHANGED" = "false" ] && [ "$DNS_DRIFT" = "false" ]`) {
		t.Error("script should check both IP_CHANGED and DNS_DRIFT before exiting")
	}
}

func TestReconcileScriptDNSDriftTrigger(t *testing.T) {
	// Content drift: record content != CURRENT_IP
	if !strings.Contains(reconcileScript, `"$REC_CONTENT" != "$CURRENT_IP"`) {
		t.Error("script should detect content drift by comparing REC_CONTENT to CURRENT_IP")
	}
	// TTL drift
	if !strings.Contains(reconcileScript, `"$REC_TTL" != "$RECORD_TTL"`) {
		t.Error("script should detect TTL drift")
	}
	// Proxied drift
	if !strings.Contains(reconcileScript, `"$REC_PROXIED" != "$CF_PROXIED"`) {
		t.Error("script should detect proxied drift")
	}
}

func TestReconcileScriptWGOnlyOnIPChange(t *testing.T) {
	// WG restart should be gated on IP_CHANGED=true
	if !strings.Contains(reconcileScript, `"$IP_CHANGED" = "true" ] && [ -n "$EXIT_SSH_HOST"`) {
		t.Error("WG restart must be conditional on IP_CHANGED=true")
	}
}

func TestReconcileEnvConfig(t *testing.T) {
	project := model.Project{
		Project: "test-project",
		Cloudflare: model.Cloudflare{
			Zone:    "example.com",
			TTL:     120,
			Proxied: false,
		},
		Domains: model.Domains{
			Entry:     "entry.example.com",
			Panel:     "entry.example.com",
			WireGuard: "wg.example.com",
		},
		Nodes: model.Nodes{
			HK: model.Node{
				Host:      "5.6.7.8",
				SSHHost:   "ssh-exit.example.com",
				WGService: "wg-quick@wg0",
				SSH: model.SSH{
					User: "root",
					Port: 22,
				},
			},
		},
		Notifications: model.Notifications{
			Enabled: true,
			Telegram: model.TelegramConfig{
				BotToken: "test-token",
				ChatID:   "-123",
			},
		},
	}

	env := reconcileEnvConfig(project, "cf-token-value", "tg-token-value")

	checks := map[string]string{
		"CF_API_TOKEN":    "cf-token-value",
		"CF_ZONE":         "example.com",
		"DOMAINS":         "entry.example.com wg.example.com",
		"RECORD_TTL":      "120",
		"CF_PROXIED":      "false",
		"EXIT_SSH_HOST":   "ssh-exit.example.com",
		"EXIT_SSH_PORT":   "22",
		"EXIT_SSH_USER":   "root",
		"EXIT_WG_SERVICE": "wg-quick@wg0",
		"TG_ENABLED":      "true",
		"TG_BOT_TOKEN":    "tg-token-value",
		"TG_CHAT_ID":      "-123",
		"PROJECT_NAME":    "test-project",
	}

	for key, want := range checks {
		line := key + "=" + want
		if !strings.Contains(env, line) {
			t.Errorf("env config missing %q, got:\n%s", line, env)
		}
	}
}

func TestReconcileEnvConfigProxiedTrue(t *testing.T) {
	project := model.Project{
		Cloudflare: model.Cloudflare{
			Zone:    "example.com",
			TTL:     60,
			Proxied: true,
		},
		Domains: model.Domains{Entry: "a.example.com"},
		Nodes: model.Nodes{
			HK: model.Node{
				WGService: "wg-quick@wg0",
				SSH:       model.SSH{User: "root", Port: 0},
			},
		},
	}

	env := reconcileEnvConfig(project, "tok", "")
	if !strings.Contains(env, "CF_PROXIED=true") {
		t.Errorf("expected CF_PROXIED=true, got:\n%s", env)
	}
	// Port 0 should default to 22
	if !strings.Contains(env, "EXIT_SSH_PORT=22") {
		t.Errorf("expected EXIT_SSH_PORT=22, got:\n%s", env)
	}
	// No TG token → disabled
	if !strings.Contains(env, "TG_ENABLED=false") {
		t.Errorf("expected TG_ENABLED=false, got:\n%s", env)
	}
}
