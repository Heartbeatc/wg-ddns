# wgstack

帮你搭建「入口节点 + 出口节点」的代理底层链路。

## 这是什么

wgstack 是一个部署工具，帮你把两台服务器连成一条代理链路：

- **入口节点** — 接收客户端连接的服务器
- **出口节点** — 流量最终从此出去的服务器
- 两台机器之间用 **WireGuard** 建加密隧道
- 出口节点用 **sing-box** 提供 SOCKS 代理
- **Cloudflare** 管理域名解析

你只需要提供服务器的 SSH 信息和 Cloudflare Token，工具会帮你完成底层的所有配置。

面板（3x-ui / x-panel）的设置仍然需要手动完成，但工具会一步步告诉你怎么做。

典型场景举例：美国 VPS 做入口 + 香港 VPS 做出口，但本工具不限于特定地区组合。

## 适合什么场景

- 你有两台服务器，一台做入口，一台做出口
- 入口节点的 IP 可能变化，需要自动同步域名
- 你用 Cloudflare 管理域名
- 你用 3x-ui 或 x-panel 做面板
- 你想让底层搭建过程自动化

## 需要准备什么

| 项目 | 说明 |
|------|------|
| 入口节点 | 需要 root 权限，知道公网 IP 和 SSH 登录方式 |
| 出口节点 | 需要 root 权限，知道公网 IP 和 SSH 登录方式 |
| Cloudflare API Token | 在 Cloudflare 控制台创建，需要 Zone DNS 编辑权限 |
| 域名 | 一个对外域名即可（面板和代理入口通常共用同一个） |

SSH 支持两种登录方式：**密码** 或 **私钥文件**（如 `id_rsa.pem`）。

### 关于 SSH 连接地址

如果你在**本地电脑或管理机**上运行 wgstack，工具需要通过 SSH 连接到目标节点。默认情况下，SSH 使用你填写的节点公网 IP。

**如果节点的公网 IP 可能变化，推荐配置一个稳定的 SSH 连接域名**，这样即使 IP 漂移，wgstack 仍然能通过域名连上节点执行 deploy、health、reconcile 等操作。常见的动态 IP 场景：

- **入口节点**：换机器、换 IP 后，旧 IP 立即失效
- **出口节点**：使用家宽或动态公网 IP 的 VPS，IP 可能随时变化

配置方法：

- 在 `wgstack.json` 的节点配置中设置 `"ssh_host"`，例如：
  - 入口节点：`"ssh_host": "ssh.entry.example.com"`
  - 出口节点：`"ssh_host": "ssh.exit.example.com"`
- 这个域名必须**直连目标服务器**，不应走 Cloudflare 代理（Proxy 关闭，即 DNS only）
- 它可以与对外代理域名不同
- 向导中也会引导你填写

如果你直接在某台目标节点本机上运行 wgstack，则该节点不需要 SSH——程序会直接在本机操作。

## 关于域名

大多数情况下，你只需要准备一个对外域名。面板访问和客户端连接代理用的是同一个域名，只是通过不同端口分流：

- `example.com:面板端口` — 访问 3x-ui / x-panel 管理界面
- `example.com:443` — 客户端连接代理

**域名相同是正常的，不需要专门准备多个不同的子域名。**

如果你确实希望把面板和代理入口分开（比如 `panel.example.com` 访问面板、`us.example.com` 连接代理），向导里也支持分别填写。

WireGuard Endpoint 是出口节点通过隧道连接入口节点时使用的域名，默认直接复用你的对外域名，不需要单独准备。如果单独配置，也应填写域名——本工具会将其纳入 Cloudflare DNS 同步，不支持直接填写 IP 地址。

## 在哪台机器上运行

wgstack 是一个「管理端」工具，可以运行在：

- **本地电脑** — 通过 SSH 远程配置两台节点
- **入口节点本机** — 只需要出口节点的 SSH 信息
- **出口节点本机** — 只需要入口节点的 SSH 信息

只要运行 wgstack 的机器能通过 SSH 访问目标节点，就可以完成部署。如果你就在某台目标节点上运行，程序会跳过该节点的 SSH 配置，直接在本机部署，并自动检测本机公网 IP。

**重要：运行位置不会保存到配置文件中。** 同一份配置可以在不同机器上使用。主菜单在执行部署、检查、DNS 同步等操作前会重新确认当前运行位置。如果使用 CLI 命令，需要通过参数指定运行位置：

```bash
wgstack apply --local-entry    # 在入口节点本机运行
wgstack apply --local-exit     # 在出口节点本机运行
```

