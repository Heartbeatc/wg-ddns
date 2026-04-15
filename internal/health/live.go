package health

import (
	"fmt"
	"net"
	"strings"
	"time"

	"wg-ddns/internal/address"
	"wg-ddns/internal/model"
	"wg-ddns/internal/sshclient"
)

type Probe struct {
	Name   string
	Status string
	Detail string
}

func RunLive(project model.Project) ([]Probe, error) {
	usClient, err := sshclient.Dial(project.Nodes.US)
	if err != nil {
		return nil, err
	}
	defer usClient.Close()

	hkClient, err := sshclient.Dial(project.Nodes.HK)
	if err != nil {
		return nil, err
	}
	defer hkClient.Close()

	usIP, err := DetectPublicIPv4(usClient, project.Checks.PublicIPCheckURL)
	if err != nil {
		return nil, err
	}

	return []Probe{
		checkDNS(project, usIP),
		checkWG(usClient, "US"),
		checkWG(hkClient, "HK"),
		checkHKSocks(hkClient, project),
		checkExit(usClient, project),
	}, nil
}

func RenderLive(probes []Probe) string {
	var b strings.Builder
	for _, probe := range probes {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", probe.Status, probe.Name, probe.Detail)
	}
	return b.String()
}

func checkDNS(project model.Project, usIP string) Probe {
	names := []string{project.Domains.Entry, project.Domains.Panel, project.Domains.WireGuard}
	var mismatches []string
	for _, name := range names {
		ips, err := net.LookupIP(name)
		if err != nil {
			mismatches = append(mismatches, fmt.Sprintf("%s resolve failed: %v", name, err))
			continue
		}

		found := false
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil && v4.String() == usIP {
				found = true
				break
			}
		}
		if !found {
			mismatches = append(mismatches, fmt.Sprintf("%s does not contain %s", name, usIP))
		}
	}

	if len(mismatches) > 0 {
		return Probe{Name: "DNS", Status: "FAIL", Detail: strings.Join(mismatches, "; ")}
	}
	return Probe{Name: "DNS", Status: "PASS", Detail: fmt.Sprintf("all managed names resolve to %s", usIP)}
}

func checkWG(client *sshclient.Client, label string) Probe {
	out, err := client.RunShell(`wg show all latest-handshakes`)
	if err != nil {
		return Probe{Name: label + " WireGuard", Status: "FAIL", Detail: strings.TrimSpace(out)}
	}

	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) == 0 {
		return Probe{Name: label + " WireGuard", Status: "FAIL", Detail: "no peers reported by wg show"}
	}

	var newest int64
	for _, field := range lines {
		ts, parseErr := parseUnix(field)
		if parseErr == nil && ts > newest {
			newest = ts
		}
	}
	if newest == 0 {
		return Probe{Name: label + " WireGuard", Status: "FAIL", Detail: "all peer handshakes are 0"}
	}
	age := time.Since(time.Unix(newest, 0)).Round(time.Second)
	return Probe{Name: label + " WireGuard", Status: "PASS", Detail: "latest handshake age " + age.String()}
}

func checkHKSocks(client *sshclient.Client, project model.Project) Probe {
	listen := project.Nodes.HK.SocksListen
	command := fmt.Sprintf("ss -lnt | grep -F %q", listen)
	out, err := client.RunShell(command)
	if err != nil {
		return Probe{Name: "HK SOCKS", Status: "FAIL", Detail: strings.TrimSpace(out)}
	}
	if strings.TrimSpace(out) == "" {
		return Probe{Name: "HK SOCKS", Status: "FAIL", Detail: "listener not found"}
	}
	return Probe{Name: "HK SOCKS", Status: "PASS", Detail: listen + " is listening"}
}

func checkExit(client *sshclient.Client, project model.Project) Probe {
	command := fmt.Sprintf(
		"curl -fsS --max-time 20 --socks5-hostname %s %s",
		shellEscape(project.Nodes.HK.SocksListen),
		shellEscape(project.Checks.ExitCheckURL),
	)
	out, err := client.RunShell(command)
	if err != nil {
		return Probe{Name: "Egress", Status: "FAIL", Detail: strings.TrimSpace(out)}
	}
	value := strings.TrimSpace(out)
	if value != project.Checks.ExitLocation {
		return Probe{Name: "Egress", Status: "FAIL", Detail: fmt.Sprintf("expected %s, got %s", project.Checks.ExitLocation, value)}
	}
	return Probe{Name: "Egress", Status: "PASS", Detail: fmt.Sprintf("curl via %s returned %s", address.Host(project.Nodes.HK.SocksListen), value)}
}

var fallbackIPServices = []string{
	"https://api.ipify.org",
	"https://ipv4.icanhazip.com",
}

// DetectPublicIPv4 detects the public IPv4 of the remote host.
// It tries the configured URL first, then falls back to well-known services.
// It NEVER falls back to local route addresses to avoid returning private IPs
// that would corrupt DNS records.
func DetectPublicIPv4(client *sshclient.Client, primaryURL string) (string, error) {
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
// It rejects private (RFC 1918), loopback, link-local, CGNAT, multicast,
// and other reserved ranges.
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
		return false // 0.0.0.0/8
	}
	if v4[0] == 10 {
		return false // 10.0.0.0/8
	}
	if v4[0] == 127 {
		return false // 127.0.0.0/8
	}
	if v4[0] == 169 && v4[1] == 254 {
		return false // 169.254.0.0/16 link-local
	}
	if v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31 {
		return false // 172.16.0.0/12
	}
	if v4[0] == 192 && v4[1] == 168 {
		return false // 192.168.0.0/16
	}
	if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return false // 100.64.0.0/10 CGNAT
	}
	if v4[0] >= 224 {
		return false // 224.0.0.0/4 multicast + 240.0.0.0/4 reserved
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
