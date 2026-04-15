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

	fmt.Fprintln(&b, "2. 添加专用线路入站/节点")
	fmt.Fprintf(&b, "   - 节点地址使用你的对外域名: %s\n", project.Domains.Entry)
	if project.Domains.Panel == project.Domains.Entry {
		fmt.Fprintln(&b, "   - 面板和代理入口共用同一个域名是正常的，真正区分用途的是端口")
	}
	fmt.Fprintln(&b, "   - 不要填写出口节点的真实地址")
	fmt.Fprintf(&b, "   - 绑定专用用户标识:     %s\n", project.PanelGuide.RouteUser)
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "3. 添加路由规则")
	fmt.Fprintf(&b, "   - 匹配用户:   %s\n", project.PanelGuide.RouteUser)
	fmt.Fprintf(&b, "   - 出站标签:   %s\n", project.PanelGuide.OutboundTag)
	fmt.Fprintln(&b, "   - 其他字段留空")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "4. 保存并重启 Xray 服务")
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "5. 验证")
	fmt.Fprintln(&b, "   - 用客户端连接专用线路节点")
	fmt.Fprintf(&b, "   - 访问 %s\n", project.Checks.TestURL)
	if project.Checks.ExitLocation != "" {
		fmt.Fprintf(&b, "   - 应该显示出口位置: %s\n", project.Checks.ExitLocation)
	} else {
		fmt.Fprintln(&b, "   - 确认出口 IP 符合预期")
	}
	fmt.Fprintln(&b)

	fmt.Fprintln(&b, "提示：")
	fmt.Fprintln(&b, "   - 如果入口节点 IP 发生变化，运行 wgstack reconcile 即可自动修复")
	fmt.Fprintln(&b, "   - 出口节点 IP 变化不影响隧道（出口是主动连接方，会自动重连）")

	return b.String()
}
