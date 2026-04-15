package deploy

import (
	"fmt"
	"io"
	"os"
	"strings"

	"wg-ddns/internal/cloudflare"
	"wg-ddns/internal/model"
	"wg-ddns/internal/sshclient"
)

const reconcileScript = `#!/bin/sh
set -eu

CONFIG="/etc/wgstack/reconcile.env"
STATE_DIR="/var/lib/wgstack"
STATE_FILE="$STATE_DIR/reconcile_last_ip"
ZONE_CACHE="$STATE_DIR/reconcile_zone_id"

if [ ! -f "$CONFIG" ]; then
  echo "ERROR: config not found: $CONFIG"
  exit 1
fi

. "$CONFIG"

# --- Step 1: Detect current public IP ---
CURRENT_IP=$(curl -4fsS --max-time 10 "$CHECK_URL" 2>/dev/null | tr -d '[:space:]')
if [ -z "$CURRENT_IP" ]; then
  echo "ERROR: failed to detect public IP"
  exit 1
fi

case "$CURRENT_IP" in
  [0-9]*.[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "ERROR: not a valid IPv4: $CURRENT_IP"; exit 1 ;;
esac

mkdir -p "$STATE_DIR"
LAST_IP=""
[ -f "$STATE_FILE" ] && LAST_IP=$(cat "$STATE_FILE")

IP_CHANGED=false
if [ "$CURRENT_IP" != "$LAST_IP" ]; then
  IP_CHANGED=true
fi

# --- Step 2: Resolve Cloudflare zone ID ---
ZONE_ID=""
[ -f "$ZONE_CACHE" ] && ZONE_ID=$(cat "$ZONE_CACHE")
if [ -z "$ZONE_ID" ]; then
  ZONE_RESP=$(curl -fsS --max-time 10 \
    -H "Authorization: Bearer $CF_API_TOKEN" \
    -H "Content-Type: application/json" \
    "https://api.cloudflare.com/client/v4/zones?name=$CF_ZONE" 2>&1)
  ZONE_ID=$(printf '%s' "$ZONE_RESP" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"//;s/"//')
  if [ -z "$ZONE_ID" ]; then
    echo "ERROR: failed to resolve zone ID for $CF_ZONE"
    exit 1
  fi
  echo "$ZONE_ID" > "$ZONE_CACHE"
fi

API_BASE="https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records"

# --- Step 3: Check each domain for drift ---
DRIFTED=""
DRIFT_DETAILS=""

for DOMAIN in $DOMAINS; do
  REC_RESP=$(curl -fsS --max-time 10 \
    -H "Authorization: Bearer $CF_API_TOKEN" \
    -H "Content-Type: application/json" \
    "$API_BASE?type=A&name=$DOMAIN" 2>&1) || true

  REC_ID=$(printf '%s' "$REC_RESP" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"//;s/"//')

  if [ -z "$REC_ID" ]; then
    DRIFTED="$DRIFTED $DOMAIN"
    DRIFT_DETAILS="$DRIFT_DETAILS $DOMAIN:missing"
    continue
  fi

  REC_CONTENT=$(printf '%s' "$REC_RESP" | grep -o '"content":"[^"]*"' | head -1 | sed 's/"content":"//;s/"//')
  REC_TTL=$(printf '%s' "$REC_RESP" | grep -o '"ttl":[0-9]*' | head -1 | sed 's/"ttl"://')
  REC_PROXIED=$(printf '%s' "$REC_RESP" | grep -o '"proxied":[a-z]*' | head -1 | sed 's/"proxied"://')

  DOMAIN_OK=true
  REASONS=""

  if [ "$REC_CONTENT" != "$CURRENT_IP" ]; then
    DOMAIN_OK=false
    REASONS="${REASONS}content=${REC_CONTENT} "
  fi
  if [ -n "$REC_TTL" ] && [ "$REC_TTL" != "$RECORD_TTL" ]; then
    DOMAIN_OK=false
    REASONS="${REASONS}ttl=${REC_TTL} "
  fi
  if [ -n "$REC_PROXIED" ] && [ "$REC_PROXIED" != "$CF_PROXIED" ]; then
    DOMAIN_OK=false
    REASONS="${REASONS}proxied=${REC_PROXIED} "
  fi

  if [ "$DOMAIN_OK" = "false" ]; then
    DRIFTED="$DRIFTED $DOMAIN"
    DRIFT_DETAILS="$DRIFT_DETAILS $DOMAIN:${REASONS}"
  fi
done

# --- Step 4: Decide whether to act ---
DNS_DRIFT=false
if [ -n "$DRIFTED" ]; then
  DNS_DRIFT=true
fi

if [ "$IP_CHANGED" = "false" ] && [ "$DNS_DRIFT" = "false" ]; then
  exit 0
fi

# Determine trigger reason for logging and notification
TRIGGER=""
if [ "$IP_CHANGED" = "true" ] && [ "$DNS_DRIFT" = "true" ]; then
  TRIGGER="IP 变化 + DNS 漂移"
elif [ "$IP_CHANGED" = "true" ]; then
  TRIGGER="IP 变化"
else
  TRIGGER="DNS 漂移"
fi

echo "$(date): Reconcile triggered — $TRIGGER"
if [ "$IP_CHANGED" = "true" ]; then
  echo "  IP: ${LAST_IP:-<none>} -> $CURRENT_IP"
fi
if [ "$DNS_DRIFT" = "true" ]; then
  echo "  Drifted records:$DRIFT_DETAILS"
fi

# --- Step 5: Fix all domains ---
UPDATED=""
FAILED=""

for DOMAIN in $DOMAINS; do
  REC_RESP=$(curl -fsS --max-time 10 \
    -H "Authorization: Bearer $CF_API_TOKEN" \
    -H "Content-Type: application/json" \
    "$API_BASE?type=A&name=$DOMAIN" 2>&1) || true
  REC_ID=$(printf '%s' "$REC_RESP" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"//;s/"//')

  DATA="{\"type\":\"A\",\"name\":\"$DOMAIN\",\"content\":\"$CURRENT_IP\",\"ttl\":$RECORD_TTL,\"proxied\":$CF_PROXIED}"

  if [ -z "$REC_ID" ]; then
    RESULT=$(curl -fsS --max-time 10 -X POST \
      -H "Authorization: Bearer $CF_API_TOKEN" \
      -H "Content-Type: application/json" \
      "$API_BASE" -d "$DATA" 2>&1) || true
  else
    RESULT=$(curl -fsS --max-time 10 -X PATCH \
      -H "Authorization: Bearer $CF_API_TOKEN" \
      -H "Content-Type: application/json" \
      "$API_BASE/$REC_ID" -d "$DATA" 2>&1) || true
  fi

  case "$RESULT" in
    *'"success":true'*)
      echo "  Fixed $DOMAIN -> $CURRENT_IP"
      UPDATED="$UPDATED $DOMAIN"
      ;;
    *)
      echo "  WARN: failed to fix $DOMAIN"
      FAILED="$FAILED $DOMAIN"
      ;;
  esac
done

# --- Step 6: Restart exit WireGuard (only on IP change) ---
WG_STATUS="skipped"
if [ "$IP_CHANGED" = "true" ] && [ -n "$EXIT_SSH_HOST" ] && [ -f "$EXIT_SSH_KEY" ]; then
  DNS_READY=false
  ATTEMPT=1
  echo "Waiting for exit node DNS to resolve $WG_ENDPOINT_DOMAIN -> $CURRENT_IP"
  while [ "$ATTEMPT" -le 12 ]; do
    RESOLVED_ON_EXIT=$(ssh -i "$EXIT_SSH_KEY" \
      -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
      -o ConnectTimeout=15 -o BatchMode=yes \
      -p "$EXIT_SSH_PORT" \
      "${EXIT_SSH_USER}@${EXIT_SSH_HOST}" \
      "getent ahostsv4 '$WG_ENDPOINT_DOMAIN' 2>/dev/null | awk 'NR==1 {print \$1; exit}'" 2>/dev/null || true)

    if [ "$RESOLVED_ON_EXIT" = "$CURRENT_IP" ]; then
      DNS_READY=true
      echo "Exit node now resolves $WG_ENDPOINT_DOMAIN -> $CURRENT_IP"
      break
    fi

    echo "  Exit resolver sees ${RESOLVED_ON_EXIT:-<none>} for $WG_ENDPOINT_DOMAIN (attempt $ATTEMPT/12)"
    ATTEMPT=$((ATTEMPT + 1))
    sleep 5
  done

  if [ "$DNS_READY" = "true" ]; then
    echo "Restarting exit WireGuard via $EXIT_SSH_USER@$EXIT_SSH_HOST"
    if ssh -i "$EXIT_SSH_KEY" \
      -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
      -o ConnectTimeout=15 -o BatchMode=yes \
      -p "$EXIT_SSH_PORT" \
      "${EXIT_SSH_USER}@${EXIT_SSH_HOST}" \
      "systemctl restart $EXIT_WG_SERVICE" 2>&1; then
      WG_STATUS="success"
      echo "Exit WireGuard restarted"
    else
      WG_STATUS="failed"
      echo "WARN: failed to restart exit WireGuard"
    fi
  else
    WG_STATUS="dns-not-ready"
    echo "WARN: exit node did not resolve $WG_ENDPOINT_DOMAIN to $CURRENT_IP in time"
  fi
fi

# --- Step 7: Persist state ---
CAN_SAVE_STATE=true
if [ -n "$FAILED" ]; then
  CAN_SAVE_STATE=false
fi
if [ "$IP_CHANGED" = "true" ] && [ "$WG_STATUS" != "success" ]; then
  CAN_SAVE_STATE=false
fi

if [ "$CAN_SAVE_STATE" = "true" ] && [ -n "$UPDATED" ]; then
  echo "$CURRENT_IP" > "$STATE_FILE"
  echo "State saved: $CURRENT_IP"
else
  echo "State NOT saved — remaining work will be retried on next run"
fi

# --- Step 8: Telegram notification ---
if [ "$TG_ENABLED" = "true" ] && [ -n "$TG_BOT_TOKEN" ] && [ -n "$TG_CHAT_ID" ]; then
  SAVED="yes"
  [ "$CAN_SAVE_STATE" != "true" ] && SAVED="no (will retry)"
  MSG="[$PROJECT_NAME] 入口自动修复

触发原因: $TRIGGER"

  if [ "$IP_CHANGED" = "true" ]; then
    MSG="$MSG
旧 IP: ${LAST_IP:-首次记录}
新 IP: $CURRENT_IP"
  else
    MSG="$MSG
当前 IP: $CURRENT_IP（未变化）
漂移记录:$DRIFT_DETAILS"
  fi

  MSG="$MSG

DNS 更新:${UPDATED:- 无}
DNS 失败:${FAILED:- 无}
状态保存: $SAVED
出口 WG: $WG_STATUS

详细质量查询: https://iplark.com/$CURRENT_IP"

  curl -fsS --max-time 10 -X POST \
    "https://api.telegram.org/bot${TG_BOT_TOKEN}/sendMessage" \
    -d "chat_id=${TG_CHAT_ID}" \
    --data-urlencode "text=${MSG}" >/dev/null 2>&1 || echo "WARN: Telegram notification failed"
fi

if [ -n "$FAILED" ]; then
  echo "ERROR: some DNS updates failed:$FAILED"
  exit 1
fi
if [ "$IP_CHANGED" = "true" ] && [ "$WG_STATUS" != "success" ]; then
  echo "ERROR: exit WireGuard refresh did not complete successfully ($WG_STATUS)"
  exit 1
fi

echo "$(date): Reconcile complete ($TRIGGER), entry IP: $CURRENT_IP"
`

