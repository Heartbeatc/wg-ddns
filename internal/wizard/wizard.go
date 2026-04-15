package wizard

import (
	"fmt"
	"io"
	"net"
	"strings"

	"wg-ddns/internal/health"
	"wg-ddns/internal/keygen"
	"wg-ddns/internal/model"
	"wg-ddns/internal/sshclient"
)

func RunSetup(w io.Writer) (model.Project, model.RunContext, bool, error) {
	if !IsTerminal() {
		return model.Project{}, model.RunContext{}, false, fmt.Errorf("部署向导需要在终端中运行。\n如需非交互模式，请使用: wgstack apply --config <配置文件>")
	}

	p := NewPrompter(w)

	printWelcome(w)
	fmt.Fprint(w, "\n按回车开始...")
	p.WaitEnter()

	// --- Step 1: Run location ---
	fmt.Fprintln(w, "\n--- 第 1 步：运行位置 ---")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  本工具通过 SSH 远程配置目标节点。")
	fmt.Fprintln(w, "  如果你正在某台目标节点上运行，可以跳过该节点的 SSH 配置。")
	fmt.Fprintln(w)

	runLocIdx := p.Select("你当前在哪台机器上运行 wgstack？", []string{
		"本地电脑 / 管理机（需要两台节点的 SSH 信息）",
		"入口节点本机（只需要出口节点的 SSH 信息）",
		"出口节点本机（只需要入口节点的 SSH 信息）",
	})

	entryIsLocal := runLocIdx == 1
	exitIsLocal := runLocIdx == 2

	// --- Step 2: Entry node ---
	fmt.Fprintln(w, "\n--- 第 2 步：入口节点 ---")
	fmt.Fprintln(w)

	var entryHost, entryUser, entryAuthMethod, entryPassword, entryKeyPath string
	if entryIsLocal {
		fmt.Fprintln(w, "  当前机器即入口节点，无需 SSH 配置。")
		fmt.Fprintln(w, "  正在自动检测本机公网 IP...")
		entryHost = detectOrAskIP(w, p, "入口节点")
		entryUser = "root"
	} else {
		entryHost = p.LineWith("入口节点的 IP 地址", "", validateIP)
		entryUser = p.Line("SSH 用户名", "root")
		authIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})
		entryAuthMethod = "password"
		if authIdx == 0 {
			entryPassword = p.Password("SSH 密码")
		} else {
			entryAuthMethod = "private_key"
			entryKeyPath = p.Line("私钥文件路径", "~/.ssh/id_rsa")
		}
	}

	// --- Step 3: Exit node ---
	fmt.Fprintln(w, "\n--- 第 3 步：出口节点 ---")
	fmt.Fprintln(w)

	var exitHost, exitUser, exitAuthMethod, exitPassword, exitKeyPath string
	if exitIsLocal {
		fmt.Fprintln(w, "  当前机器即出口节点，无需 SSH 配置。")
		fmt.Fprintln(w, "  正在自动检测本机公网 IP...")
		exitHost = detectOrAskIP(w, p, "出口节点")
		exitUser = "root"
	} else {
		exitHost = p.LineWith("出口节点的 IP 地址", "", validateIP)
		exitUser = p.Line("SSH 用户名", "root")
		authIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})
		exitAuthMethod = "password"
		if authIdx == 0 {
			exitPassword = p.Password("SSH 密码")
		} else {
			exitAuthMethod = "private_key"
			exitKeyPath = p.Line("私钥文件路径", "~/.ssh/id_rsa")
		}
	}

	// --- Step 4: Cloudflare ---
	fmt.Fprintln(w, "\n--- 第 4 步：Cloudflare ---")
	fmt.Fprintln(w)

	cfZone := p.LineWith("Cloudflare Zone 域名（你的主域名，如 example.com）", "", validateDomain)
	cfToken := p.Password("Cloudflare API Token")

	// --- Step 5: Domains ---
	fmt.Fprintln(w, "\n--- 第 5 步：域名 ---")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  面板域名和入口域名通常都解析到入口节点，但用途不同：")
	fmt.Fprintln(w, "  • 面板域名：用于访问 3x-ui / x-panel 管理界面")
	fmt.Fprintln(w, "  • 入口域名：客户端连接代理时使用的地址")
	fmt.Fprintln(w)

	panelDomain := p.LineWith("面板域名（已部署的面板访问地址）", "panel."+cfZone, validateDomain)
	entryDomain := p.LineWith("代理入口域名（客户端连接代理时使用）", "entry."+cfZone, validateDomain)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  WireGuard 域名是出口节点连接入口节点时使用的地址。")
	fmt.Fprintln(w, "  大多数情况下可以直接复用入口域名，无需单独配置。")
	wgSeparate := p.Confirm("WireGuard 使用单独域名？", false)
	var wgDomain string
	if wgSeparate {
		wgDomain = p.LineWith("WireGuard 域名", "wg."+cfZone, validateDomain)
	} else {
		wgDomain = entryDomain
	}

	// --- Step 6: Panel + health ---
	fmt.Fprintln(w, "\n--- 第 6 步：面板与检查 ---")
	fmt.Fprintln(w)

	outboundTag := p.Line("面板出站标签", "exit-socks")
	routeUser := p.Line("专用线路用户标识", "exit-user@local")

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  出口地区代码用于健康检查时校验出口位置。")
	fmt.Fprintln(w, "  例如 HK（香港）、JP（日本）、SG（新加坡）、US（美国）。")
	fmt.Fprintln(w, "  留空则只测试连通性，不校验地区。")
	exitLocation := p.OptionalLine("出口地区代码（留空跳过）")

	if p.Err() != nil {
		return model.Project{}, model.RunContext{}, false, p.Err()
	}

	// Generate WireGuard keys
	fmt.Fprintln(w, "\n正在生成 WireGuard 密钥...")
	entryKey, err := keygen.Generate()
	if err != nil {
		return model.Project{}, model.RunContext{}, false, err
	}
	exitKey, err := keygen.Generate()
	if err != nil {
		return model.Project{}, model.RunContext{}, false, err
	}
	fmt.Fprintln(w, "密钥已生成。")

	project := model.Project{
		Project: "entry-exit-link",
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
				Host: entryHost,
				SSH: model.SSH{
					User:                  entryUser,
					Port:                  22,
					AuthMethod:            entryAuthMethod,
					Password:              entryPassword,
					PrivateKeyPath:        entryKeyPath,
					InsecureIgnoreHostKey: true,
				},
				WGAddress:    "10.66.66.1/24",
				WGPort:       51820,
				WGPrivateKey: entryKey.PrivateKey,
				WGPublicKey:  entryKey.PublicKey,
				WGConfigPath: "/etc/wireguard/wg0.conf",
				WGService:    "wg-quick@wg0",
				Deploy: model.NodeDeploy{
					AutoInstall: true,
				},
			},
			HK: model.Node{
				Role: "exit",
				Host: exitHost,
				SSH: model.SSH{
					User:                  exitUser,
					Port:                  22,
					AuthMethod:            exitAuthMethod,
					Password:              exitPassword,
					PrivateKeyPath:        exitKeyPath,
					InsecureIgnoreHostKey: true,
				},
				WGAddress:       "10.66.66.2/24",
				WGPrivateKey:    exitKey.PrivateKey,
				WGPublicKey:     exitKey.PublicKey,
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
			ExitLocation:     exitLocation,
		},
	}

	rc := model.RunContext{
		EntryIsLocal: entryIsLocal,
		ExitIsLocal:  exitIsLocal,
	}

	printSummary(w, project, rc)

	shouldDeploy := p.Confirm("\n确认开始部署？", true)
	if p.Err() != nil {
		return model.Project{}, model.RunContext{}, false, p.Err()
	}

	return project, rc, shouldDeploy, nil
}

