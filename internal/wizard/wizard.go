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

// RunLocationOptions is the shared list of run-location choices used by both
// the setup wizard and the main menu.
var RunLocationOptions = []string{
	"本地电脑 / 管理机（需要两台节点的 SSH 信息）",
	"入口节点本机（只需要出口节点的 SSH 信息）",
	"出口节点本机（只需要入口节点的 SSH 信息）",
}

// AskRunContext prompts the user to identify which machine they are on
// and returns the corresponding RunContext.
func AskRunContext(p *Prompter) model.RunContext {
	idx := p.Select("你当前在哪台机器上运行？", RunLocationOptions)
	return model.RunContext{
		EntryIsLocal: idx == 1,
		ExitIsLocal:  idx == 2,
	}
}

// nodeInput holds user-supplied node connection info collected by the wizard.
type nodeInput struct {
	host       string // public IP of the node
	sshHost    string // SSH connection address (domain or IP); empty means same as host
	user       string
	authMethod string
	password   string
	keyPath    string
}

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

	runLocIdx := p.Select("你当前在哪台机器上运行 wgstack？", RunLocationOptions)

	entryIsLocal := runLocIdx == 1
	exitIsLocal := runLocIdx == 2

	// --- Step 2: Entry node ---
	fmt.Fprintln(w, "\n--- 第 2 步：入口节点 ---")
	fmt.Fprintln(w)
	entry := collectNodeInfo(w, p, "入口节点", entryIsLocal)

	// --- Step 3: Exit node ---
	fmt.Fprintln(w, "\n--- 第 3 步：出口节点 ---")
	fmt.Fprintln(w)
	exit := collectNodeInfo(w, p, "出口节点", exitIsLocal)

	// --- Step 4: Cloudflare ---
	fmt.Fprintln(w, "\n--- 第 4 步：Cloudflare ---")
	fmt.Fprintln(w)

	cfZone := p.LineWith("Cloudflare Zone 域名（你的主域名，如 example.com）", "", validateDomain)
	cfToken := p.Password("Cloudflare API Token")

	// --- Step 5: Domains ---
	fmt.Fprintln(w, "\n--- 第 5 步：域名 ---")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  大多数情况下，面板访问和客户端连接代理使用同一个域名，")
	fmt.Fprintln(w, "  只是通过不同端口分流（如 :面板端口 访问面板，:443 连接代理）。")
	fmt.Fprintln(w)

	entryDomain := p.LineWith("你的对外域名（面板和代理共用）", cfZone, validateDomain)

	panelSeparate := p.Confirm("面板访问域名是否与此不同？", false)
	var panelDomain string
	if panelSeparate {
		panelDomain = p.LineWith("面板域名", "panel."+cfZone, validateDomain)
	} else {
		panelDomain = entryDomain
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  WireGuard Endpoint 是出口节点连接入口节点时使用的域名。")
	fmt.Fprintln(w, "  默认复用你的对外域名，不需要额外准备。")
	fmt.Fprintln(w, "  如果单独配置，也应填写域名（本工具会将其纳入 DNS 同步）。")
	wgSeparate := p.Confirm("WireGuard Endpoint 使用单独域名？", false)
	var wgDomain string
	if wgSeparate {
		wgDomain = p.LineWith("WireGuard Endpoint 域名", "wg."+cfZone, validateDomain)
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
				Role:    "entry",
				Host:    entry.host,
				SSHHost: entry.sshHost,
				SSH: model.SSH{
					User:                  entry.user,
					Port:                  22,
					AuthMethod:            entry.authMethod,
					Password:              entry.password,
					PrivateKeyPath:        entry.keyPath,
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
				Role:    "exit",
				Host:    exit.host,
				SSHHost: exit.sshHost,
				SSH: model.SSH{
					User:                  exit.user,
					Port:                  22,
					AuthMethod:            exit.authMethod,
					Password:              exit.password,
					PrivateKeyPath:        exit.keyPath,
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
			ExitCheckURL:     "https://api.ipify.org",
			PublicIPCheckURL: "https://api.ipify.org",
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

// collectNodeInfo gathers host/SSH info for one node.
// When isLocal is true, SSH fields are skipped and the public IP is auto-detected.
func collectNodeInfo(w io.Writer, p *Prompter, label string, isLocal bool) nodeInput {
	if isLocal {
		fmt.Fprintf(w, "  当前机器即%s，无需 SSH 配置。\n", label)
		fmt.Fprintln(w, "  正在自动检测本机公网 IP...")
		host := detectOrAskIP(w, p, label)
		return nodeInput{host: host, user: "root"}
	}

	host := p.LineWith(label+"的公网 IP 地址", "", validateIP)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  SSH 连接地址用于本工具远程管理该节点。")
	fmt.Fprintln(w, "  如果节点 IP 可能变化（如入口节点换 IP，或出口节点使用家宽动态 IP），")
	fmt.Fprintln(w, "  推荐填写一个稳定的域名，这样即使 IP 漂移，工具仍能连上节点。")
	fmt.Fprintln(w, "  如果 IP 不会变化，直接回车使用公网 IP 即可。")
	sshHost := p.OptionalLine("SSH 连接地址（域名或 IP，留空则使用公网 IP）")

	user := p.Line("SSH 用户名", "root")
	authIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})

	ni := nodeInput{host: host, sshHost: sshHost, user: user, authMethod: "password"}
	if authIdx == 0 {
		ni.password = p.Password("SSH 密码")
	} else {
		ni.authMethod = "private_key"
		ni.keyPath = p.Line("私钥文件路径", "~/.ssh/id_rsa")
	}
	return ni
}

// detectOrAskIP tries to auto-detect the local machine's public IP.
// On success it asks for confirmation; on failure it falls back to manual input.
func detectOrAskIP(w io.Writer, p *Prompter, label string) string {
	localClient := sshclient.NewLocal()
	detectedIP, detectErr := health.DetectPublicIPv4(localClient, "https://api.ipify.org")
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
	fmt.Fprintln(w, "  • 一个对外域名（面板和代理入口通常共用）")
}

func printSummary(w io.Writer, project model.Project, rc model.RunContext) {
	fmt.Fprintln(w, "\n========================================")
	fmt.Fprintln(w, "  部署摘要")
	fmt.Fprintln(w, "========================================")
	fmt.Fprintln(w)

	printNodeSummary(w, "入口节点", project.Nodes.US, rc.EntryIsLocal)
	fmt.Fprintln(w)
	printNodeSummary(w, "出口节点", project.Nodes.HK, rc.ExitIsLocal)
	fmt.Fprintf(w, "  SOCKS:    %s\n", project.Nodes.HK.SocksListen)
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Cloudflare:")
	fmt.Fprintf(w, "  Zone:          %s\n", project.Cloudflare.Zone)
	allSame := project.Domains.Entry == project.Domains.Panel && project.Domains.Entry == project.Domains.WireGuard
	if allSame {
		fmt.Fprintf(w, "  对外域名:      %s（面板、代理入口、WG Endpoint 共用）\n", project.Domains.Entry)
	} else {
		fmt.Fprintf(w, "  入口域名:      %s\n", project.Domains.Entry)
		if project.Domains.Panel == project.Domains.Entry {
			fmt.Fprintf(w, "  面板域名:      %s（与入口域名相同）\n", project.Domains.Panel)
		} else {
			fmt.Fprintf(w, "  面板域名:      %s\n", project.Domains.Panel)
		}
		if project.Domains.WireGuard == project.Domains.Entry {
			fmt.Fprintf(w, "  WG Endpoint:   %s（复用对外域名）\n", project.Domains.WireGuard)
		} else {
			fmt.Fprintf(w, "  WG Endpoint:   %s\n", project.Domains.WireGuard)
		}
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

func printNodeSummary(w io.Writer, label string, node model.Node, isLocal bool) {
	fmt.Fprintf(w, "%s:\n", label)
	fmt.Fprintf(w, "  公网 IP:  %s\n", node.Host)
	if isLocal {
		fmt.Fprintln(w, "  部署方式: 本机")
	} else {
		fmt.Fprintf(w, "  SSH:      %s@%s (%s)\n", node.SSH.User, node.SSHAddr(), authLabel(node.SSH.AuthMethod))
		if node.SSHHost != "" && node.SSHHost != node.Host {
			fmt.Fprintf(w, "  SSH 地址: %s（独立于公网 IP）\n", node.SSHHost)
		}
	}
	fmt.Fprintf(w, "  WG 地址:  %s\n", node.WGAddress)
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