const reconcileServiceUnit = `[Unit]
Description=wgstack entry node auto-reconcile
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/wgstack-reconcile
`

func reconcileTimerUnit(intervalSec int) string {
	return fmt.Sprintf(`[Unit]
Description=wgstack entry node auto-reconcile timer

[Timer]
OnBootSec=60
OnUnitActiveSec=%ds

[Install]
WantedBy=timers.target
`, intervalSec)
}

func reconcileEnvConfig(project model.Project, cfToken, tgToken string) string {
	domains := project.Domains.Unique()
	proxied := "false"
	if project.Cloudflare.Proxied {
		proxied = "true"
	}
	tgEnabled := "false"
	tgChatID := ""
	if project.Notifications.Enabled && tgToken != "" {
		tgEnabled = "true"
		tgChatID = project.Notifications.Telegram.ChatID
	}
	exitPort := project.Nodes.HK.SSH.Port
	if exitPort == 0 {
		exitPort = 22
	}
	return fmt.Sprintf(`CF_API_TOKEN=%s
CF_ZONE=%s
DOMAINS=%s
CHECK_URL=https://api.ipify.org
RECORD_TTL=%d
CF_PROXIED=%s

EXIT_SSH_HOST=%s
EXIT_SSH_PORT=%d
EXIT_SSH_USER=%s
EXIT_SSH_KEY=/etc/wgstack/exit_key
EXIT_WG_SERVICE=%s
WG_ENDPOINT_DOMAIN=%s

TG_ENABLED=%s
TG_BOT_TOKEN=%s
TG_CHAT_ID=%s
PROJECT_NAME=%s
`, cfToken, project.Cloudflare.Zone, strings.Join(domains, " "),
		project.Cloudflare.TTL, proxied,
		project.Nodes.HK.SSHAddr(), exitPort, project.Nodes.HK.SSH.User,
		project.Nodes.HK.WGService, project.Domains.WireGuard,
		tgEnabled, tgToken, tgChatID, project.Project)
}

