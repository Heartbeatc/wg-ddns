package planner

import (
	"fmt"
	"strings"

	"wg-ddns/internal/address"
	"wg-ddns/internal/model"
)

type Step struct {
	Title   string
	Details []string
}

func Build(project model.Project) []Step {
	return []Step{
		{
			Title: "Prepare US entry host",
			Details: []string{
				fmt.Sprintf("Install or validate WireGuard on %s (%s)", project.Nodes.US.Host, project.Domains.Entry),
				fmt.Sprintf("Write wg0.conf with address %s and listen port %d", project.Nodes.US.WGAddress, project.Nodes.US.WGPort),
				fmt.Sprintf("Deploy over SSH as %s@%s:%d via %s", project.Nodes.US.SSH.User, project.Nodes.US.Host, project.Nodes.US.SSH.Port, project.Nodes.US.SSH.AuthMethod),
			},
		},
		{
			Title: "Prepare HK exit host",
			Details: []string{
				fmt.Sprintf("Install or validate WireGuard on %s", project.Nodes.HK.Host),
				fmt.Sprintf("Install or validate %s on HK host", project.Nodes.HK.Proxy),
				fmt.Sprintf("Expose SOCKS on %s:%s for the US host only", address.Host(project.Nodes.HK.SocksListen), address.Port(project.Nodes.HK.SocksListen)),
				fmt.Sprintf("Deploy over SSH as %s@%s:%d via %s", project.Nodes.HK.SSH.User, project.Nodes.HK.Host, project.Nodes.HK.SSH.Port, project.Nodes.HK.SSH.AuthMethod),
			},
		},
		{
			Title: "Configure Cloudflare",
			Details: []string{
				fmt.Sprintf("Zone: %s", project.Cloudflare.Zone),
				fmt.Sprintf("Record type: %s, ttl: %d, proxied: %t", project.Cloudflare.RecordType, project.Cloudflare.TTL, project.Cloudflare.Proxied),
				fmt.Sprintf("Entry domain -> %s", project.Domains.Entry),
				fmt.Sprintf("Panel domain -> %s", project.Domains.Panel),
				fmt.Sprintf("WireGuard domain -> %s", project.Domains.WireGuard),
			},
		},
		{
			Title: "Validate base connectivity",
			Details: []string{
				fmt.Sprintf("US host should reach HK WG IP %s", address.CIDRIP(project.Nodes.HK.WGAddress)),
				fmt.Sprintf("US host should test HK egress via %s", project.Checks.TestURL),
			},
		},
		{
			Title: "Hand off to panel",
			Details: []string{
				fmt.Sprintf("Create SOCKS outbound tag %q in 3x-ui/x-panel", project.PanelGuide.OutboundTag),
				fmt.Sprintf("Create a dedicated panel user %q and route it to that outbound", project.PanelGuide.RouteUser),
			},
		},
	}
}

func Render(steps []Step) string {
	var b strings.Builder
	for i, step := range steps {
		fmt.Fprintf(&b, "%d. %s\n", i+1, step.Title)
		for _, detail := range step.Details {
			fmt.Fprintf(&b, "   - %s\n", detail)
		}
	}
	return b.String()
}
