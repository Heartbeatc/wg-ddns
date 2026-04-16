package wizard

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
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
		fmt.Fprintln(w, helpStyle.Render(fmt.Sprintf("  当前机器即%s，无需 SSH 配置。", label)))
		fmt.Fprintln(w, helpStyle.Render("  正在自动检测本机公网 IP..."))
		def := strings.TrimSpace(prev.Host)
		host := detectOrAskIPWithDefault(w, p, label, def)
		return nodeInput{host: host, user: "root"}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, helpStyle.Render("  先填写首次连接地址；连上节点后会自动探测当前公网 IP。"))
	sshDef := strings.TrimSpace(prev.SSHHost)
	if sshDef == "" {
		sshDef = strings.TrimSpace(prev.Host)
	}
	sshHost := p.LineWith("首次连接地址（IP 或已存在域名）", sshDef, nil)

	userDef := strings.TrimSpace(prev.SSH.User)
	if userDef == "" {
		userDef = "root"
	}
	user := p.LineWith("SSH 用户名", userDef, nil)

	authIdx := p.Select("SSH 登录方式:", []string{"密码", "私钥文件"})

	ni := nodeInput{sshHost: sshHost, user: user, authMethod: "password"}
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

	hostDef := strings.TrimSpace(prev.Host)
	ni.host = detectOrAskRemoteIP(w, p, label, ni, hostDef)
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
		fmt.Fprintf(w, "%s\n", successTextStyle.Render("  检测到当前机器公网 IP: "+detectedIP))
		if strings.Contains(label, "入口") {
			fmt.Fprintln(w, helpStyle.Render("  这里只确认当前公网 IP；入口业务域名会在后面的域名步骤设置。"))
		} else {
			fmt.Fprintln(w, helpStyle.Render("  这里只确认当前公网 IP；SSH 管理域名会在后续步骤继续配置。"))
		}
		fmt.Fprintln(w)
		if p.Confirm("确认使用这个公网 IP 作为该节点当前地址？", true) {
			return detectedIP
		}
		def := defaultIP
		if def == "" {
			def = detectedIP
		}
		return p.LineWith(label+"当前公网 IP（域名稍后设置）", def, validateIP)
	}

	fmt.Fprintf(w, "%s\n", warnTextStyle.Render("  自动检测失败: "+detectErr.Error()))
	fmt.Fprintln(w)
	def := defaultIP
	if def == "" {
		def = ""
	}
	return p.LineWith(label+"当前公网 IP（请手动输入，域名稍后设置）", def, validateIP)
}

func detectOrAskRemoteIP(w io.Writer, p *Prompter, label string, ni nodeInput, defaultIP string) string {
	temp := model.Node{
		Host:    defaultIP,
		SSHHost: ni.sshHost,
		SSH: model.SSH{
			User:                  ni.user,
			Port:                  22,
			AuthMethod:            ni.authMethod,
			Password:              ni.password,
			PrivateKeyPath:        ni.keyPath,
			InsecureIgnoreHostKey: true,
		},
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, helpStyle.Render("  正在通过 SSH 检测该节点当前公网 IP..."))
	client, err := sshclient.Dial(temp)
	if err == nil {
		defer client.Close()
		detectedIP, detectErr := health.DetectPublicIPv4(client, "https://api.ipify.org")
		if detectErr == nil {
			fmt.Fprintf(w, "%s\n", successTextStyle.Render("  通过 SSH 检测到当前公网 IP: "+detectedIP))
			fmt.Fprintln(w, helpStyle.Render("  该 IP 仅用于 DDNS、健康检查和部署摘要；业务域名会在后面的域名步骤设置。"))
			fmt.Fprintln(w)
			if p.Confirm("确认使用这个公网 IP 作为该节点当前地址？", true) {
				return detectedIP
			}
			def := defaultIP
			if def == "" {
				def = detectedIP
			}
			return p.LineWith(label+"当前公网 IP（用于 DDNS / 健康检查）", def, validateIP)
		}
		fmt.Fprintf(w, "%s\n", warnTextStyle.Render("  已连上节点，但自动检测公网 IP 失败: "+detectErr.Error()))
	} else {
		fmt.Fprintf(w, "%s\n", warnTextStyle.Render("  首次连接失败，无法自动检测公网 IP: "+err.Error()))
	}

	def := defaultIP
	if def == "" && validateIP(ni.sshHost) == "" {
		def = ni.sshHost
	}
	return p.LineWith(label+"当前公网 IP（用于 DDNS / 健康检查）", def, validateIP)
}

func printWelcome(w io.Writer) {
	wd, _ := os.Getwd()
	fmt.Fprintln(w)
	fmt.Fprintln(w, renderWelcome(shortenPath(wd)))
}

func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+string(filepath.Separator)) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
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
