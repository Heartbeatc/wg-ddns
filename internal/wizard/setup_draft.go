package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"wg-ddns/internal/cloudflare"
	"wg-ddns/internal/keygen"
	"wg-ddns/internal/model"
)

// SetupDraft holds in-memory configuration during the menu-based setup flow.
// Nothing is written to disk until the user chooses save or deploy.
type SetupDraft struct {
	Project model.Project
	RC      model.RunContext
	RCSet   bool

	ExitDDNSTouched  bool
	EntryAutoTouched bool
}

// SetupAction describes how RunSetupMenu completed.
type SetupAction int

const (
	ActionCancel SetupAction = iota
	ActionSaveOnly
	ActionDeploy
)

// SetupResult is returned when the menu-based setup exits.
type SetupResult struct {
	Action  SetupAction
	Project model.Project
	RC      model.RunContext
}

func runLocationIndex(rc model.RunContext) int {
	if rc.EntryIsLocal {
		return 1
	}
	if rc.ExitIsLocal {
		return 2
	}
	return 0
}

func (d *SetupDraft) statusRunLocation() string {
	if !d.RCSet {
		return "未配置"
	}
	return "已配置 · " + RunLocationOptions[runLocationIndex(d.RC)]
}

func (d *SetupDraft) statusEntry() string {
	if !d.RCSet {
		if nodeRemoteConfigured(d.Project.Nodes.US) || strings.TrimSpace(d.Project.Nodes.US.Host) != "" {
			return "已配置（待确认运行位置）"
		}
		return "未配置（需先设置运行位置）"
	}
	if d.RC.EntryIsLocal {
		if strings.TrimSpace(d.Project.Nodes.US.Host) == "" {
			return "未配置"
		}
		return "已配置 · 本机"
	}
	if !nodeRemoteConfigured(d.Project.Nodes.US) {
		return "未配置"
	}
	return "已配置 · 远程"
}

func (d *SetupDraft) statusExit() string {
	if !d.RCSet {
		if nodeRemoteConfigured(d.Project.Nodes.HK) || strings.TrimSpace(d.Project.Nodes.HK.Host) != "" {
			return "已配置（待确认运行位置）"
		}
		return "未配置（需先设置运行位置）"
	}
	if d.RC.ExitIsLocal {
		if strings.TrimSpace(d.Project.Nodes.HK.Host) == "" {
			return "未配置"
		}
		return "已配置 · 本机"
	}
	if !nodeRemoteConfigured(d.Project.Nodes.HK) {
		return "未配置"
	}
	return "已配置 · 远程"
}

func (d *SetupDraft) statusCloudflare() string {
	if strings.TrimSpace(d.Project.Cloudflare.Zone) == "" {
		return "未配置"
	}
	if _, _, err := cloudflare.ResolveToken(d.Project.Cloudflare); err != nil {
		return "未配置（缺少 Token）"
	}
	return "已配置 · Zone " + d.Project.Cloudflare.Zone
}

func (d *SetupDraft) statusDomains() string {
	dom := d.Project.Domains
	if strings.TrimSpace(dom.Entry) == "" || strings.TrimSpace(dom.Panel) == "" || strings.TrimSpace(dom.WireGuard) == "" {
		return "未配置"
	}
	return "已配置"
}

func (d *SetupDraft) statusExitDDNS() string {
	if !d.ExitDDNSTouched {
		return "未确认（建议进入此项确认）"
	}
	if d.Project.ExitDDNS.Enabled {
		return "已启用 · " + d.Project.ExitDDNS.Domain
	}
	return "已关闭"
}

func (d *SetupDraft) statusEntryAuto() string {
	if !d.EntryAutoTouched {
		return "未确认（建议进入此项确认）"
	}
	if d.Project.EntryAutoReconcile.Enabled {
		iv := d.Project.EntryAutoReconcile.Interval
		if iv < 60 {
			iv = 60
		}
		return "已启用 · 间隔 " + strconv.Itoa(iv) + "s"
	}
	return "已关闭"
}

func (d *SetupDraft) statusPanel() string {
	if strings.TrimSpace(d.Project.PanelGuide.OutboundTag) == "" || strings.TrimSpace(d.Project.PanelGuide.RouteUser) == "" {
		return "未配置"
	}
	return "已配置"
}

