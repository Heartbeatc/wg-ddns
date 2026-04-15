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
			Title: "配置入口节点",
			Details: []string{
				fmt.Sprintf("在 %s 上安装或验证 WireGuard", project.Nodes.US.Host),
				fmt.Sprintf("写入 wg0.conf，地址 %s，监听端口 %d", project.Nodes.US.WGAddress, project.Nodes.US.WGPort),
				fmt.Sprintf("SSH: %s@%s:%d", project.Nodes.US.SSH.User, project.Nodes.US.SSHAddr(), project.Nodes.US.SSH.Port),
			},
		},
		{
			Title: "配置出口节点",
			Details: []string{
				fmt.Sprintf("在 %s 上安装或验证 WireGuard", project.Nodes.HK.Host),
				fmt.Sprintf("安装或验证 %s", project.Nodes.HK.Proxy),
				fmt.Sprintf("SOCKS 监听 %s:%s（仅入口节点可达）", address.Host(project.Nodes.HK.SocksListen), address.Port(project.Nodes.HK.SocksListen)),
				fmt.Sprintf("SSH: %s@%s:%d", project.Nodes.HK.SSH.User, project.Nodes.HK.SSHAddr(), project.Nodes.HK.SSH.Port),
			},
		},
		{
			Title: "配置 Cloudflare",
			Details: []string{
				fmt.Sprintf("Zone: %s", project.Cloudflare.Zone),
				fmt.Sprintf("记录类型: %s，TTL: %d，Proxied: %t", project.Cloudflare.RecordType, project.Cloudflare.TTL, project.Cloudflare.Proxied),
				fmt.Sprintf("域名: %s", strings.Join(project.Domains.Unique(), ", ")),
			},
		},
		{
			Title: "验证基础连通",
			Details: []string{
				fmt.Sprintf("入口节点应能 ping 出口 WG 地址 %s", address.CIDRIP(project.Nodes.HK.WGAddress)),
				egressPlanDetail(project),
			},
		},
		{
			Title: "面板配置（人工）",
			Details: []string{
				fmt.Sprintf("在 3x-ui/x-panel 中创建 SOCKS 出站 tag %q", project.PanelGuide.OutboundTag),
				fmt.Sprintf("创建专用线路用户 %q 并路由到该出站", project.PanelGuide.RouteUser),
			},
		},
	}
}

func egressPlanDetail(project model.Project) string {
	if project.Checks.ExitLocation != "" {
		return fmt.Sprintf("通过出口 SOCKS 访问 %s 应从 %s 出", project.Checks.TestURL, project.Checks.ExitLocation)
	}
	return fmt.Sprintf("通过出口 SOCKS 访问 %s 验证连通性", project.Checks.TestURL)
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