// detectOrAskIP tries to auto-detect the local machine's public IP.
// On success, it shows the detected IP and asks for confirmation.
// On failure, it falls back to manual input.
func detectOrAskIP(w io.Writer, p *Prompter, label string) string {
	localClient := sshclient.NewLocal()
	detectedIP, detectErr := health.DetectPublicIPv4(localClient, "https://ifconfig.me/ip")
	localClient.Close()

	if detectErr == nil {
		fmt.Fprintf(w, "  检测到公网 IP: %s\n", detectedIP)
		fmt.Fprintln(w)
		if p.Confirm("使用此 IP？", true) {
			return detectedIP
		}
		return p.LineWith(label+"的公网 IP", "", validateIP)
	}

	fmt.Fprintf(w, "  自动检测失败: %v\n", detectErr)
	fmt.Fprintln(w)
	return p.LineWith(label+"的公网 IP（请手动输入）", "", validateIP)
}

func printWelcome(w io.Writer) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w, "  wgstack - 代理底层部署工具")
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "这个工具帮你搭建「入口节点 + 出口节点」的代理底层链路。")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "它会自动完成：")
	fmt.Fprintln(w, "  • 在两台节点间建立 WireGuard 加密隧道")
	fmt.Fprintln(w, "  • 在出口节点部署 sing-box 作为 SOCKS 代理")
	fmt.Fprintln(w, "  • 生成并下发所有配置文件")
	fmt.Fprintln(w, "  • 启动相关服务")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "你需要提前准备：")
	fmt.Fprintln(w, "  • 一台入口节点和一台出口节点（需要 root 权限）")
	fmt.Fprintln(w, "  • 目标节点的 SSH 登录信息（密码或私钥）")
	fmt.Fprintln(w, "  • 一个 Cloudflare API Token（需要 DNS 编辑权限）")
	fmt.Fprintln(w, "  • 面板域名和代理入口域名")
}

