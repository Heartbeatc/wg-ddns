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
| 入口节点 | 需要 root 权限，知道 IP 地址和 SSH 登录方式 |
| 出口节点 | 需要 root 权限，知道 IP 地址和 SSH 登录方式 |
| Cloudflare API Token | 在 Cloudflare 控制台创建，需要 Zone DNS 编辑权限 |
| 域名 | 至少需要面板域名和入口域名 |

SSH 支持两种登录方式：**密码** 或 **私钥文件**（如 `id_rsa.pem`）。

## 在哪台机器上运行

wgstack 是一个「管理端」工具，可以运行在：

- **本地电脑** — 通过 SSH 远程配置两台节点
- **入口节点本机** — 只需要出口节点的 SSH 信息
- **出口节点本机** — 只需要入口节点的 SSH 信息

只要运行 wgstack 的机器能通过 SSH 访问目标节点，就可以完成部署。如果你就在某台目标节点上运行，程序会跳过该节点的 SSH 配置，直接在本机部署，并自动检测本机公网 IP。

**重要：运行位置不会保存到配置文件中。** 同一份配置可以在不同机器上使用。如果你把配置带到另一台机器上运行，需要通过 CLI 参数指定运行位置：

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
2. **入口节点** — 填写 IP 和 SSH 信息（如果在本机运行则跳过 SSH）
3. **出口节点** — 填写 IP 和 SSH 信息（如果在本机运行则跳过 SSH）
4. **Cloudflare** — 主域名和 API Token
5. **域名** — 面板域名、入口域名、WG 域名（默认复用入口域名）
6. **面板与检查** — 出站标签、用户标识、出口地区代码（可选）

### 关于域名

- **面板域名**：访问 3x-ui / x-panel 管理界面的域名，通常解析到入口节点
- **入口域名**：客户端连接代理时使用的域名，通常也解析到入口节点
- **WG 域名**：出口节点通过 WireGuard 连接入口节点时使用的地址

面板域名和入口域名虽然通常指向同一台服务器，但用途不同。WG 域名在大多数场景下可以直接复用入口域名，不需要单独配置。

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

- 入口地址使用**入口域名**，**不要用出口节点的真实地址**
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
- **Cloudflare Token 同理**：把 `token` 留空，设置 `token_env` 为 `CLOUDFLARE_API_TOKEN`
- **不要把 `wgstack.json` 提交到 Git**：`.gitignore` 已经包含了它
- **优先使用私钥登录**：更安全也更方便

## 日常维护

### 入口 IP 变了

```bash
wgstack reconcile
```

自动检测新 IP、更新 DNS、刷新隧道。加 `--dry-run` 可预览变更。

### 出口 IP 变了

不需要操作。WireGuard 隧道不受影响（出口节点是主动连接方，会自动重连）。

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

- 检查 IP 地址是否正确
- 确认 SSH 端口是 22
- 检查用户名和密码/私钥是否正确
- 确认防火墙允许 SSH

### Cloudflare Token 不对

- 在 [Cloudflare 控制台](https://dash.cloudflare.com/profile/api-tokens) 检查 Token
- 需要 **Zone - DNS - Edit** 权限
- 确认 Zone 域名拼写正确

### 出口不通

- 运行 `wgstack health --live` 查看各项状态
- WireGuard 握手为 0 说明隧道未建立
- 检查防火墙是否放通了 WireGuard 端口（默认 51820）

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
