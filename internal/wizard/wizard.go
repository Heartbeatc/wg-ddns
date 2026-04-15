package wizard

import (
	"fmt"
	"io"
	"net"
	"strings"

	"wg-ddns/internal/keygen"
	"wg-ddns/internal/model"
)

func RunSetup(w io.Writer) (model.Project, bool, error) {
	if !IsTerminal() {
		return model.Project{}, false, fmt.Errorf("部署向导需要在终端中运行。\n如需非交互模式，请使用: wgstack apply --config <配置文件>")
	}

	p := NewPrompter(w)

	printWelcome(w)
	fmt.Fprint(w, "\n按回车开始...")
	p.WaitEnter()

	// --- Step 1: US node ---
	fmt.Fprintln(w, "\n--- 第 1 步：美国入口机 ---")
	fmt.Fprintln(w)

	usHost := p.LineWith("美国 VPS 的 IP 地址", "", validateIP)
	usUser := p.Line("SSH 用户名", "root")

	usAuthIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})
	usAuthMethod := "password"
	var usPassword, usKeyPath string
	if usAuthIdx == 0 {
		usPassword = p.Password("SSH 密码")
	} else {
		usAuthMethod = "private_key"
		usKeyPath = p.Line("私钥文件路径", "~/.ssh/id_rsa")
	}

	// --- Step 2: HK node ---
	fmt.Fprintln(w, "\n--- 第 2 步：香港出口机 ---")
	fmt.Fprintln(w)

	hkHost := p.LineWith("香港 VPS 的 IP 地址", "", validateIP)
	hkUser := p.Line("SSH 用户名", "root")

	hkAuthIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})
	hkAuthMethod := "password"
	var hkPassword, hkKeyPath string
	if hkAuthIdx == 0 {
		hkPassword = p.Password("SSH 密码")
	} else {
		hkAuthMethod = "private_key"
		hkKeyPath = p.Line("私钥文件路径", "~/.ssh/id_rsa")
	}

	// --- Step 3: Cloudflare ---
	fmt.Fprintln(w, "\n--- 第 3 步：Cloudflare ---")
	fmt.Fprintln(w)

	cfZone := p.LineWith("Cloudflare Zone 域名（你的主域名，如 example.com）", "", validateDomain)
	cfToken := p.Password("Cloudflare API Token")

	// --- Step 4: Domains ---
	fmt.Fprintln(w, "\n--- 第 4 步：域名 ---")
	fmt.Fprintln(w)

	entryDomain := p.LineWith("入口域名", "us."+cfZone, validateDomain)
	panelDomain := p.LineWith("面板域名", "panel."+cfZone, validateDomain)
	wgDomain := p.LineWith("WireGuard 域名", "wg."+cfZone, validateDomain)

	// --- Step 5: Panel ---
	fmt.Fprintln(w, "\n--- 第 5 步：面板配置 ---")
	fmt.Fprintln(w)

	outboundTag := p.Line("面板出站标签", "hk-socks")
	routeUser := p.Line("专用节点用户标识", "hk-user@local")

	if p.Err() != nil {
		return model.Project{}, false, p.Err()
	}

	// Generate WireGuard keys
	fmt.Fprintln(w, "\n正在生成 WireGuard 密钥...")
	usKey, err := keygen.Generate()
	if err != nil {
		return model.Project{}, false, err
	}
	hkKey, err := keygen.Generate()
	if err != nil {
		return model.Project{}, false, err
	}
	fmt.Fprintln(w, "密钥已生成。")

	project := model.Project{
		Project: "us-entry-hk-exit",
		Cloudflare: model.Cloudflare{
			Zone:       cfZone,
			Token:      cfToken,
			TokenEnv:   "CLOUDFLARE_API_TOKEN",
			RecordType: "A",
			TTL:        120,
			Proxied:    false,
		},
		Domains: model.Domains{
			Entry:     entryDomain,
			Panel:     panelDomain,
			WireGuard: wgDomain,
		},
		Nodes: model.Nodes{
			US: model.Node{
				Role: "entry",
				Host: usHost,
				SSH: model.SSH{
					User:                  usUser,
					Port:                  22,
					AuthMethod:            usAuthMethod,
					Password:              usPassword,
					PrivateKeyPath:        usKeyPath,
					InsecureIgnoreHostKey: true,
				},
				WGAddress:    "10.66.66.1/24",
				WGPort:       51820,
				WGPrivateKey: usKey.PrivateKey,
				WGPublicKey:  usKey.PublicKey,
				WGConfigPath: "/etc/wireguard/wg0.conf",
				WGService:    "wg-quick@wg0",
				Deploy: model.NodeDeploy{
					AutoInstall: true,
				},
			},
			HK: model.Node{
				Role: "exit",
				Host: hkHost,
				SSH: model.SSH{
					User:                  hkUser,
					Port:                  22,
					AuthMethod:            hkAuthMethod,
					Password:              hkPassword,
					PrivateKeyPath:        hkKeyPath,
					InsecureIgnoreHostKey: true,
				},
				WGAddress:       "10.66.66.2/24",
				WGPrivateKey:    hkKey.PrivateKey,
				WGPublicKey:     hkKey.PublicKey,
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
			OutboundTag: outboundTag,
			RouteUser:   routeUser,
		},
		Checks: model.HealthCheck{
			TestURL:          "https://ifconfig.me",
			ExitCheckURL:     "https://ifconfig.me/country_code",
			PublicIPCheckURL: "https://ifconfig.me/ip",
			ExitLocation:     "HK",
		},
	}

	printSummary(w, project)

	shouldDeploy := p.Confirm("\n确认开始部署？", true)
	if p.Err() != nil {
		return model.Project{}, false, p.Err()
	}

	return project, shouldDeploy, nil
}

