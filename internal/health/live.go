package health

import (
	"fmt"
	"net"
	"strings"
	"time"

	"wg-ddns/internal/model"
	"wg-ddns/internal/sshclient"
)

type Probe struct {
	Name     string
	Status   string
	Detail   string
	Duration time.Duration
}

func RunLive(project model.Project, rc model.RunContext) ([]Probe, error) {
	entryClient, err := sshclient.DialOrLocal(project.Nodes.US, rc.EntryIsLocal)
	if err != nil {
		return nil, err
	}
	defer entryClient.Close()

	exitClient, err := sshclient.DialOrLocal(project.Nodes.HK, rc.ExitIsLocal)
	if err != nil {
		return nil, err
	}
	defer exitClient.Close()

	entryIP, err := DetectPublicIPv4(entryClient, project.Checks.PublicIPCheckURL)
	if err != nil {
		return nil, err
	}

	return []Probe{
		timed(func() Probe { return checkDNS(project, entryIP) }),
		timed(func() Probe { return checkWG(entryClient, "入口") }),
		timed(func() Probe { return checkWG(exitClient, "出口") }),
		timed(func() Probe { return checkExitSocks(exitClient, project) }),
		timed(func() Probe { return checkEgress(entryClient, project) }),
	}, nil
}

func RenderLive(probes []Probe) string {
	var b strings.Builder
	for _, probe := range probes {
		if probe.Duration > 0 {
			fmt.Fprintf(&b, "- [%s] %s: %s (%s)\n", probe.Status, probe.Name, probe.Detail, probe.Duration.Round(time.Millisecond))
		} else {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", probe.Status, probe.Name, probe.Detail)
		}
	}
	return b.String()
}

func timed(fn func() Probe) Probe {
	start := time.Now()
	p := fn()
	p.Duration = time.Since(start)
	return p
}

func checkDNS(project model.Project, entryIP string) Probe {
	names := project.Domains.Unique()
	var mismatches []string
	for _, name := range names {
		ips, err := net.LookupIP(name)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("%s 解析失败: %v", name, err))
			continue
		}

		found := false
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil && v4.String() == entryIP {
				found = true
				break
			}
		}
		if !found {
			mismatches = append(mismatches, fmt.Sprintf("%s 未解析到 %s", name, entryIP))
		}
	}

	if len(mismatches) > 0 {
		return Probe{Name: "DNS", Status: "FAIL", Detail: strings.Join(mismatches, "; ")}
	}
	return Probe{Name: "DNS", Status: "PASS", Detail: fmt.Sprintf("所有域名均解析到 %s", entryIP)}
}

func checkWG(client sshclient.Runner, label string) Probe {
	out, err := client.RunShell(`wg show all latest-handshakes`)
	if err != nil {
		return Probe{Name: label + " WireGuard", Status: "FAIL", Detail: strings.TrimSpace(out)}
	}

	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) == 0 {
		return Probe{Name: label + " WireGuard", Status: "FAIL", Detail: "无 peer 信息"}
	}

	var newest int64
	for _, field := range lines {
		ts, parseErr := parseUnix(field)
		if parseErr == nil && ts > newest {
			newest = ts
		}
	}
	if newest == 0 {
		return Probe{Name: label + " WireGuard", Status: "FAIL", Detail: "所有 peer 握手时间为 0"}
	}
	age := time.Since(time.Unix(newest, 0)).Round(time.Second)
	return Probe{Name: label + " WireGuard", Status: "PASS", Detail: "最近握手 " + age.String() + " 前"}
}

func checkExitSocks(client sshclient.Runner, project model.Project) Probe {
	listen := project.Nodes.HK.SocksListen
	command := fmt.Sprintf("ss -lnt | grep -F %q", listen)
	out, err := client.RunShell(command)
	if err != nil {
		return Probe{Name: "出口 SOCKS", Status: "FAIL", Detail: strings.TrimSpace(out)}
	}
	if strings.TrimSpace(out) == "" {
		return Probe{Name: "出口 SOCKS", Status: "FAIL", Detail: "未发现监听"}
	}
	return Probe{Name: "出口 SOCKS", Status: "PASS", Detail: listen + " 正在监听"}
}

