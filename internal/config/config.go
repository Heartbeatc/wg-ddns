package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wg-ddns/internal/model"
)

const (
	DefaultPath = "wgstack.json"
	DraftPath   = "wgstack.draft.json"
)

func DefaultProject() model.Project {
	return model.Project{
		Project: "entry-exit-link",
		Cloudflare: model.Cloudflare{
			Zone:       "example.com",
			Token:      "",
			TokenEnv:   "CLOUDFLARE_API_TOKEN",
			RecordType: "A",
			TTL:        120,
			Proxied:    false,
		},
		Domains: model.Domains{
			Entry:     "entry.example.com",
			Panel:     "entry.example.com",
			WireGuard: "entry.example.com",
		},
		Nodes: model.Nodes{
			US: model.Node{
				Role: "entry",
				Host: "1.2.3.4",
				SSH: model.SSH{
					User:                  "root",
					Port:                  22,
					AuthMethod:            "private_key",
					PrivateKeyPath:        "~/.ssh/id_rsa.pem",
					InsecureIgnoreHostKey: true,
				},
				WGAddress:    "10.66.66.1/24",
				WGPort:       51820,
				WGConfigPath: "/etc/wireguard/wg0.conf",
				WGService:    "wg-quick@wg0",
				Deploy: model.NodeDeploy{
					AutoInstall: true,
				},
			},
			HK: model.Node{
				Role:    "exit",
				Host:    "5.6.7.8",
				SSHHost: "ssh.exit.example.com",
				SSH: model.SSH{
					User:                  "root",
					Port:                  22,
					AuthMethod:            "private_key",
					PrivateKeyPath:        "~/.ssh/id_rsa.pem",
					InsecureIgnoreHostKey: true,
				},
				WGAddress:       "10.66.66.2/24",
				SocksListen:     "10.66.66.2:10808",
				Proxy:           "sing-box",
				WGConfigPath:    "/etc/wireguard/wg0.conf",
				WGService:       "wg-quick@wg0",
				ProxyConfigPath: "/etc/sing-box/config.json",
				ProxyService:    "sing-box",
				Deploy: model.NodeDeploy{
					AutoInstall: true,
				},
			},
		},
		PanelGuide: model.PanelGuide{
			OutboundTag: "exit-socks",
			RouteUser:   "exit-user@local",
		},
		Checks: model.HealthCheck{
			TestURL:          "https://ifconfig.me",
			ExitCheckURL:     "https://api.ipify.org",
			PublicIPCheckURL: "https://api.ipify.org",
			ExitLocation:     "",
		},
		Notifications: model.Notifications{
			Enabled: false,
			Telegram: model.TelegramConfig{
				BotTokenEnv: "TELEGRAM_BOT_TOKEN",
			},
		},
		ExitDDNS: model.ExitDDNS{
			Enabled:  true,
			Domain:   "ssh.exit.example.com",
			Interval: 60,
		},
		EntryAutoReconcile: model.AutoReconcile{
			Enabled:  true,
			Interval: 60,
		},
	}
}

func Load(path string) (model.Project, error) {
	if path == "" {
		path = DefaultPath
	}

	project, err := loadRaw(path)
	if err != nil {
		return model.Project{}, err
	}

	if err := Validate(project); err != nil {
		return model.Project{}, err
	}

	return project, nil
}

func Save(path string, project model.Project) error {
	if path == "" {
		path = DefaultPath
	}

	if err := Validate(project); err != nil {
		return err
	}

	return saveRaw(path, project)
}

// LoadDraft loads a partially-complete wizard draft without enforcing full
// config validation.
func LoadDraft(path string) (model.Project, error) {
	if path == "" {
		path = DraftPath
	}
	return loadRaw(path)
}

// SaveDraft writes a partially-complete wizard draft to disk without requiring
// full config validation.
func SaveDraft(path string, project model.Project) error {
	if path == "" {
		path = DraftPath
	}
	return saveRaw(path, project)
}

