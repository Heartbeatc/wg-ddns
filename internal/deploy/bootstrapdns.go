package deploy

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"wg-ddns/internal/cloudflare"
	"wg-ddns/internal/model"
)

type dnsTarget struct {
	Name string
	IP   string
}

func EnsureManagedDNS(stdout io.Writer, project model.Project) error {
	targets, err := managedDNSTargets(project)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}

	fmt.Fprintln(stdout, "\n--- 预创建 DNS ---")
	cf, err := cloudflare.New(project.Cloudflare)
	if err != nil {
		return fmt.Errorf("DNS 预创建失败: %w", err)
	}

	ctx := context.Background()
	grouped := make(map[string][]string)
	for _, t := range targets {
		grouped[t.IP] = append(grouped[t.IP], t.Name)
	}

	ips := make([]string, 0, len(grouped))
	for ip := range grouped {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	for _, ip := range ips {
		names := grouped[ip]
		sort.Strings(names)
		changes, err := cf.EnsureDNSRecords(ctx, project.Cloudflare, names, ip, false)
		if err != nil {
			return fmt.Errorf("DNS 预创建失败: %w", err)
		}
		if len(changes) == 0 {
			for _, name := range names {
				fmt.Fprintf(stdout, "  %s 已符合预期 -> %s\n", name, ip)
			}
			continue
		}
		for _, ch := range changes {
			switch ch.Action {
			case "create":
				fmt.Fprintf(stdout, "  创建 %s -> %s\n", ch.Name, ch.After)
			case "update":
				fmt.Fprintf(stdout, "  更新 %s: %s => %s\n", ch.Name, ch.Before, ch.After)
			}
		}
	}

	fmt.Fprintln(stdout, "  等待 DNS 生效（快速轮询 2 分钟，必要时每 1 分钟重试 3 次）")
	if err := waitForManagedDNS(stdout, targets, net.LookupIP); err != nil {
		return err
	}
	return nil
}

func managedDNSTargets(project model.Project) ([]dnsTarget, error) {
	zone := strings.TrimSpace(project.Cloudflare.Zone)
	targetMap := map[string]string{}
	add := func(name, ip string) error {
		name = strings.TrimSpace(name)
		ip = strings.TrimSpace(ip)
		if name == "" || ip == "" || net.ParseIP(name) != nil {
			return nil
		}
		if !isManagedDomain(name, zone) {
			return nil
		}
		if prev, ok := targetMap[name]; ok && prev != ip {
			return fmt.Errorf("域名 %s 同时指向两个节点（%s / %s），请拆分配置", name, prev, ip)
		}
		targetMap[name] = ip
		return nil
	}

	for _, name := range project.Domains.Unique() {
		if err := add(name, project.Nodes.US.Host); err != nil {
			return nil, err
		}
	}
	if err := add(project.Nodes.US.SSHHost, project.Nodes.US.Host); err != nil {
		return nil, err
	}
	if err := add(project.Nodes.HK.SSHHost, project.Nodes.HK.Host); err != nil {
		return nil, err
	}
	if project.ExitDDNS.Enabled {
		if err := add(project.ExitDDNS.Domain, project.Nodes.HK.Host); err != nil {
			return nil, err
		}
	}

	targets := make([]dnsTarget, 0, len(targetMap))
	for name, ip := range targetMap {
		targets = append(targets, dnsTarget{Name: name, IP: ip})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Name < targets[j].Name })
	return targets, nil
}

func isManagedDomain(name, zone string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	zone = strings.ToLower(strings.TrimSpace(zone))
	if name == "" || zone == "" {
		return false
	}
	return name == zone || strings.HasSuffix(name, "."+zone)
}

func waitForManagedDNS(stdout io.Writer, targets []dnsTarget, lookup func(string) ([]net.IP, error)) error {
	check := func() []string { return unresolvedTargets(targets, lookup) }

	if pending := check(); len(pending) == 0 {
		fmt.Fprintln(stdout, "  DNS 已生效。")
		return nil
	}

	for i := 0; i < 24; i++ {
		time.Sleep(5 * time.Second)
		if pending := check(); len(pending) == 0 {
			fmt.Fprintln(stdout, "  DNS 已生效。")
			return nil
		} else if i == 23 {
			fmt.Fprintf(stdout, "  首轮等待结束，仍未生效：%s\n", strings.Join(pending, "; "))
		}
	}

	for retry := 1; retry <= 3; retry++ {
		fmt.Fprintf(stdout, "  第 %d 次重试：60 秒后再次检查 DNS...\n", retry)
		time.Sleep(60 * time.Second)
		if pending := check(); len(pending) == 0 {
			fmt.Fprintln(stdout, "  DNS 已生效。")
			return nil
		} else if retry == 3 {
			return fmt.Errorf("DNS 仍未生效：%s", strings.Join(pending, "; "))
		}
	}

	return nil
}

func unresolvedTargets(targets []dnsTarget, lookup func(string) ([]net.IP, error)) []string {
	var pending []string
	for _, t := range targets {
		ips, err := lookup(t.Name)
		if err != nil {
			pending = append(pending, fmt.Sprintf("%s 未解析到 %s", t.Name, t.IP))
			continue
		}
		ok := false
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil && v4.String() == t.IP {
				ok = true
				break
			}
		}
		if !ok {
			pending = append(pending, fmt.Sprintf("%s 未解析到 %s", t.Name, t.IP))
		}
	}
	return pending
}