func checkEgress(client sshclient.Runner, project model.Project) Probe {
	ipCmd := fmt.Sprintf(
		"curl -4fsS --max-time 20 --socks5-hostname %s %s",
		shellEscape(project.Nodes.HK.SocksListen),
		shellEscape(project.Checks.ExitCheckURL),
	)
	out, err := client.RunShell(ipCmd)
	if err != nil {
		return Probe{Name: "出口验证", Status: "FAIL", Detail: "连通性失败: " + strings.TrimSpace(out)}
	}
	exitIP := strings.TrimSpace(out)

	if !IsPublicIPv4(exitIP) {
		return Probe{
			Name:   "出口验证",
			Status: "FAIL",
			Detail: fmt.Sprintf("exit_check_url 返回的不是合法公网 IPv4: %q（该字段必须指向一个返回纯文本公网 IP 的接口，如 https://api.ipify.org）", exitIP),
		}
	}

	if project.Checks.ExitLocation == "" {
		return Probe{Name: "出口验证", Status: "PASS", Detail: fmt.Sprintf("出口 IP: %s（未配置预期地区，跳过校验）", exitIP)}
	}

	geoCmd := fmt.Sprintf(
		"curl -4fsS --max-time 10 --socks5-hostname %s %s",
		shellEscape(project.Nodes.HK.SocksListen),
		shellEscape(fmt.Sprintf("https://ipinfo.io/%s/country", exitIP)),
	)
	geoOut, geoErr := client.RunShell(geoCmd)
	if geoErr != nil {
		return Probe{Name: "出口验证", Status: "PASS", Detail: fmt.Sprintf("出口 IP: %s（地区查询失败，跳过校验）", exitIP)}
	}
	country := strings.TrimSpace(geoOut)

	if !strings.EqualFold(country, project.Checks.ExitLocation) {
		return Probe{Name: "出口验证", Status: "FAIL", Detail: fmt.Sprintf("期望 %s，实际 %s（出口 IP: %s）", project.Checks.ExitLocation, country, exitIP)}
	}
	return Probe{Name: "出口验证", Status: "PASS", Detail: fmt.Sprintf("出口 IP: %s，地区: %s", exitIP, country)}
}

var fallbackIPServices = []string{
	"https://api.ipify.org",
	"https://ipv4.icanhazip.com",
}

// DetectPublicIPv4 detects the public IPv4 of the remote (or local) host.
// It tries the configured URL first, then falls back to well-known services.
// It NEVER falls back to local route addresses to avoid returning private IPs
// that would corrupt DNS records.
func DetectPublicIPv4(client sshclient.Runner, primaryURL string) (string, error) {
	urls := make([]string, 0, 1+len(fallbackIPServices))
	if primaryURL != "" {
		urls = append(urls, primaryURL)
	}
	for _, u := range fallbackIPServices {
		if u != primaryURL {
			urls = append(urls, u)
		}
	}

	var lastErr error
	for _, u := range urls {
		out, err := client.RunShell(fmt.Sprintf("curl -4fsS --max-time 10 %s", shellEscape(u)))
		if err != nil {
			lastErr = fmt.Errorf("请求 %s 失败: %w", u, err)
			continue
		}
		ip := strings.TrimSpace(out)
		if ip == "" {
			continue
		}
		if !IsPublicIPv4(ip) {
			lastErr = fmt.Errorf("从 %s 获取到的 IP %q 不是合法公网 IPv4 地址", u, ip)
			continue
		}
		return ip, nil
	}

	if lastErr != nil {
		return "", fmt.Errorf("无法检测公网 IP: %w", lastErr)
	}
	return "", fmt.Errorf("无法检测公网 IP: 所有检测服务均未返回有效结果")
}

// IsPublicIPv4 returns true if s is a valid, globally routable IPv4 address.
func IsPublicIPv4(s string) bool {
	ip := net.ParseIP(s)
	if ip == nil {
		return false
	}
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	if v4[0] == 0 {
		return false
	}
	if v4[0] == 10 {
		return false
	}
	if v4[0] == 127 {
		return false
	}
	if v4[0] == 169 && v4[1] == 254 {
		return false
	}
	if v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31 {
		return false
	}
	if v4[0] == 192 && v4[1] == 168 {
		return false
	}
	if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return false
	}
	if v4[0] >= 224 {
		return false
	}
	return true
}

func parseUnix(v string) (int64, error) {
	var n int64
	for _, ch := range v {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("not unix timestamp")
		}
		n = n*10 + int64(ch-'0')
	}
	return n, nil
}

func shellEscape(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