// DeployEntryAutoReconcile installs an auto-reconcile watcher on the entry node.
// It generates a dedicated SSH key pair for entry→exit communication, deploys
// a reconcile script with systemd service+timer, and authorizes the key on the
// exit node.
func DeployEntryAutoReconcile(stdout io.Writer, project model.Project, rc model.RunContext) error {
	if !project.EntryAutoReconcile.Enabled {
		return nil
	}

	fmt.Fprintln(stdout, "\n--- 入口自动修复 ---")

	cfToken, cfSource, err := cloudflare.ResolveToken(project.Cloudflare)
	if err != nil {
		return fmt.Errorf("入口自动修复需要 Cloudflare token: %w", err)
	}
	fmt.Fprintf(stdout, "  Cloudflare token: %s\n", cfSource)
	fmt.Fprintf(stdout, "  刷新间隔: %ds\n", project.EntryAutoReconcile.Interval)

	tgToken := resolveTelegramToken(project)

	// --- Connect to entry node ---
	entryClient, err := sshclient.DialOrLocal(project.Nodes.US, rc.EntryIsLocal)
	if err != nil {
		return fmt.Errorf("入口自动修复: 无法连接入口节点: %w", err)
	}
	defer entryClient.Close()

	// --- Generate SSH key pair on entry node (idempotent) ---
	fmt.Fprintln(stdout, "  生成入口→出口 SSH 密钥对")
	if _, err := entryClient.RunShell("mkdir -p /etc/wgstack"); err != nil {
		return fmt.Errorf("入口自动修复: 创建目录失败: %w", err)
	}
	genKeyScript := `[ -f /etc/wgstack/exit_key ] || ssh-keygen -t ed25519 -f /etc/wgstack/exit_key -N '' -q -C 'wgstack-autoreconcile'
chmod 600 /etc/wgstack/exit_key`
	if _, err := entryClient.RunShell(genKeyScript); err != nil {
		return fmt.Errorf("入口自动修复: 生成 SSH 密钥失败: %w", err)
	}

	pubKeyOut, err := entryClient.RunShell("cat /etc/wgstack/exit_key.pub")
	if err != nil {
		return fmt.Errorf("入口自动修复: 读取公钥失败: %w", err)
	}
	pubKey := strings.TrimSpace(pubKeyOut)

	// --- Deploy public key to exit node ---
	fmt.Fprintln(stdout, "  授权公钥到出口节点")
	exitClient, err := sshclient.DialOrLocal(project.Nodes.HK, rc.ExitIsLocal)
	if err != nil {
		return fmt.Errorf("入口自动修复: 无法连接出口节点: %w", err)
	}
	defer exitClient.Close()

	authKeyScript := fmt.Sprintf(
		`mkdir -p ~/.ssh && chmod 700 ~/.ssh && touch ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys && grep -qF %s ~/.ssh/authorized_keys 2>/dev/null || echo %s >> ~/.ssh/authorized_keys`,
		quoteArg(pubKey), quoteArg(pubKey),
	)
	if _, err := exitClient.RunShell(authKeyScript); err != nil {
		return fmt.Errorf("入口自动修复: 部署公钥到出口节点失败: %w", err)
	}

	// --- Upload config, script, service, timer to entry node ---
	envContent := reconcileEnvConfig(project, cfToken, tgToken)

	uploads := []struct {
		path    string
		content string
		mode    string
		label   string
	}{
		{"/etc/wgstack/reconcile.env", envContent, "0600", "自动修复配置"},
		{"/usr/local/bin/wgstack-reconcile", reconcileScript, "0755", "自动修复脚本"},
		{"/etc/systemd/system/wgstack-reconcile.service", reconcileServiceUnit, "0644", "自动修复服务"},
		{"/etc/systemd/system/wgstack-reconcile.timer", reconcileTimerUnit(project.EntryAutoReconcile.Interval), "0644", "自动修复定时器"},
	}

	for _, u := range uploads {
		fmt.Fprintf(stdout, "  上传 %s -> %s\n", u.label, u.path)
		if err := entryClient.Upload(u.path, []byte(u.content), u.mode); err != nil {
			return fmt.Errorf("入口自动修复: 上传 %s 失败: %w", u.label, err)
		}
	}

	commands := []string{
		"systemctl daemon-reload",
		"systemctl enable --now wgstack-reconcile.timer",
		"systemctl start wgstack-reconcile.service",
	}
	for _, cmd := range commands {
		fmt.Fprintf(stdout, "  执行 %s\n", cmd)
		if err := runCmd(entryClient, cmd); err != nil {
			return fmt.Errorf("入口自动修复: %w", err)
		}
	}

	fmt.Fprintln(stdout, "  入口自动修复已启动")
	return nil
}

func resolveTelegramToken(project model.Project) string {
	if !project.Notifications.Enabled {
		return ""
	}
	if envName := project.Notifications.Telegram.BotTokenEnv; envName != "" {
		if t := strings.TrimSpace(os.Getenv(envName)); t != "" {
			return t
		}
	}
	return strings.TrimSpace(project.Notifications.Telegram.BotToken)
}