func printWelcome(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w, "  wgstack - 代理底层部署工具")
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "这个工具帮你搭建「美国入口 + 香港出口」的代理底层链路。")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "它会自动完成：")
	fmt.Fprintln(w, "  • 在两台 VPS 间建立 WireGuard 加密隧道")
	fmt.Fprintln(w, "  • 在香港机上部署 sing-box 作为 SOCKS 代理")
	fmt.Fprintln(w, "  • 生成并下发所有配置文件")
	fmt.Fprintln(w, "  • 启动相关服务")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "你需要提前准备：")
	fmt.Fprintln(w, "  • 一台美国 VPS 和一台香港 VPS（需要 root 权限）")
	fmt.Fprintln(w, "  • 两台机器的 SSH 登录信息（密码或私钥）")
	fmt.Fprintln(w, "  • 一个 Cloudflare API Token（需要 DNS 编辑权限）")
	fmt.Fprintln(w, "  • 三个子域名（入口、面板、WireGuard 用）")
}

func printSummary(w io.Writer, project model.Project) {
	fmt.Fprintln(w, "\n========================================")
	fmt.Fprintln(w, "  部署摘要")
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "美国入口机:")
	fmt.Fprintf(w, "  IP:       %s\n", project.Nodes.US.Host)
	fmt.Fprintf(w, "  SSH:      %s@%s (%s)\n", project.Nodes.US.SSH.User, project.Nodes.US.Host, authLabel(project.Nodes.US.SSH.AuthMethod))
	fmt.Fprintf(w, "  WG 地址:  %s\n", project.Nodes.US.WGAddress)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "香港出口机:")
	fmt.Fprintf(w, "  IP:       %s\n", project.Nodes.HK.Host)
	fmt.Fprintf(w, "  SSH:      %s@%s (%s)\n", project.Nodes.HK.SSH.User, project.Nodes.HK.Host, authLabel(project.Nodes.HK.SSH.AuthMethod))
	fmt.Fprintf(w, "  WG 地址:  %s\n", project.Nodes.HK.WGAddress)
	fmt.Fprintf(w, "  SOCKS:    %s\n", project.Nodes.HK.SocksListen)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Cloudflare:")
	fmt.Fprintf(w, "  Zone:      %s\n", project.Cloudflare.Zone)
	fmt.Fprintf(w, "  入口域名:  %s\n", project.Domains.Entry)
	fmt.Fprintf(w, "  面板域名:  %s\n", project.Domains.Panel)
	fmt.Fprintf(w, "  WG 域名:   %s\n", project.Domains.WireGuard)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "将要执行的操作：")
	fmt.Fprintln(w, "  1. 在两台 VPS 上安装 WireGuard（如果未安装）")
	fmt.Fprintln(w, "  2. 在香港 VPS 上安装 sing-box（如果未安装）")
	fmt.Fprintln(w, "  3. 下发 WireGuard 和 sing-box 配置文件")
	fmt.Fprintln(w, "  4. 启动/重启服务")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "配置将保存到: %s\n", "wgstack.json")
}

func authLabel(method string) string {
	switch method {
	case "password":
		return "密码登录"
	case "private_key":
		return "私钥登录"
	default:
		return method
	}
}

func validateIP(s string) string {
	if net.ParseIP(s) == nil {
		return "IP 地址格式不对，请重新输入。"
	}
	return ""
}

func validateDomain(s string) string {
	if !strings.Contains(s, ".") || len(s) < 4 {
		return "域名格式不对（如 example.com），请重新输入。"
	}
	return ""
}
