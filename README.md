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

安装脚本会优先下载当前平台的**预编译二进制**，不要求目标机器预装 Go。默认安装 `main` 分支对应的 `edge` 预发布包。

## 更新 wgstack

已安装过的用户，直接运行：

```bash
wgstack self-update
```

程序会优先下载当前平台的**预编译二进制**并替换当前文件，不要求本机已安装 Go。

如果需要更新到指定分支或标签：

```bash
wgstack self-update --ref main
```

- `--ref main` 会拉取 `edge` 预发布包
- `--ref v0.1.0` 会拉取对应 tag 的正式 release 包

如果当前安装路径需要 root 权限（如 `/usr/local/bin`），请使用 `sudo wgstack self-update`。

## 使用方法

### 首次使用

安装完成后，直接运行：

```bash
wgstack
```

工具会自动进入**菜单式配置向导**：主菜单列出各项配置，你可按任意顺序进入、反复修改；菜单上会显示每项「已配置 / 未配置」等状态。全部确认后，再通过「查看部署摘要」或「开始部署」统一校验并写入 `wgstack.json`（仅在保存或部署时落盘，切换菜单不会丢失已填内容）。

### 配置菜单包含哪些项

1. **运行位置** — 本机是管理机、入口节点还是出口节点（决定哪些节点要填 SSH）
2. **入口节点** — 公网 IP、SSH 地址（可选）、认证信息
3. **出口节点** — 同上
4. **Cloudflare** — Zone 与 API Token
5. **域名** — 对外域名、面板域名是否分开、WG Endpoint 是否单独域名
6. **出口管理 DDNS** — 家宽动态 IP 时是否启用 SSH 管理域名自动更新
7. **入口自动修复** — 是否在入口节点启用定时自动修复
8. **面板与检查** — 出站标签、路由用户、出口地区代码（可选）
9. **逐项验证** — 可选：入口/出口 SSH 与 root、systemd；Cloudflare Token 与 Zone；域名解析与入口 IP 对比提示
10. **查看部署摘要** — 展示全文摘要，并可从摘要中跳转回任一项修改，或确认部署
11. **开始部署** — 校验通过后写入配置并执行部署
12. **保存并退出** — 校验通过后仅写入 `wgstack.json`，不部署
13. **放弃** — 不保存直接退出

`wgstack setup` 与首次无配置文件时进入的是同一套菜单式流程。

## 程序做了什么 / 没做什么

### 程序自动完成的

- 在两台节点上安装 WireGuard 和 sing-box
- 生成 WireGuard 密钥对和配置文件
- 生成 sing-box 配置文件
- 通过 SSH（或本机）下发配置到服务器
- 启动和管理 WireGuard、sing-box 服务
- 检测入口节点的公网 IP
- 同步 Cloudflare DNS 记录
- 入口 IP 变化后自动修复 DNS 和隧道（手动 reconcile 或自动定时修复）
- 在入口节点部署自动修复定时器（可选，入口 IP 变化后自动更新 DNS + 刷新出口 WG + 推送通知）
- 在出口节点部署管理 DDNS 更新器（可选，用于动态 IP 场景）

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

**如果已启用入口自动修复**（推荐），入口节点会自动检测 IP 变化并完成修复，无需手动操作。详见下方「入口自动修复」。

**手动修复**：

```bash
wgstack reconcile
```

自动检测新 IP、更新 DNS、刷新隧道。加 `--dry-run` 可预览变更。

**提示**：如果你在本地/管理机上运行且没有配置 `ssh_host` 域名，节点 IP 变化后工具自身可能连不上旧 IP。推荐给 IP 可能变化的节点配置一个稳定的 SSH 连接域名（DNS only，不走 Cloudflare 代理），详见上方「关于 SSH 连接地址」。

### 出口 IP 变了

WireGuard 隧道不受影响——出口节点是主动连接方，会自动重连。

但如果你在本地/管理机上运行 `wgstack health` 或 `wgstack apply`，且出口节点的 `host` 写的是旧公网 IP（没有配置 `ssh_host`），工具会在 SSH 阶段失联。**如果出口节点使用家宽等动态公网 IP，强烈推荐启用出口管理 DDNS**，详见下方「出口管理 DDNS」。

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

