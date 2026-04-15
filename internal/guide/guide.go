package guide

import (
	"fmt"
	"strings"

	"wg-ddns/internal/address"
	"wg-ddns/internal/model"
)

func Render(project model.Project) string {
	var b strings.Builder

	fmt.Fprintln(&b, "你现在需要去 3x-ui / x-panel 面板中完成以下操作：")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "1. 添加 SOCKS 出站")
	fmt.Fprintln(&b, "   打开面板 → 出站设置 → 添加出站")
	fmt.Fprintf(&b, "   - 标签/tag:     %s\n", project.PanelGuide.OutboundTag)
	fmt.Fprintln(&b, "   - 协议:         SOCKS")
	fmt.Fprintf(&b, "   - 地址:         %s\n", address.Host(project.Nodes.HK.SocksListen))
	fmt.Fprintf(&b, "   - 端口:         %s\n", address.Port(project.Nodes.HK.SocksListen))
	fmt.Fprintln(&b, "   - 用户名/密码:  留空")
	fmt.Fprintln(&b, "   - MUX:          关闭")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "2. 添加香港专用入站/节点")
	fmt.Fprintf(&b, "   - 入口地址用美国域名: %s\n", project.Domains.Entry)
	fmt.Fprintln(&b, "   - 不要用香港内网地址")
	fmt.Fprintf(&b, "   - 绑定专用用户标识:   %s\n", project.PanelGuide.RouteUser)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "3. 添加路由规则")
	fmt.Fprintf(&b, "   - 匹配用户:   %s\n", project.PanelGuide.RouteUser)
	fmt.Fprintf(&b, "   - 出站标签:   %s\n", project.PanelGuide.OutboundTag)
	fmt.Fprintln(&b, "   - 其他字段留空")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "4. 保存并重启 Xray 服务")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "5. 验证")
	fmt.Fprintln(&b, "   - 用客户端连接香港专用节点")
	fmt.Fprintf(&b, "   - 访问 %s\n", project.Checks.TestURL)
	fmt.Fprintf(&b, "   - 应该显示出口位置: %s\n", project.Checks.ExitLocation)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "提示：")
	fmt.Fprintln(&b, "   - 如果美国入口 IP 发生变化，运行 wgstack reconcile 即可自动修复")
	fmt.Fprintln(&b, "   - 如果香港出口 IP 变化，WireGuard 隧道不受影响（香港是主动连接方）")

	return b.String()
}