`health --live` 和 `reconcile` 同样支持 `--local-entry` / `--local-exit` 参数。

## 一键安装

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

### 向导流程

1. **选择运行位置** — 你在哪台机器上运行（决定需要哪些 SSH 信息）
2. **入口节点** — 填写公网 IP、SSH 连接地址（可选，推荐域名）和 SSH 信息
3. **出口节点** — 填写公网 IP、SSH 连接地址（可选）和 SSH 信息
4. **Cloudflare** — 主域名和 API Token
5. **域名** — 输入你的对外域名，确认面板和 WG Endpoint 是否复用（默认都复用）
6. **面板与检查** — 出站标签、用户标识、出口地区代码（可选）

## 程序做了什么 / 没做什么

### 程序自动完成的

- 在两台节点上安装 WireGuard 和 sing-box
- 生成 WireGuard 密钥对和配置文件
- 生成 sing-box 配置文件
- 通过 SSH（或本机）下发配置到服务器
- 启动和管理 WireGuard、sing-box 服务
- 检测入口节点的公网 IP
- 同步 Cloudflare DNS 记录
- 入口 IP 变化后自动修复 DNS 和隧道

### 程序不会做的

- **不会自动配置 3x-ui / x-panel**：面板的入站、出站、路由规则需要你手动设置
- **不会修改面板数据库**：程序只管底层链路
- **不会自动续费或管理 VPS**
- **不会配置防火墙规则**：如需放通端口，请手动操作
- **不会处理 TLS 证书**

## 面板里要做什么

底层部署完成后，你还需要在 3x-ui / x-panel 中完成以下操作：

### 1. 添加 SOCKS 出站

在面板的「出站」设置中添加：

- **标签/tag**：`exit-socks`（或你在向导里填的标签）
- **协议**：SOCKS
- **地址**：`10.66.66.2`（出口节点的 WireGuard 内网地址）
- **端口**：`10808`
- **用户名和密码**：留空
- **MUX**：关闭

### 2. 添加专用线路入站/节点

- 节点地址使用你的对外域名（就是面板访问和代理入口共用的那个域名）
- 如果面板和代理入口使用同一个域名，这是正常的，区分用途的是端口和入站配置
- 不要填写出口节点的真实地址
- 绑定一个专用用户标识（如 `exit-user@local`）

### 3. 添加路由规则

- **匹配用户**：填你在上一步绑定的用户标识
- **出站标签**：填 SOCKS 出站的标签
- 其他字段留空

### 4. 保存并重启

保存设置，重启 Xray 服务。

### 5. 验证

