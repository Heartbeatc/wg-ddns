package guide

import (
	"fmt"
	"strings"

	"wg-ddns/internal/address"
	"wg-ddns/internal/model"
)

func Render(project model.Project) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Manual panel steps for project %q\n\n", project.Project)
	fmt.Fprintf(&b, "1. Add a SOCKS outbound in 3x-ui / x-panel\n")
	fmt.Fprintf(&b, "   - tag: %s\n", project.PanelGuide.OutboundTag)
	fmt.Fprintf(&b, "   - address: %s\n", address.Host(project.Nodes.HK.SocksListen))
	fmt.Fprintf(&b, "   - port: %s\n", address.Port(project.Nodes.HK.SocksListen))
	fmt.Fprintf(&b, "   - username/password: leave empty\n")
	fmt.Fprintf(&b, "   - mux: off\n")
	fmt.Fprintf(&b, "   - sockopt: off\n\n")

	fmt.Fprintf(&b, "2. Add a Hong Kong dedicated inbound/node in the panel\n")
	fmt.Fprintf(&b, "   - keep the public entry address as the US domain: %s\n", project.Domains.Entry)
	fmt.Fprintf(&b, "   - do not place the HK private address in any client-facing node\n")
	fmt.Fprintf(&b, "   - bind a dedicated user identifier such as: %s\n\n", project.PanelGuide.RouteUser)

	fmt.Fprintf(&b, "3. Add a routing rule\n")
	fmt.Fprintf(&b, "   - user: %s\n", project.PanelGuide.RouteUser)
	fmt.Fprintf(&b, "   - outbound tag: %s\n", project.PanelGuide.OutboundTag)
	fmt.Fprintf(&b, "   - leave unrelated match fields empty\n\n")

	fmt.Fprintf(&b, "4. Save and restart the panel/Xray service\n\n")

	fmt.Fprintf(&b, "5. Verify with the client\n")
	fmt.Fprintf(&b, "   - connect to the HK dedicated node\n")
	fmt.Fprintf(&b, "   - visit %s\n", project.Checks.TestURL)
	fmt.Fprintf(&b, "   - expected egress location: %s\n", project.Checks.ExitLocation)

	return b.String()
}