func Validate(project model.Project) error {
	var missing []string
	require := func(label, val string) {
		if strings.TrimSpace(val) == "" {
			missing = append(missing, label)
		}
	}

	require("project", project.Project)

	// Cloudflare
	require("cloudflare.zone", project.Cloudflare.Zone)
	if strings.TrimSpace(project.Cloudflare.Token) == "" && strings.TrimSpace(project.Cloudflare.TokenEnv) == "" {
		missing = append(missing, "cloudflare.token or cloudflare.token_env")
	}
	require("cloudflare.record_type", project.Cloudflare.RecordType)

	// Domains
	require("domains.entry", project.Domains.Entry)
	require("domains.panel", project.Domains.Panel)
	require("domains.wireguard", project.Domains.WireGuard)

	// Entry node (US)
	require("nodes.us.host", project.Nodes.US.Host)
	require("nodes.us.ssh.user", project.Nodes.US.SSH.User)
	require("nodes.us.wg_address", project.Nodes.US.WGAddress)
	require("nodes.us.wg_config_path", project.Nodes.US.WGConfigPath)
	require("nodes.us.wg_service", project.Nodes.US.WGService)
	if project.Nodes.US.WGPort <= 0 {
		missing = append(missing, "nodes.us.wg_port")
	}

	// Exit node (HK)
	require("nodes.hk.host", project.Nodes.HK.Host)
	require("nodes.hk.ssh.user", project.Nodes.HK.SSH.User)
	require("nodes.hk.wg_address", project.Nodes.HK.WGAddress)
	require("nodes.hk.wg_config_path", project.Nodes.HK.WGConfigPath)
	require("nodes.hk.wg_service", project.Nodes.HK.WGService)
	require("nodes.hk.socks_listen", project.Nodes.HK.SocksListen)
	require("nodes.hk.proxy_config_path", project.Nodes.HK.ProxyConfigPath)
	require("nodes.hk.proxy_service", project.Nodes.HK.ProxyService)

	// Panel guide & health checks
	require("panel_guide.outbound_tag", project.PanelGuide.OutboundTag)
	require("panel_guide.route_user", project.PanelGuide.RouteUser)
	require("healthcheck.exit_check_url (必须返回纯文本公网 IP)", project.Checks.ExitCheckURL)
	require("healthcheck.public_ip_check_url", project.Checks.PublicIPCheckURL)

	if project.ExitDDNS.Enabled {
		require("exit_ddns.domain", project.ExitDDNS.Domain)
		if project.ExitDDNS.Interval < 60 {
			missing = append(missing, "exit_ddns.interval_seconds (最小 60)")
		}
		if strings.TrimSpace(project.ExitDDNS.Domain) != "" {
			sshHost := strings.TrimSpace(project.Nodes.HK.SSHHost)
			if sshHost == "" {
				missing = append(missing, "nodes.hk.ssh_host（启用出口管理 DDNS 时，ssh_host 必须设置为 exit_ddns.domain）")
			} else if sshHost != strings.TrimSpace(project.ExitDDNS.Domain) {
				missing = append(missing, fmt.Sprintf(
					"nodes.hk.ssh_host 与 exit_ddns.domain 不一致（ssh_host=%q, domain=%q）——启用出口管理 DDNS 时两者必须相同",
					sshHost, project.ExitDDNS.Domain))
			}
		}
	}

	if project.EntryAutoReconcile.Enabled {
		if project.EntryAutoReconcile.Interval < 60 {
			missing = append(missing, "entry_autoreconcile.interval_seconds (最小 60)")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	if project.Cloudflare.TTL < 1 {
		return fmt.Errorf("cloudflare.ttl must be greater than 0")
	}

	return nil
}

func loadRaw(path string) (model.Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Project{}, err
	}

	var project model.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return model.Project{}, fmt.Errorf("decode config: %w", err)
	}
	return project, nil
}

func saveRaw(path string, project model.Project) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

// ValidateDeploy checks that the project has all fields needed for actual
// deployment. rc carries runtime-only state about which nodes are local;
// SSH authentication is only required for non-local nodes.
func ValidateDeploy(project model.Project, rc model.RunContext) error {
	var problems []string

	if !rc.EntryIsLocal && project.Nodes.US.SSH.Port <= 0 {
		problems = append(problems, "入口节点 SSH 端口 (nodes.us.ssh.port)")
	}
	if !rc.ExitIsLocal && project.Nodes.HK.SSH.Port <= 0 {
		problems = append(problems, "出口节点 SSH 端口 (nodes.hk.ssh.port)")
	}

	checkAuth := func(prefix string, ssh model.SSH) {
		switch ssh.AuthMethod {
		case "password":
			if strings.TrimSpace(ssh.Password) == "" && strings.TrimSpace(ssh.PasswordEnv) == "" {
				problems = append(problems, prefix+".password or "+prefix+".password_env")
			}
		case "private_key":
			if strings.TrimSpace(ssh.PrivateKeyPath) == "" {
				problems = append(problems, prefix+".private_key_path")
			}
		default:
			problems = append(problems, prefix+".auth_method 未配置")
		}
	}

	if !rc.EntryIsLocal {
		checkAuth("nodes.us.ssh", project.Nodes.US.SSH)
	}
	if !rc.ExitIsLocal {
		checkAuth("nodes.hk.ssh", project.Nodes.HK.SSH)
	}

	checkWGKey := func(label, key string) {
		if strings.TrimSpace(key) == "" {
			problems = append(problems, label+" 为空")
			return
		}
		data, err := base64.StdEncoding.DecodeString(key)
		if err != nil {
			problems = append(problems, label+" 不是有效的 base64")
			return
		}
		if len(data) != 32 {
			problems = append(problems, fmt.Sprintf("%s 长度不正确（需要 32 字节，实际 %d）", label, len(data)))
		}
	}

	checkWGKey("入口 WG 私钥 (nodes.us.wg_private_key)", project.Nodes.US.WGPrivateKey)
	checkWGKey("入口 WG 公钥 (nodes.us.wg_public_key)", project.Nodes.US.WGPublicKey)
	checkWGKey("出口 WG 私钥 (nodes.hk.wg_private_key)", project.Nodes.HK.WGPrivateKey)
	checkWGKey("出口 WG 公钥 (nodes.hk.wg_public_key)", project.Nodes.HK.WGPublicKey)

	if len(problems) > 0 {
		return fmt.Errorf("部署配置不完整: %s", strings.Join(problems, ", "))
	}

	return nil
}
