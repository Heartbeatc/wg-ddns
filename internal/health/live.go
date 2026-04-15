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

func DetectPublicIPv4(client *sshclient.Client, externalURL string) (string, error) {
	if externalURL != "" {
		out, err := client.RunShell(fmt.Sprintf("curl -4fsS --max-time 15 %s", shellEscape(externalURL)))
		if err == nil {
			value := strings.TrimSpace(out)
			if value != "" {
				return value, nil
			}
		}
	}

	out, err := client.RunShell(`ip -4 route get 1.1.1.1 | awk '/src/ {for (i = 1; i <= NF; i++) if ($i == "src") {print $(i+1); exit}}'`)
	if err != nil {
		return "", fmt.Errorf("detect US public IP: %w: %s", err, strings.TrimSpace(out))
	}
	value := strings.TrimSpace(out)
	if value == "" {
		return "", fmt.Errorf("detect US public IP: empty output")
	}
	return value, nil
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