## 入口自动修复

### 这是什么

入口节点的公网 IP 可能因为换机器、重启或运营商调整而变化；Cloudflare 上的 DNS 记录也可能被手动改动或意外偏离期望。启用入口自动修复后，**入口节点上会部署一个定时任务**，持续保持入口业务域名收敛到期望状态。它能处理两类情况：

1. **入口 IP 变化** — IP 漂移后自动更新所有 DNS 记录并重启出口 WireGuard
2. **DNS 记录漂移** — 即使 IP 没变，某条记录的 content/TTL/proxied 偏离期望值或记录被删除，也会自动修复

如果 IP 没有变化且所有 DNS 记录都符合期望，定时任务会安静退出，不调用任何 API。

### 与出口管理 DDNS 的区别

| | 入口自动修复 | 出口管理 DDNS |
|---|---|---|
| 部署位置 | 入口节点 | 出口节点 |
| 维护的域名 | 入口业务域名（面板、代理入口、WG Endpoint） | 出口 SSH 管理域名 |
| 触发条件 | IP 变化 或 DNS 记录漂移 | IP 变化 |
| 触发动作 | 更新 DNS + 重启出口 WG（仅 IP 变化时）+ 通知 | 仅更新 DNS |
| 目的 | 持续保持入口业务域名收敛到期望状态 | 保持管理端能 SSH 到出口节点 |

### 如何启用

**向导方式**：首次运行 `wgstack` 时，向导第 7 步会询问是否启用入口自动修复，默认推荐开启。

**手动配置**：在 `wgstack.json` 中添加：

```json
"entry_autoreconcile": {
  "enabled": true,
  "interval_seconds": 300
}
```

### 部署了什么

启用后，`wgstack apply` 会在入口节点上安装：

| 文件 | 说明 |
|------|------|
| `/etc/wgstack/reconcile.env` | 自动修复配置（Cloudflare token、域名列表、出口 SSH 信息、Telegram） |
| `/etc/wgstack/exit_key` | 入口→出口的 SSH 私钥（自动生成的 ed25519 密钥对） |
| `/usr/local/bin/wgstack-reconcile` | 修复脚本（检测 IP + DNS 漂移 → 更新 DNS → 重启出口 WG → 通知） |
| `wgstack-reconcile.service` | systemd oneshot 服务 |
| `wgstack-reconcile.timer` | systemd 定时器（默认每 5 分钟） |

同时会在出口节点的 `~/.ssh/authorized_keys` 中添加对应的公钥，授权入口节点 SSH 连接出口节点重启 WireGuard。

配置文件 `/etc/wgstack/reconcile.env` 权限为 `0600`，其中包含 Cloudflare API token 和出口节点 SSH 信息。

### 它依赖什么

- 入口节点上的 `systemd`（定时任务）
- 入口节点上的 `curl`（检测 IP 和调用 Cloudflare API）
- 入口节点上的 `ssh`（重启出口 WG）
- 对出口节点的 SSH 管理能力（通过自动生成的密钥对）
- 正确的 Cloudflare API Token

### 自动修复失败会怎样

修复失败不会影响当前正在运行的 WireGuard 和 sing-box 服务。你可以通过 `journalctl -u wgstack-reconcile` 查看修复日志。`wgstack health --live` 也会检测自动修复定时器的状态和最近一次执行结果。

**重试逻辑**：只有当所有目标域名的 DNS 更新全部成功，并且入口 IP 变化时出口侧 WireGuard 刷新也成功完成，新 IP 才会写入状态文件。如果有任何一个域名更新失败，或者出口节点仍未解析到新的 WG Endpoint 而无法安全重启，状态文件不会更新——下次定时触发时会自动重试。

**DNS 漂移修复**：即使入口 IP 没有变化，脚本每次运行时也会检查 Cloudflare 上每条 A 记录的 content、TTL 和 proxied 是否与配置一致。如果某条记录被手动改错、被删除、或 TTL/proxied 偏离期望，脚本会自动修复。这意味着 Cloudflare 上的入口业务域名会被持续校准到期望状态。

