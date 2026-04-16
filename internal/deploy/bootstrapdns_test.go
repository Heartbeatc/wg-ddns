package deploy

import (
	"net"
	"testing"

	"wg-ddns/internal/config"
)

func TestManagedDNSTargets(t *testing.T) {
	project := config.DefaultProject()
	project.Cloudflare.Zone = "hmcn.ai"
	project.Domains.Entry = "b.hmcn.ai"
	project.Domains.Panel = "b.hmcn.ai"
	project.Domains.WireGuard = "b.hmcn.ai"
	project.Nodes.US.Host = "1.2.3.4"
	project.Nodes.US.SSHHost = "ssh-entry.hmcn.ai"
	project.Nodes.HK.Host = "5.6.7.8"
	project.Nodes.HK.SSHHost = "jp.hmcn.ai"
	project.ExitDDNS.Enabled = true
	project.ExitDDNS.Domain = "jp.hmcn.ai"

	targets, err := managedDNSTargets(project)
	if err != nil {
		t.Fatalf("managedDNSTargets() error = %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("managedDNSTargets() len = %d, want 3", len(targets))
	}
	got := map[string]string{}
	for _, t0 := range targets {
		got[t0.Name] = t0.IP
	}
	if got["b.hmcn.ai"] != "1.2.3.4" {
		t.Fatalf("entry domain = %q, want 1.2.3.4", got["b.hmcn.ai"])
	}
	if got["ssh-entry.hmcn.ai"] != "1.2.3.4" {
		t.Fatalf("entry ssh domain = %q, want 1.2.3.4", got["ssh-entry.hmcn.ai"])
	}
	if got["jp.hmcn.ai"] != "5.6.7.8" {
		t.Fatalf("exit ssh domain = %q, want 5.6.7.8", got["jp.hmcn.ai"])
	}
}

func TestManagedDNSTargetsConflict(t *testing.T) {
	project := config.DefaultProject()
	project.Cloudflare.Zone = "hmcn.ai"
	project.Nodes.US.Host = "1.2.3.4"
	project.Nodes.HK.Host = "5.6.7.8"
	project.Nodes.US.SSHHost = "same.hmcn.ai"
	project.Nodes.HK.SSHHost = "same.hmcn.ai"

	if _, err := managedDNSTargets(project); err == nil {
		t.Fatal("managedDNSTargets() expected conflict error")
	}
}

func TestUnresolvedTargets(t *testing.T) {
	targets := []dnsTarget{
		{Name: "ok.hmcn.ai", IP: "1.1.1.1"},
		{Name: "bad.hmcn.ai", IP: "2.2.2.2"},
	}
	lookup := func(name string) ([]net.IP, error) {
		switch name {
		case "ok.hmcn.ai":
			return []net.IP{net.ParseIP("1.1.1.1")}, nil
		case "bad.hmcn.ai":
			return []net.IP{net.ParseIP("9.9.9.9")}, nil
		default:
			return nil, nil
		}
	}

	pending := unresolvedTargets(targets, lookup)
	if len(pending) != 1 {
		t.Fatalf("unresolvedTargets() len = %d, want 1", len(pending))
	}
	if pending[0] != "bad.hmcn.ai 未解析到 2.2.2.2" {
		t.Fatalf("unexpected pending detail: %s", pending[0])
	}
}