用客户端连接专用线路节点，访问 [ifconfig.me](https://ifconfig.me)，确认出口 IP 符合预期。

## 安全说明

### 配置文件里有什么敏感信息

`wgstack.json` 中包含 SSH 密码（如果选了密码登录）、Cloudflare API Token 和 WireGuard 私钥。

### 程序做了哪些保护

- 配置文件权限设为 `0600`（只有当前用户可以读写）
- 密码输入时不会在屏幕上显示

### 你可以进一步做的

- **用环境变量代替明文密码**：在 `wgstack.json` 中把 `password` 留空，设置 `password_env` 为环境变量名
- **Cloudflare Token 推荐用环境变量**：设置 `token_env` 为 `CLOUDFLARE_API_TOKEN`，然后通过 `export CLOUDFLARE_API_TOKEN=xxx` 注入。**如果环境变量和配置文件中都有 token，环境变量优先**——这意味着你可以随时通过环境变量覆盖旧 token，无需修改配置文件
- **不要把 `wgstack.json` 提交到 Git**：`.gitignore` 已经包含了它
- **优先使用私钥登录**：更安全也更方便

## Telegram 运维通知

wgstack 支持通过 Telegram Bot 推送运维通知。启用后，以下事件会自动推送到你的 Telegram 聊天：

- 部署成功 / 失败
- 入口节点 IP 发生变化（含 DNS 更新详情和 IP 归属信息）
- reconcile 失败
- 健康检查发现故障

通知失败不会影响主流程——如果 Telegram 不可达或配置有误，程序会正常继续。

### 如何配置

1. 在 Telegram 中找到 [@BotFather](https://t.me/BotFather)，发送 `/newbot`，按提示创建 Bot，拿到 Bot Token
2. 把 Bot 加入你想接收通知的群组，或直接给 Bot 发一条消息
3. 获取 Chat ID：访问 `https://api.telegram.org/bot<你的Token>/getUpdates`，在返回的 JSON 中找到 `chat.id`
4. 在 `wgstack.json` 中填写：

```json
"notifications": {
  "enabled": true,
  "telegram": {
    "bot_token": "123456:ABC-DEF...",
    "chat_id": "-100123456789"
  }
}
```

Bot Token 支持环境变量注入：把 `bot_token` 留空，设置 `bot_token_env` 为环境变量名（默认 `TELEGRAM_BOT_TOKEN`），然后通过环境变量传入。

### 通知内容

- **IP 变化通知**：包含新 IP、DNS 更新记录、自动查询的 IP 归属摘要（国家/城市/ISP，数据来自 ipinfo.io HTTPS API）、iplark 详细质量查询外链（`https://iplark.com/{ip}`，便于人工查看 IP 纯净度）、各探测项延迟
- **部署通知**：包含入口/出口节点地址
- **健康检查通知**：列出所有失败和通过的探测项及耗时；如果健康检查本身执行失败（如 SSH 连接不上），也会推送错误通知
- **reconcile 失败通知**：包含错误信息

## 日常维护

### 入口 IP 变了

```bash
wgstack reconcile
```

自动检测新 IP、更新 DNS、刷新隧道。加 `--dry-run` 可预览变更。

**提示**：如果你在本地/管理机上运行且没有配置 `ssh_host` 域名，节点 IP 变化后工具自身可能连不上旧 IP。推荐给 IP 可能变化的节点配置一个稳定的 SSH 连接域名（DNS only，不走 Cloudflare 代理），详见上方「关于 SSH 连接地址」。

### 出口 IP 变了

WireGuard 隧道不受影响——出口节点是主动连接方，会自动重连。

但如果你在本地/管理机上运行 `wgstack health` 或 `wgstack apply`，且出口节点的 `host` 写的是旧公网 IP（没有配置 `ssh_host`），工具会在 SSH 阶段失联。**如果出口节点使用家宽等动态公网 IP，强烈推荐配置 `ssh_host` 域名**，详见上方「关于 SSH 连接地址」。

### 检查连通性

```bash
wgstack health --live
```

### 重新部署

```bash
wgstack apply
```

### 打开主菜单

```bash
wgstack
```

## 常见问题

### SSH 连接失败

- 检查 SSH 连接地址是否正确（如果配置了 `ssh_host`，检查该域名是否解析到当前节点 IP）
- 如果节点 IP 刚变过（入口换 IP 或出口动态 IP 漂移），且没有配置 `ssh_host` 域名，需要先更新配置文件中的 `host`
- 确认 SSH 端口是 22
- 检查用户名和密码/私钥是否正确
- 确认防火墙允许 SSH

### Cloudflare Token 不对

- 环境变量优先于配置文件：如果设置了 `token_env` 对应的环境变量，它会覆盖 `token` 字段
- 如果 `curl` 能成功但 wgstack 报 `Invalid access token`，检查配置文件中是否残留了旧的 `token` 值（环境变量应该会覆盖它，但确认 `token_env` 是否正确填写）
- 在 [Cloudflare 控制台](https://dash.cloudflare.com/profile/api-tokens) 检查 Token
- 需要 **Zone - DNS - Edit** 权限
- 确认 Zone 域名拼写正确

### 出口不通

- 运行 `wgstack health --live` 查看各项状态
- WireGuard 握手为 0 说明隧道未建立
- 检查防火墙是否放通了 WireGuard 端口（默认 51820）
- 如果出口验证报「返回的不是合法公网 IPv4」，检查 `healthcheck.exit_check_url`——该字段必须指向一个返回纯文本公网 IP 的接口（如 `https://api.ipify.org`），不能是返回 HTML、JSON 或国家代码的地址

### 部署失败了

- 程序会明确告诉你哪一步失败
- 配置已保存，修复后运行 `wgstack apply` 重试

## 高级用法

```bash
wgstack init          # 生成配置模板
wgstack plan          # 查看部署计划
wgstack render        # 生成本地配置文件
wgstack apply         # 部署到服务器
wgstack guide         # 查看面板操作说明
wgstack health --live # 实时健康检查
wgstack reconcile     # 同步 DNS
```

所有命令支持 `--config path` 指定配置文件。

在目标节点本机运行时，加上 `--local-entry` 或 `--local-exit` 参数：

```bash
wgstack apply --local-entry          # 在入口节点上部署
wgstack health --live --local-exit   # 在出口节点上检查
wgstack reconcile --local-entry      # 在入口节点上同步 DNS
```