**出口 WG 刷新时序**：当入口 IP 变化时，脚本不会在更新 Cloudflare 后立刻重启出口 WireGuard，而是会先等待出口节点自己的 DNS 解析结果看到新的 WG Endpoint IP，再执行 `systemctl restart wg-quick@wg0`。这样可以避免“DNS 刚更新但出口本地缓存还是旧 IP”导致的空重启。

**触发通知区分原因**：Telegram 通知会明确标注触发原因是「IP 变化」还是「DNS 漂移」（或两者同时），便于运维定位问题。出口 WireGuard 只在 IP 变化时重启，DNS 漂移修复不会触发 WG 重启。

## 出口管理 DDNS

### 这是什么

如果出口节点使用动态公网 IP（如家宽），IP 变化后本地管理端会失去对出口节点的 SSH 连接能力。出口管理 DDNS 解决这个问题：**在出口节点上部署一个轻量更新器**，由出口节点自己检测公网 IP 变化，自动更新 Cloudflare 上的 SSH 管理域名。

这样，无论出口 IP 如何变化，管理端始终可以通过 `ssh_host` 域名连上出口节点。

### 出口管理域名 vs 入口业务域名

| | 入口业务域名 | 出口管理域名 |
|---|---|---|
| 用途 | 客户端连接代理、访问面板、WG Endpoint | SSH 管理出口节点 |
| 更新方式 | 由管理端 `wgstack reconcile` 更新 | 由出口节点自己更新 |
| Cloudflare 代理 | 可以开启 | **必须 DNS only** |
| 客户端使用 | 是 | 否 |
| 示例 | `entry.example.com` | `ssh-exit.example.com` |

### 如何启用

**向导方式**：首次运行 `wgstack` 时，向导会询问出口 IP 是否可能变化。选择"是"后，可以启用 DDNS 并填写管理域名。

**手动配置**：在 `wgstack.json` 中添加：

```json
"exit_ddns": {
  "enabled": true,
  "domain": "ssh-exit.example.com",
  "interval_seconds": 300
}
```

同时把出口节点的 `ssh_host` 设置为**完全相同的域名**：

```json
"nodes": {
  "hk": {
    "ssh_host": "ssh-exit.example.com",
    ...
  }
}
```

**注意：** 启用出口管理 DDNS 时，`nodes.hk.ssh_host` 必须与 `exit_ddns.domain` 一致，否则配置校验会报错。这是为了确保 DDNS 维护的域名正是工具 SSH 连接使用的地址，避免"DDNS 在更新，但工具仍连错地址"的情况。向导路径会自动保持两者一致。

### 部署了什么

启用后，`wgstack apply` 会在出口节点上安装：

| 文件 | 说明 |
|------|------|
| `/etc/wgstack/ddns.env` | DDNS 配置（Cloudflare token、域名、刷新间隔） |
| `/usr/local/bin/wgstack-ddns` | 更新脚本（检测 IP → 更新 Cloudflare） |
| `wgstack-ddns.service` | systemd oneshot 服务 |
| `wgstack-ddns.timer` | systemd 定时器（默认每 5 分钟） |

配置文件 `/etc/wgstack/ddns.env` 权限为 `0600`，其中包含 Cloudflare API token。

### DDNS 更新失败会怎样

更新失败不会影响 WireGuard 和 sing-box 的运行。你可以通过 `journalctl -u wgstack-ddns` 查看更新日志。`wgstack health --live` 也会检测 DDNS 定时器状态。

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
wgstack self-update   # 更新到最新版本
```

所有命令支持 `--config path` 指定配置文件。

在目标节点本机运行时，加上 `--local-entry` 或 `--local-exit` 参数：

```bash
wgstack apply --local-entry          # 在入口节点上部署
wgstack health --live --local-exit   # 在出口节点上检查
wgstack reconcile --local-entry      # 在入口节点上同步 DNS
```
