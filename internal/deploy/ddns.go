package deploy

import (
	"fmt"
	"io"

	"wg-ddns/internal/cloudflare"
	"wg-ddns/internal/model"
)

const ddnsScript = `#!/bin/sh
set -eu

CONFIG="/etc/wgstack/ddns.env"
STATE_DIR="/var/lib/wgstack"
STATE_FILE="$STATE_DIR/ddns_last_ip"
ZONE_CACHE="$STATE_DIR/ddns_zone_id"

if [ ! -f "$CONFIG" ]; then
  echo "ERROR: config not found: $CONFIG"
  exit 1
fi

. "$CONFIG"

CURRENT_IP=$(curl -4fsS --max-time 10 "$CHECK_URL" 2>/dev/null | tr -d '[:space:]')
if [ -z "$CURRENT_IP" ]; then
  echo "ERROR: failed to detect public IP from $CHECK_URL"
  exit 1
fi

case "$CURRENT_IP" in
  [0-9]*.[0-9]*.[0-9]*.[0-9]*) ;;
  *) echo "ERROR: not a valid IPv4: $CURRENT_IP"; exit 1 ;;
esac

mkdir -p "$STATE_DIR"
LAST_IP=""
[ -f "$STATE_FILE" ] && LAST_IP=$(cat "$STATE_FILE")
if [ "$CURRENT_IP" = "$LAST_IP" ]; then
  exit 0
fi

echo "IP changed: ${LAST_IP:-<none>} -> $CURRENT_IP"

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
    echo "$ZONE_RESP"
    exit 1
  fi
  echo "$ZONE_ID" > "$ZONE_CACHE"
fi

REC_RESP=$(curl -fsS --max-time 10 \
  -H "Authorization: Bearer $CF_API_TOKEN" \
  -H "Content-Type: application/json" \
  "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records?type=A&name=$DDNS_DOMAIN" 2>&1)
REC_ID=$(printf '%s' "$REC_RESP" | grep -o '"id":"[^"]*"' | head -1 | sed 's/"id":"//;s/"//')

API="https://api.cloudflare.com/client/v4/zones/$ZONE_ID/dns_records"
DATA="{\"type\":\"A\",\"name\":\"$DDNS_DOMAIN\",\"content\":\"$CURRENT_IP\",\"ttl\":$RECORD_TTL,\"proxied\":false}"

if [ -z "$REC_ID" ]; then
  echo "Creating DNS record $DDNS_DOMAIN -> $CURRENT_IP"
  RESULT=$(curl -fsS --max-time 10 -X POST \
    -H "Authorization: Bearer $CF_API_TOKEN" \
    -H "Content-Type: application/json" \
    "$API" -d "$DATA" 2>&1)
else
  echo "Updating DNS record $DDNS_DOMAIN -> $CURRENT_IP"
  RESULT=$(curl -fsS --max-time 10 -X PATCH \
    -H "Authorization: Bearer $CF_API_TOKEN" \
    -H "Content-Type: application/json" \
    "$API/$REC_ID" -d "$DATA" 2>&1)
fi

case "$RESULT" in
  *'"success":true'*)
    echo "$CURRENT_IP" > "$STATE_FILE"
    echo "OK: $DDNS_DOMAIN -> $CURRENT_IP"
    ;;
  *)
    echo "ERROR: Cloudflare API failed"
    echo "$RESULT"
    exit 1
    ;;
esac
`

const ddnsServiceUnit = `[Unit]
Description=wgstack exit node DDNS updater
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/wgstack-ddns
`

func ddnsTimerUnit(intervalSec int) string {
	return fmt.Sprintf(`[Unit]
Description=wgstack exit node DDNS timer

[Timer]
OnBootSec=30
OnUnitActiveSec=%ds

[Install]
WantedBy=timers.target
`, intervalSec)
}

func ddnsEnvConfig(project model.Project, token string) string {
	return fmt.Sprintf("CF_API_TOKEN=%s\nCF_ZONE=%s\nDDNS_DOMAIN=%s\nCHECK_URL=https://api.ipify.org\nRECORD_TTL=%d\n",
		token, project.Cloudflare.Zone, project.ExitDDNS.Domain, project.Cloudflare.TTL)
}

// DeployExitDDNS installs the DDNS updater on the exit node.
// It uploads a config file, update script, and systemd service+timer,
// then enables and starts the timer.
func DeployExitDDNS(stdout io.Writer, project model.Project, rc model.RunContext) error {
	if !project.ExitDDNS.Enabled {
		return nil
	}

	fmt.Fprintln(stdout, "\n--- 出口管理 DDNS ---")

	token, source, err := cloudflare.ResolveToken(project.Cloudflare)
	if err != nil {
		return fmt.Errorf("出口 DDNS 需要 Cloudflare token: %w", err)
	}
	fmt.Fprintf(stdout, "  Cloudflare token: %s\n", source)
	fmt.Fprintf(stdout, "  管理域名: %s\n", project.ExitDDNS.Domain)
	fmt.Fprintf(stdout, "  刷新间隔: %ds\n", project.ExitDDNS.Interval)

	client, _, err := dialNodeForDeploy(stdout, "出口节点", project.Nodes.HK, rc.ExitIsLocal)
	if err != nil {
		return fmt.Errorf("出口 DDNS: 无法连接出口节点: %w", err)
	}
	defer client.Close()

	uploads := []struct {
		path    string
		content string
		mode    string
		label   string
	}{
		{"/etc/wgstack/ddns.env", ddnsEnvConfig(project, token), "0600", "DDNS 配置"},
		{"/usr/local/bin/wgstack-ddns", ddnsScript, "0755", "DDNS 脚本"},
		{"/etc/systemd/system/wgstack-ddns.service", ddnsServiceUnit, "0644", "DDNS 服务"},
		{"/etc/systemd/system/wgstack-ddns.timer", ddnsTimerUnit(project.ExitDDNS.Interval), "0644", "DDNS 定时器"},
	}

	for _, u := range uploads {
		fmt.Fprintf(stdout, "  上传 %s -> %s\n", u.label, u.path)
		if err := client.Upload(u.path, []byte(u.content), u.mode); err != nil {
			return fmt.Errorf("出口 DDNS: 上传 %s 失败: %w", u.label, err)
		}
	}

	commands := []string{
		"systemctl daemon-reload",
		"systemctl enable --now wgstack-ddns.timer",
		"systemctl start wgstack-ddns.service",
	}
	for _, cmd := range commands {
		fmt.Fprintf(stdout, "  执行 %s\n", cmd)
		if err := runCmd(client, cmd); err != nil {
			return fmt.Errorf("出口 DDNS: %w", err)
		}
	}

	fmt.Fprintln(stdout, "  出口管理 DDNS 已启动")
	return nil
}