func nodeRemoteConfigured(n model.Node) bool {
	if strings.TrimSpace(n.Host) == "" || strings.TrimSpace(n.SSH.User) == "" {
		return false
	}
	switch n.SSH.AuthMethod {
	case "password":
		return strings.TrimSpace(n.SSH.Password) != "" || strings.TrimSpace(n.SSH.PasswordEnv) != ""
	case "private_key":
		return strings.TrimSpace(n.SSH.PrivateKeyPath) != ""
	default:
		return false
	}
}

func NewSetupDraft() (*SetupDraft, error) {
	p := model.Project{
		Project: "entry-exit-link",
		Cloudflare: model.Cloudflare{
			Zone:       "",
			Token:      "",
			TokenEnv:   "CLOUDFLARE_API_TOKEN",
			RecordType: "A",
			TTL:        120,
			Proxied:    false,
		},
		Domains: model.Domains{},
		Nodes: model.Nodes{
			US: model.Node{
				Role:         "entry",
				SSH:          model.SSH{Port: 22, InsecureIgnoreHostKey: true},
				WGAddress:    "10.66.66.1/24",
				WGPort:       51820,
				WGConfigPath: "/etc/wireguard/wg0.conf",
				WGService:    "wg-quick@wg0",
				Deploy:       model.NodeDeploy{AutoInstall: true},
			},
			HK: model.Node{
				Role:            "exit",
				SSH:             model.SSH{Port: 22, InsecureIgnoreHostKey: true},
				WGAddress:       "10.66.66.2/24",
				SocksListen:     "10.66.66.2:10808",
				Proxy:           "sing-box",
				WGConfigPath:    "/etc/wireguard/wg0.conf",
				WGService:       "wg-quick@wg0",
				ProxyConfigPath: "/etc/sing-box/config.json",
				ProxyService:    "sing-box",
				Deploy:          model.NodeDeploy{AutoInstall: true},
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
		},
		Notifications: model.Notifications{
			Enabled: false,
			Telegram: model.TelegramConfig{
				BotTokenEnv: "TELEGRAM_BOT_TOKEN",
			},
		},
		ExitDDNS: model.ExitDDNS{
			Enabled:  true,
			Interval: 60,
		},
		EntryAutoReconcile: model.AutoReconcile{
			Enabled:  true,
			Interval: 60,
		},
	}

	entryKey, err := keygen.Generate()
	if err != nil {
		return nil, fmt.Errorf("生成入口 WireGuard 密钥: %w", err)
	}
	exitKey, err := keygen.Generate()
	if err != nil {
		return nil, fmt.Errorf("生成出口 WireGuard 密钥: %w", err)
	}
	p.Nodes.US.WGPrivateKey = entryKey.PrivateKey
	p.Nodes.US.WGPublicKey = entryKey.PublicKey
	p.Nodes.HK.WGPrivateKey = exitKey.PrivateKey
	p.Nodes.HK.WGPublicKey = exitKey.PublicKey

	return &SetupDraft{Project: p}, nil
}

func NewSetupDraftFromProject(project model.Project) (*SetupDraft, error) {
	if strings.TrimSpace(project.Project) == "" {
		project.Project = "entry-exit-link"
	}
	if project.Cloudflare.TokenEnv == "" {
		project.Cloudflare.TokenEnv = "CLOUDFLARE_API_TOKEN"
	}
	if project.Cloudflare.RecordType == "" {
		project.Cloudflare.RecordType = "A"
	}
	if project.Cloudflare.TTL < 1 {
		project.Cloudflare.TTL = 120
	}
	if project.Nodes.US.SSH.Port == 0 {
		project.Nodes.US.SSH.Port = 22
	}
	if project.Nodes.HK.SSH.Port == 0 {
		project.Nodes.HK.SSH.Port = 22
	}
	project.Nodes.US.SSH.InsecureIgnoreHostKey = true
	project.Nodes.HK.SSH.InsecureIgnoreHostKey = true

	if strings.TrimSpace(project.Nodes.US.WGAddress) == "" {
		project.Nodes.US.WGAddress = "10.66.66.1/24"
	}
	if project.Nodes.US.WGPort <= 0 {
		project.Nodes.US.WGPort = 51820
	}
	if strings.TrimSpace(project.Nodes.US.WGConfigPath) == "" {
		project.Nodes.US.WGConfigPath = "/etc/wireguard/wg0.conf"
	}
	if strings.TrimSpace(project.Nodes.US.WGService) == "" {
		project.Nodes.US.WGService = "wg-quick@wg0"
	}
	if strings.TrimSpace(project.Nodes.HK.WGAddress) == "" {
		project.Nodes.HK.WGAddress = "10.66.66.2/24"
	}
	if strings.TrimSpace(project.Nodes.HK.SocksListen) == "" {
		project.Nodes.HK.SocksListen = "10.66.66.2:10808"
	}
	if strings.TrimSpace(project.Nodes.HK.Proxy) == "" {
		project.Nodes.HK.Proxy = "sing-box"
	}
	if strings.TrimSpace(project.Nodes.HK.WGConfigPath) == "" {
		project.Nodes.HK.WGConfigPath = "/etc/wireguard/wg0.conf"
	}
	if strings.TrimSpace(project.Nodes.HK.WGService) == "" {
		project.Nodes.HK.WGService = "wg-quick@wg0"
	}
	if strings.TrimSpace(project.Nodes.HK.ProxyConfigPath) == "" {
		project.Nodes.HK.ProxyConfigPath = "/etc/sing-box/config.json"
	}
	if strings.TrimSpace(project.Nodes.HK.ProxyService) == "" {
		project.Nodes.HK.ProxyService = "sing-box"
	}
	if strings.TrimSpace(project.PanelGuide.OutboundTag) == "" {
		project.PanelGuide.OutboundTag = "exit-socks"
	}
	if strings.TrimSpace(project.PanelGuide.RouteUser) == "" {
		project.PanelGuide.RouteUser = "exit-user@local"
	}
	if strings.TrimSpace(project.Checks.TestURL) == "" {
		project.Checks.TestURL = "https://ifconfig.me"
	}
	if strings.TrimSpace(project.Checks.ExitCheckURL) == "" {
		project.Checks.ExitCheckURL = "https://api.ipify.org"
	}
	if strings.TrimSpace(project.Checks.PublicIPCheckURL) == "" {
		project.Checks.PublicIPCheckURL = "https://api.ipify.org"
	}
	if project.Notifications.Telegram.BotTokenEnv == "" {
		project.Notifications.Telegram.BotTokenEnv = "TELEGRAM_BOT_TOKEN"
	}
	if project.ExitDDNS.Interval < 60 {
		project.ExitDDNS.Interval = 60
	}
	if project.EntryAutoReconcile.Interval < 60 {
		project.EntryAutoReconcile.Interval = 60
	}

	if strings.TrimSpace(project.Nodes.US.WGPrivateKey) == "" || strings.TrimSpace(project.Nodes.US.WGPublicKey) == "" {
		entryKey, err := keygen.Generate()
		if err != nil {
			return nil, fmt.Errorf("生成入口 WireGuard 密钥: %w", err)
		}
		project.Nodes.US.WGPrivateKey = entryKey.PrivateKey
		project.Nodes.US.WGPublicKey = entryKey.PublicKey
	}
	if strings.TrimSpace(project.Nodes.HK.WGPrivateKey) == "" || strings.TrimSpace(project.Nodes.HK.WGPublicKey) == "" {
		exitKey, err := keygen.Generate()
		if err != nil {
			return nil, fmt.Errorf("生成出口 WireGuard 密钥: %w", err)
		}
		project.Nodes.HK.WGPrivateKey = exitKey.PrivateKey
		project.Nodes.HK.WGPublicKey = exitKey.PublicKey
	}

	return &SetupDraft{
		Project:          project,
		ExitDDNSTouched:  project.ExitDDNS.Enabled || strings.TrimSpace(project.ExitDDNS.Domain) != "",
		EntryAutoTouched: project.EntryAutoReconcile.Enabled || project.EntryAutoReconcile.Interval > 0,
	}, nil
}
