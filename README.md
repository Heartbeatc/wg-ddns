# wgstack

帮你自动搭建「美国入口 + 香港出口」的代理底层链路。

## 这是什么

wgstack 是一个部署工具，帮你把两台 VPS 连起来：

- **美国 VPS** 作为入口机（用户连接到这里）
- **香港 VPS** 作为出口机（流量从这里出去）
- 两台机器之间用 **WireGuard** 建加密隧道
- 香港机用 **sing-box** 提供 SOCKS 代理
- **Cloudflare** 管理域名解析

你只需要提供两台 VPS 的 SSH 信息和 Cloudflare Token，工具会帮你完成底层的所有配置。

面板（3x-ui / x-panel）的设置仍然需要手动完成，但工具会一步步告诉你怎么做。

## 适合什么场景

- 你有一台美国 VPS 作为代理入口
- 你有一台香港 VPS（可以是家宽出口）
- 美国 VPS 的 IP 可能变化，需要自动同步域名
- 你用 Cloudflare 管理域名
- 你用 3x-ui 或 x-panel 做面板
- 你想让底层搭建过程自动化

## 需要准备什么

在开始之前，请准备好：

| 项目 | 说明 |
|------|------|
| 美国 VPS | 需要 root 权限，知道 IP 地址和 SSH 登录方式 |
| 香港 VPS | 需要 root 权限，知道 IP 地址和 SSH 登录方式 |
| Cloudflare API Token | 在 Cloudflare 控制台创建，需要 Zone DNS 编辑权限 |
| 三个子域名 | 入口域名、面板域名、WireGuard 域名（如 us.example.com） |

SSH 支持两种登录方式：**密码** 或 **私钥文件**（如 `id_rsa.pem`）。向导里会让你选择。

## 一键安装

