package wizard

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"wg-ddns/internal/cloudflare"
	"wg-ddns/internal/config"
	"wg-ddns/internal/deploy"
	"wg-ddns/internal/model"
	"wg-ddns/internal/sshclient"
)

// VerifyEntrySSH checks SSH (or local) connectivity and basic root/systemd environment.
func VerifyEntrySSH(w io.Writer, project model.Project, rc model.RunContext) error {
	if err := config.ValidateDeploy(project, rc); err != nil {
		return fmt.Errorf("配置不完整，无法验证: %w", err)
	}
	client, err := dialNodeForWizardVerify(w, project.Nodes.US, rc.EntryIsLocal)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer client.Close()

	out, err := client.RunShell("id -u")
	if err != nil {
		return fmt.Errorf("执行 id -u 失败: %w: %s", err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "0" {
		return fmt.Errorf("需要 root，当前 uid=%s", strings.TrimSpace(out))
	}

	out, err = client.RunShell("command -v systemctl >/dev/null 2>&1 && echo ok")
	if err != nil || strings.TrimSpace(out) != "ok" {
		return fmt.Errorf("systemd 不可用")
	}

	fmt.Fprintln(w, "  入口节点验证通过：SSH/本机可用，root + systemd 正常。")
	return nil
}

// VerifyExitSSH checks SSH (or local) connectivity and basic root/systemd environment.
func VerifyExitSSH(w io.Writer, project model.Project, rc model.RunContext) error {
	if err := config.ValidateDeploy(project, rc); err != nil {
		return fmt.Errorf("配置不完整，无法验证: %w", err)
	}
	client, err := dialNodeForWizardVerify(w, project.Nodes.HK, rc.ExitIsLocal)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer client.Close()

	out, err := client.RunShell("id -u")
	if err != nil {
		return fmt.Errorf("执行 id -u 失败: %w: %s", err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "0" {
		return fmt.Errorf("需要 root，当前 uid=%s", strings.TrimSpace(out))
	}

	out, err = client.RunShell("command -v systemctl >/dev/null 2>&1 && echo ok")
	if err != nil || strings.TrimSpace(out) != "ok" {
		return fmt.Errorf("systemd 不可用")
	}

	fmt.Fprintln(w, "  出口节点验证通过：SSH/本机可用，root + systemd 正常。")
	return nil
}

// VerifyCloudflare checks token and zone resolution (implies DNS API access).
func VerifyCloudflare(w io.Writer, project model.Project) error {
	if strings.TrimSpace(project.Cloudflare.Zone) == "" {
		return fmt.Errorf("请先填写 Cloudflare Zone")
	}
	if _, _, err := cloudflare.ResolveToken(project.Cloudflare); err != nil {
		return err
	}
	cf, err := cloudflare.New(project.Cloudflare)
	if err != nil {
		return err
	}
	if err := cf.VerifyZone(context.Background()); err != nil {
		return fmt.Errorf("Cloudflare 验证失败（请检查 Token 与 Zone）: %w", err)
	}
	fmt.Fprintln(w, "  Cloudflare 验证通过：Token 有效，Zone 可访问。")
	return nil
}

// VerifyDomains performs format checks and DNS resolution hints vs entry public IP.
func VerifyDomains(w io.Writer, project model.Project) error {
	d := project.Domains
	for _, label := range []struct{ name, val string }{
		{"入口/对外域名", d.Entry},
		{"面板域名", d.Panel},
		{"WireGuard Endpoint 域名", d.WireGuard},
	} {
		if msg := validateDomain(label.val); msg != "" {
			return fmt.Errorf("%s: %s", label.name, msg)
		}
	}

	entryIP := strings.TrimSpace(project.Nodes.US.Host)
	names := project.Domains.Unique()
	var lines []string
	for _, name := range names {
		ips, err := net.LookupIP(name)
		if err != nil {
			if isManagedVerifyDomain(name, project) {
				lines = append(lines, fmt.Sprintf("%s: 暂未解析（部署时会自动创建/更新）", name))
			} else {
				lines = append(lines, fmt.Sprintf("%s: 解析失败 — %v", name, err))
			}
			continue
		}
		var v4s []string
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				v4s = append(v4s, v4.String())
			}
		}
		if len(v4s) == 0 {
			lines = append(lines, fmt.Sprintf("%s: 无 A/AAAA IPv4 记录", name))
			continue
		}
		line := fmt.Sprintf("%s -> %s", name, strings.Join(v4s, ", "))
		if entryIP != "" {
			match := false
			for _, v := range v4s {
				if v == entryIP {
					match = true
					break
				}
			}
			if !match {
				line += fmt.Sprintf("（与入口公网 IP %s 不一致，可能正常若使用 CDN/代理）", entryIP)
			} else {
				line += "（与入口公网 IP 一致）"
			}
		}
		lines = append(lines, line)
	}

	fmt.Fprintln(w, "  域名检查：")
	for _, ln := range lines {
		fmt.Fprintf(w, "    %s\n", ln)
	}
	fmt.Fprintln(w, "  （是否在 Cloudflare 上为「仅 DNS」需登录控制台确认。）")
	return nil
}

// EnsureAndVerifyDomains creates/updates managed Cloudflare records and waits
// until the local resolver sees the desired values.
func EnsureAndVerifyDomains(w io.Writer, project model.Project) error {
	if err := config.Validate(project); err != nil {
		return fmt.Errorf("配置不完整，无法预创建 DNS: %w", err)
	}
	if err := deploy.EnsureManagedDNS(w, project); err != nil {
		return err
	}
	return VerifyDomains(w, project)
}

func dialNodeForWizardVerify(w io.Writer, node model.Node, isLocal bool) (sshclient.Runner, error) {
	if isLocal {
		return sshclient.DialOrLocal(node, true)
	}
	client, err := sshclient.Dial(node)
	if err == nil {
		return client, nil
	}
	if node.SSHHost != "" && net.ParseIP(node.Host) != nil {
		fmt.Fprintf(w, "  SSH 管理域名暂不可用，先使用当前公网 IP %s 进行验证。\n", node.Host)
		fmt.Fprintln(w, "  部署时会自动创建/更新该管理域名的 DNS 记录。")
		temp := node
		temp.SSHHost = ""
		return sshclient.Dial(temp)
	}
	return nil, err
}

func isManagedVerifyDomain(name string, project model.Project) bool {
	zone := strings.ToLower(strings.TrimSpace(project.Cloudflare.Zone))
	name = strings.ToLower(strings.TrimSpace(name))
	if zone == "" || name == "" {
		return false
	}
	return name == zone || strings.HasSuffix(name, "."+zone)
}

func runVerifySubmenu(w io.Writer, p *Prompter, d *SetupDraft) {
	for {
		fmt.Fprintln(w, "\n--- 逐项验证 ---")
		opts := []string{
			"验证入口节点 SSH / 本机环境",
			"验证出口节点 SSH / 本机环境",
			"验证 Cloudflare",
			"预创建/验证域名 DNS",
			"返回配置主菜单",
		}
		ch := p.Select("请选择验证项:", opts)
		if p.Err() != nil {
			return
		}
		var err error
		switch ch {
		case 0:
			err = VerifyEntrySSH(w, d.Project, d.RC)
		case 1:
			err = VerifyExitSSH(w, d.Project, d.RC)
		case 2:
			err = VerifyCloudflare(w, d.Project)
		case 3:
			err = EnsureAndVerifyDomains(w, d.Project)
		case 4:
			return
		}
		if ch >= 0 && ch <= 3 {
			if err != nil {
				fmt.Fprintf(w, "  验证失败: %v\n", err)
			}
		}
	}
}
