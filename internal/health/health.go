package health

import (
	"fmt"
	"strings"

	"wg-ddns/internal/address"
	"wg-ddns/internal/model"
)

type Check struct {
	Name   string
	Detail string
}

func Expected(project model.Project) []Check {
	return []Check{
		{
			Name:   "DNS",
			Detail: fmt.Sprintf("%s, %s, and %s should resolve to the current US public IP", project.Domains.Entry, project.Domains.Panel, project.Domains.WireGuard),
		},
		{
			Name:   "WireGuard",
			Detail: fmt.Sprintf("US %s and HK %s should have a recent handshake", address.CIDRIP(project.Nodes.US.WGAddress), address.CIDRIP(project.Nodes.HK.WGAddress)),
		},
		{
			Name:   "HK SOCKS",
			Detail: fmt.Sprintf("HK host should listen on %s", project.Nodes.HK.SocksListen),
		},
		{
			Name:   "Egress",
			Detail: fmt.Sprintf("US host curl via HK SOCKS to %s should exit from %s", project.Checks.TestURL, project.Checks.ExitLocation),
		},
	}
}

func Render(checks []Check) string {
	var b strings.Builder
	for _, check := range checks {
		fmt.Fprintf(&b, "- %s: %s\n", check.Name, check.Detail)
	}
	return b.String()
}
