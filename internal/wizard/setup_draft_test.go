package wizard

import (
	"strings"
	"testing"

	"wg-ddns/internal/model"
)

func TestSetupDraftStatusRunLocation(t *testing.T) {
	d := &SetupDraft{Project: model.Project{}}
	if got := d.statusRunLocation(); got != "未配置" {
		t.Fatalf("want 未配置, got %q", got)
	}
	d.RCSet = true
	d.RC = model.RunContext{EntryIsLocal: true}
	if !strings.Contains(d.statusRunLocation(), "已配置") {
		t.Fatalf("expected 已配置, got %q", d.statusRunLocation())
	}
}

func TestSetupDraftStatusEntryRemote(t *testing.T) {
	d := &SetupDraft{
		RCSet: true,
		RC:    model.RunContext{},
		Project: model.Project{
			Nodes: model.Nodes{
				US: model.Node{
					Host: "1.2.3.4",
					SSH: model.SSH{
						User:       "root",
						Port:       22,
						AuthMethod: "password",
						Password:   "x",
					},
				},
			},
		},
	}
	if !strings.Contains(d.statusEntry(), "已配置") {
		t.Fatalf("got %q", d.statusEntry())
	}
}

func TestSetupDraftStatusCloudflare(t *testing.T) {
	d := &SetupDraft{Project: model.Project{Cloudflare: model.Cloudflare{Zone: "example.com", Token: "t"}}}
	if !strings.Contains(d.statusCloudflare(), "已配置") {
		t.Fatalf("got %q", d.statusCloudflare())
	}
}