在你的管理机器上运行（也可以是美国 VPS 本身）：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Heartbeatc/wg-ddns/main/scripts/install.sh)
```

## 使用方法

### 首次使用

安装完成后，直接运行：

```bash
wgstack
```

工具会自动进入部署向导，一步一步引导你完成配置和部署。

### 向导会问你什么

1. 美国 VPS 的 IP 和 SSH 登录信息（用户名、密码或私钥路径）
2. 香港 VPS 的 IP 和 SSH 登录信息
3. Cloudflare 的主域名和 API Token
4. 三个子域名（会自动建议默认值，你也可以直接回车确认）
5. 面板中要用的出站标签名和用户标识

### 向导会自动做什么

1. 生成 WireGuard 密钥对（无需手动操作）
2. 通过 SSH 连接两台 VPS
3. 自动安装 WireGuard 和 sing-box（如果未安装）
4. 生成并下发所有配置文件
5. 启动相关服务
6. 保存配置到本地 `wgstack.json`

### 部署完成后

部署完成后，向导会详细告诉你去面板里还需要做什么。具体步骤见下一节。

## 程序做了什么 / 没做什么

### 程序自动完成的

- 在两台 VPS 上安装 WireGuard 和 sing-box
- 生成 WireGuard 密钥对和配置文件
- 生成 sing-box 配置文件
- 通过 SSH 下发配置到远程服务器
- 启动和管理 WireGuard、sing-box 服务
- 检测美国入口的公网 IP
- 同步 Cloudflare DNS 记录（入口域名、面板域名、WG 域名）
- 美国 IP 变化后自动修复 DNS 和隧道

### 程序不会做的

- **不会自动配置 3x-ui / x-panel**：面板的入站、出站、路由规则需要你手动设置
- **不会修改面板数据库**：程序只管底层链路，不碰面板
- **不会自动续费或管理 VPS**：VPS 的购买和维护是你自己的事
- **不会配置防火墙规则**：如果 VPS 有额外防火墙，需要你手动放通端口
- **不会处理 TLS 证书**：面板的 HTTPS 证书配置由面板自己处理

## 面板里要做什么

底层部署完成后，你还需要在 3x-ui / x-panel 中完成以下操作：

### 1. 添加 SOCKS 出站

在面板的「出站」设置中添加：

- **标签/tag**：`hk-socks`
- **协议**：SOCKS
- **地址**：`10.66.66.2`
- **端口**：`10808`
- **用户名和密码**：留空
- **MUX**：关闭

### 2. 添加香港专用入站/节点

- 入口地址使用美国域名（如 `us.example.com`），**不要用香港的内网地址**
- 绑定一个专用用户标识（如 `hk-user@local`）

### 3. 添加路由规则

- **匹配用户**：`hk-user@local`
- **出站标签**：`hk-socks`
- 其他字段留空

### 4. 保存并重启

保存设置，重启 Xray 服务。

### 5. 验证

用客户端连接香港专用节点，访问 [ifconfig.me](https://ifconfig.me)，应该显示香港 IP。

## 安全说明

### 配置文件里有什么敏感信息

`wgstack.json` 中包含：

- SSH 密码（如果你选了密码登录）
- Cloudflare API Token
- WireGuard 私钥

### 程序做了哪些保护

- 配置文件权限设为 `0600`（只有当前用户可以读写）
- 密码输入时不会在屏幕上显示

### 你可以进一步做的

- **用环境变量代替明文密码**：在 `wgstack.json` 中，把 `password` 留空，设置 `password_env` 为环境变量名（如 `US_SSH_PASSWORD`），程序会自动读取
- **Cloudflare Token 同理**：把 `token` 留空，设置 `token_env` 为 `CLOUDFLARE_API_TOKEN`，然后 `export CLOUDFLARE_API_TOKEN=你的token`
- **不要把 `wgstack.json` 提交到 Git**：`.gitignore` 已经包含了它
- **用私钥登录代替密码**：更安全也更方便，向导里选"私钥文件"即可

## 日常维护

### 美国 IP 变了

运行以下命令，工具会自动更新 Cloudflare DNS 并刷新 WireGuard 隧道：

```bash
wgstack reconcile
```

预览变更但不执行：

```bash
wgstack reconcile --dry-run
```

### 香港出口 IP 变了

不需要操作。WireGuard 隧道不受影响，因为香港是主动连接方（会自动重连到美国的域名）。

### 检查连通性

```bash
wgstack health --live
```

会检查 DNS 解析、WireGuard 握手、SOCKS 监听、出口位置等。

### 重新部署

修改了配置或需要重新部署：

```bash
wgstack apply
```

### 打开主菜单

```bash
wgstack
```

如果已有配置文件，会显示主菜单供你选择操作。

## 常见问题

### SSH 连接失败

- 检查 IP 地址是否正确
- 确认 SSH 端口是 22（如果不是，需要在配置中修改）
- 检查用户名和密码（或私钥路径）是否正确
- 确认 VPS 防火墙允许 SSH 连接
- 如果用私钥登录，确认私钥文件路径正确且文件存在

### Cloudflare Token 不对

- 在 [Cloudflare 控制台](https://dash.cloudflare.com/profile/api-tokens) 检查 Token 是否有效
- Token 需要有 **Zone - DNS - Edit** 权限
- 确认 Zone 域名拼写正确

### 香港出口不通

- 运行 `wgstack health --live` 查看各项状态
- 如果 WireGuard 握手时间为 0，说明隧道未建立
- 检查香港 VPS 的防火墙是否放通了 WireGuard 端口（默认 51820）
- 检查 sing-box 是否正在运行

### 美国 IP 变了之后怎么恢复

运行 `wgstack reconcile`，会自动：

1. 检测美国 VPS 的新 IP
2. 更新 Cloudflare DNS 记录
3. 重启香港侧 WireGuard 以连接新地址

### 想修改配置怎么办

运行 `wgstack setup` 重新走部署向导，或直接编辑 `wgstack.json` 后运行 `wgstack apply`。

### 部署失败了怎么办

- 程序会明确告诉你哪一步失败了
- 配置文件已经保存，修复问题后运行 `wgstack apply` 重试即可
- 常见原因：SSH 密码错误、VPS 无法连接、防火墙阻止

## 高级用法

如果你熟悉命令行，也可以直接用参数模式：

```bash
wgstack init                              # 生成配置模板
wgstack plan --config wgstack.json        # 查看部署计划
wgstack render --config wgstack.json      # 生成本地配置文件
wgstack apply --config wgstack.json       # 部署到服务器
wgstack guide --config wgstack.json       # 查看面板操作说明
wgstack health --config wgstack.json --live  # 实时健康检查
wgstack reconcile --config wgstack.json   # 同步 DNS
```