func printSummary(w io.Writer, project model.Project, rc model.RunContext) {
	fmt.Fprintln(w, "\n========================================")
	fmt.Fprintln(w, "  部署摘要")
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "入口节点:")
	fmt.Fprintf(w, "  IP:       %s\n", project.Nodes.US.Host)
	if rc.EntryIsLocal {
		fmt.Fprintln(w, "  部署方式: 本机")
	} else {
		fmt.Fprintf(w, "  SSH:      %s@%s (%s)\n", project.Nodes.US.SSH.User, project.Nodes.US.Host, authLabel(project.Nodes.US.SSH.AuthMethod))
	}
	fmt.Fprintf(w, "  WG 地址:  %s\n", project.Nodes.US.WGAddress)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "出口节点:")
	fmt.Fprintf(w, "  IP:       %s\n", project.Nodes.HK.Host)
	if rc.ExitIsLocal {
		fmt.Fprintln(w, "  部署方式: 本机")
	} else {
		fmt.Fprintf(w, "  SSH:      %s@%s (%s)\n", project.Nodes.HK.SSH.User, project.Nodes.HK.Host, authLabel(project.Nodes.HK.SSH.AuthMethod))
	}
	fmt.Fprintf(w, "  WG 地址:  %s\n", project.Nodes.HK.WGAddress)
	fmt.Fprintf(w, "  SOCKS:    %s\n", project.Nodes.HK.SocksListen)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Cloudflare:")
	fmt.Fprintf(w, "  Zone:       %s\n", project.Cloudflare.Zone)
	fmt.Fprintf(w, "  面板域名:   %s\n", project.Domains.Panel)
	fmt.Fprintf(w, "  入口域名:   %s\n", project.Domains.Entry)
	if project.Domains.WireGuard == project.Domains.Entry {
		fmt.Fprintf(w, "  WG 域名:    %s（复用入口域名）\n", project.Domains.WireGuard)
	} else {
		fmt.Fprintf(w, "  WG 域名:    %s\n", project.Domains.WireGuard)
	}
	fmt.Fprintln(w)
	if project.Checks.ExitLocation != "" {
		fmt.Fprintf(w, "健康检查:     出口地区 %s\n", project.Checks.ExitLocation)
	} else {
		fmt.Fprintln(w, "健康检查:     不校验出口地区")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "将要执行的操作：")
	fmt.Fprintln(w, "  1. 在两台节点上安装 WireGuard（如果未安装）")
	fmt.Fprintln(w, "  2. 在出口节点上安装 sing-box（如果未安装）")
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
