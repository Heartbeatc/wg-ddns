package wizard

import (
	"fmt"
	"io"
	"net"
	"strings"

	"wg-ddns/internal/health"
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

// collectNodeInfo gathers host/SSH info for one node.
// When isLocal is true, SSH fields are skipped and the public IP is auto-detected.
func collectNodeInfo(w io.Writer, p *Prompter, label string, isLocal bool) nodeInput {
	return collectNodeInfoWithDefaults(w, p, label, isLocal, model.Node{})
}

// collectNodeInfoWithDefaults is like collectNodeInfo but pre-fills from prev when non-empty.
func collectNodeInfoWithDefaults(w io.Writer, p *Prompter, label string, isLocal bool, prev model.Node) nodeInput {
	if isLocal {
		fmt.Fprintf(w, "  当前机器即%s，无需 SSH 配置。\n", label)
		fmt.Fprintln(w, "  正在自动检测本机公网 IP...")
		def := strings.TrimSpace(prev.Host)
		host := detectOrAskIPWithDefault(w, p, label, def)
		return nodeInput{host: host, user: "root"}
	}

	hostDef := strings.TrimSpace(prev.Host)
	host := p.LineWith(label+"的公网 IP 地址", hostDef, validateIP)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  SSH 连接地址用于本工具远程管理该节点。")
	fmt.Fprintln(w, "  如果节点 IP 可能变化（如入口节点换 IP，或出口节点使用家宽动态 IP），")
	fmt.Fprintln(w, "  推荐填写一个稳定的域名，这样即使 IP 漂移，工具仍能连上节点。")
	fmt.Fprintln(w, "  如果 IP 不会变化，直接回车使用公网 IP 即可。")
	if sh := strings.TrimSpace(prev.SSHHost); sh != "" {
		fmt.Fprintf(w, "  当前 SSH 地址: %s\n", sh)
	}
	sshHost := p.OptionalLine("SSH 连接地址（域名或 IP，留空则使用公网 IP）")

	userDef := strings.TrimSpace(prev.SSH.User)
	if userDef == "" {
		userDef = "root"
	}
	user := p.LineWith("SSH 用户名", userDef, nil)

	authIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})

	ni := nodeInput{host: host, sshHost: sshHost, user: user, authMethod: "password"}
	if authIdx == 0 {
		ni.authMethod = "password"
		pw := p.PasswordOptional("SSH 密码")
		if pw != "" {
			ni.password = pw
		} else if strings.TrimSpace(prev.SSH.Password) != "" {
			ni.password = prev.SSH.Password
		} else {
			ni.password = p.Password("SSH 密码")
		}
	} else {
		ni.authMethod = "private_key"
		keyDef := strings.TrimSpace(prev.SSH.PrivateKeyPath)
		if keyDef == "" {
			keyDef = "~/.ssh/id_rsa"
		}
		ni.keyPath = p.LineWith("私钥文件路径", keyDef, nil)
	}
	return ni
}

// detectOrAskIP tries to auto-detect the local machine's public IP.
// On success it asks for confirmation; on failure it falls back to manual input.
func detectOrAskIP(w io.Writer, p *Prompter, label string) string {
	return detectOrAskIPWithDefault(w, p, label, "")
}

// detectOrAskIPWithDefault is like detectOrAskIP; if defaultIP is set it is offered as fallback default.
func detectOrAskIPWithDefault(w io.Writer, p *Prompter, label, defaultIP string) string {
	localClient := sshclient.NewLocal()
	detectedIP, detectErr := health.DetectPublicIPv4(localClient, "https://api.ipify.org")
	localClient.Close()

	if detectErr == nil {
		fmt.Fprintf(w, "  检测到公网 IP: %s\n", detectedIP)
		fmt.Fprintln(w)
		if p.Confirm("使用此 IP？", true) {
			return detectedIP
		}
		def := defaultIP
		if def == "" {
			def = detectedIP
		}
		return p.LineWith(label+"的公网 IP", def, validateIP)
	}

	fmt.Fprintf(w, "  自动检测失败: %v\n", detectErr)
	fmt.Fprintln(w)
	def := defaultIP
	if def == "" {
		def = ""
	}
	return p.LineWith(label+"的公网 IP（请手动输入）", def, validateIP)
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
	fmt.Fprintln(w)
	fmt.Fprintln(w, "接下来是「配置菜单」：可按任意顺序填写各项，随时返回修改，")
	fmt.Fprintln(w, "确认无误后再保存或部署，无需一次性从头填到尾。")
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
	if project.ExitDDNS.Enabled {
		fmt.Fprintln(w, "出口管理 DDNS:")
		fmt.Fprintf(w, "  管理域名:    %s（DNS only）\n", project.ExitDDNS.Domain)
		fmt.Fprintf(w, "  刷新间隔:    %ds\n", project.ExitDDNS.Interval)
		fmt.Fprintln(w, "  出口节点会自动维护此域名指向当前公网 IP")
		fmt.Fprintln(w)
	}
	if project.EntryAutoReconcile.Enabled {
		fmt.Fprintln(w, "入口自动修复:")
		fmt.Fprintf(w, "  刷新间隔:    %ds\n", project.EntryAutoReconcile.Interval)
		fmt.Fprintln(w, "  入口 IP 变化后自动更新 DNS、刷新出口 WG、推送通知")
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "将要执行的操作：")
	fmt.Fprintln(w, "  1. 在两台节点上安装 WireGuard（如果未安装）")
	fmt.Fprintln(w, "  2. 在出口节点上安装 sing-box（如果未安装）")
	fmt.Fprintln(w, "  3. 下发 WireGuard 和 sing-box 配置文件")
	fmt.Fprintln(w, "  4. 启动/重启服务")
	n := 5
	if project.ExitDDNS.Enabled {
		fmt.Fprintf(w, "  %d. 在出口节点部署管理 DDNS 更新器\n", n)
		n++
	}
	if project.EntryAutoReconcile.Enabled {
		fmt.Fprintf(w, "  %d. 在入口节点部署自动修复定时器\n", n)
	}
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
