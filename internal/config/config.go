package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wg-ddns/internal/model"
)

const DefaultPath = "wgstack.json"

func DefaultProject() model.Project {
	return model.Project{
		Project: "us-entry-hk-exit",
		Cloudflare: model.Cloudflare{
			Zone:       "example.com",
			TokenEnv:   "CLOUDFLARE_API_TOKEN",
			RecordType: "A",
			TTL:        120,
			Proxied:    false,
		},
		Domains: model.Domains{
			Entry:     "us.example.com",
			Panel:     "panel.example.com",
			WireGuard: "wg.example.com",
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
				Role: "exit",
				Host: "5.6.7.8",
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
			OutboundTag: "hk-socks",
			RouteUser:   "hk-test@local",
		},
		Checks: model.HealthCheck{
			TestURL:          "https://ifconfig.me",
			ExitCheckURL:     "https://ifconfig.me/country_code",
			PublicIPCheckURL: "https://ifconfig.me/ip",
			ExitLocation:     "HK",
		},
	}
}

func Load(path string) (model.Project, error) {
	if path == "" {
		path = DefaultPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return model.Project{}, err
	}

	var project model.Project
	if err := json.Unmarshal(data, &project); err != nil {
		return model.Project{}, fmt.Errorf("decode config: %w", err)
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func Validate(project model.Project) error {
	var missing []string

	if strings.TrimSpace(project.Project) == "" {
		missing = append(missing, "project")
	}
	if strings.TrimSpace(project.Cloudflare.Zone) == "" {
		missing = append(missing, "cloudflare.zone")
	}
	if strings.TrimSpace(project.Cloudflare.TokenEnv) == "" {
		missing = append(missing, "cloudflare.token_env")
	}
	if strings.TrimSpace(project.Cloudflare.RecordType) == "" {
		missing = append(missing, "cloudflare.record_type")
	}
	if strings.TrimSpace(project.Domains.Entry) == "" {
		missing = append(missing, "domains.entry")
	}
	if strings.TrimSpace(project.Domains.Panel) == "" {
		missing = append(missing, "domains.panel")
	}
	if strings.TrimSpace(project.Domains.WireGuard) == "" {
		missing = append(missing, "domains.wireguard")
	}
	if strings.TrimSpace(project.Nodes.US.Host) == "" {
		missing = append(missing, "nodes.us.host")
	}
	if strings.TrimSpace(project.Nodes.HK.Host) == "" {
		missing = append(missing, "nodes.hk.host")
	}
	if strings.TrimSpace(project.Nodes.US.SSH.User) == "" {
		missing = append(missing, "nodes.us.ssh.user")
	}
	if strings.TrimSpace(project.Nodes.HK.SSH.User) == "" {
		missing = append(missing, "nodes.hk.ssh.user")
	}
	if strings.TrimSpace(project.Nodes.US.WGAddress) == "" {
		missing = append(missing, "nodes.us.wg_address")
	}
	if strings.TrimSpace(project.Nodes.HK.WGAddress) == "" {
		missing = append(missing, "nodes.hk.wg_address")
	}
	if project.Nodes.US.WGPort <= 0 {
		missing = append(missing, "nodes.us.wg_port")
	}
	if strings.TrimSpace(project.Nodes.HK.SocksListen) == "" {
		missing = append(missing, "nodes.hk.socks_listen")
	}
	if strings.TrimSpace(project.Nodes.US.WGConfigPath) == "" {
		missing = append(missing, "nodes.us.wg_config_path")
	}
	if strings.TrimSpace(project.Nodes.HK.WGConfigPath) == "" {
		missing = append(missing, "nodes.hk.wg_config_path")
	}
	if strings.TrimSpace(project.Nodes.US.WGService) == "" {
		missing = append(missing, "nodes.us.wg_service")
	}
	if strings.TrimSpace(project.Nodes.HK.WGService) == "" {
		missing = append(missing, "nodes.hk.wg_service")
	}
	if strings.TrimSpace(project.Nodes.HK.ProxyConfigPath) == "" {
		missing = append(missing, "nodes.hk.proxy_config_path")
	}
	if strings.TrimSpace(project.Nodes.HK.ProxyService) == "" {
		missing = append(missing, "nodes.hk.proxy_service")
	}
	if strings.TrimSpace(project.PanelGuide.OutboundTag) == "" {
		missing = append(missing, "panel_guide.outbound_tag")
	}
	if strings.TrimSpace(project.PanelGuide.RouteUser) == "" {
		missing = append(missing, "panel_guide.route_user")
	}
	if strings.TrimSpace(project.Checks.ExitCheckURL) == "" {
		missing = append(missing, "healthcheck.exit_check_url")
	}
	if strings.TrimSpace(project.Checks.PublicIPCheckURL) == "" {
		missing = append(missing, "healthcheck.public_ip_check_url")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required fields: %s", strings.Join(missing, ", "))
	}
	if project.Cloudflare.TTL < 1 {
		return fmt.Errorf("cloudflare.ttl must be greater than 0")
	}

	return nil
}

func ValidateDeploy(project model.Project) error {
	var missing []string

	if project.Nodes.US.SSH.Port <= 0 {
		missing = append(missing, "nodes.us.ssh.port")
	}
	if project.Nodes.HK.SSH.Port <= 0 {
		missing = append(missing, "nodes.hk.ssh.port")
	}

	checkAuth := func(prefix string, ssh model.SSH) {
		switch ssh.AuthMethod {
		case "password":
			if strings.TrimSpace(ssh.Password) == "" && strings.TrimSpace(ssh.PasswordEnv) == "" {
				missing = append(missing, prefix+".password or "+prefix+".password_env")
			}
		case "private_key":
			if strings.TrimSpace(ssh.PrivateKeyPath) == "" {
				missing = append(missing, prefix+".private_key_path")
			}
		default:
			missing = append(missing, prefix+".auth_method")
		}
	}

	checkAuth("nodes.us.ssh", project.Nodes.US.SSH)
	checkAuth("nodes.hk.ssh", project.Nodes.HK.SSH)

	if len(missing) > 0 {
		return fmt.Errorf("deploy config incomplete: %s", strings.Join(missing, ", "))
	}

	return nil
}
