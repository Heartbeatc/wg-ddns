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
	exitDetail := fmt.Sprintf("通过出口 SOCKS 访问 %s 应显示 %s", project.Checks.TestURL, project.Checks.ExitLocation)
	if project.Checks.ExitLocation == "" {
		exitDetail = fmt.Sprintf("通过出口 SOCKS 访问 %s（不做地区校验）", project.Checks.TestURL)
	}

	return []Check{
		{
			Name:   "DNS",
			Detail: fmt.Sprintf("%s 应解析到入口节点公网 IP", strings.Join(project.Domains.Unique(), "、")),
		},
		{
			Name:   "WireGuard",
			Detail: fmt.Sprintf("入口 %s 和出口 %s 应有最近的握手记录", address.CIDRIP(project.Nodes.US.WGAddress), address.CIDRIP(project.Nodes.HK.WGAddress)),
		},
		{
			Name:   "出口 SOCKS",
			Detail: fmt.Sprintf("出口节点应监听 %s", project.Nodes.HK.SocksListen),
		},
		{
			Name:   "出口验证",
			Detail: exitDetail,
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
