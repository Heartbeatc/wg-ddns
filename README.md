# wg-ddns

`wg-ddns` 是一个面向下面这类拓扑的命令行工具：

- 美国 VPS 作为入口机
- 香港 VPS 作为出口机
- 两端通过 WireGuard 建立隧道
- 香港机用 sing-box 提供私网 SOCKS 入站
- Cloudflare 托管入口域名、面板域名、WireGuard 域名
- 3x-ui / x-panel 保持人工配置

它解决的重点不是面板本身，而是底层这几件事：

- 生成美国入口 + 香港出口所需配置
- 通过 SSH 下发到两台 VPS
- 自动安装缺失的 WireGuard / sing-box
- 做联通性和出口检查
- 在美国入口 IP 变化后自动同步 Cloudflare
- 刷新香港侧 WireGuard，恢复入口到出口的链路

## 安装

推荐直接用一条命令安装：

```bash
bash <(curl -Ls https://raw.githubusercontent.com/Heartbeatc/wg-ddns/main/scripts/install.sh)
```

安装完成后可执行：

```bash
wgstack --help
```

默认会把二进制安装到：

- `/usr/local/bin`，如果当前用户可写
- 否则安装到 `~/.local/bin`

支持的环境变量：

- `WGDDNS_INSTALL_DIR`：指定安装目录
- `WGDDNS_REF`：指定安装的分支或 tag

## 快速开始

1. 初始化配置：

```bash
wgstack init
```

2. 修改生成的 `wgstack.json`

仓库里有两份示例可以直接参考：

- `wgstack.json.example`：私钥登录
- `wgstack.password-auth.json.example`：账密登录

3. 查看部署计划：

```bash
wgstack plan --config wgstack.json
```

4. 生成本地配置产物：

```bash
wgstack render --config wgstack.json --out build
```

5. 通过 SSH 部署到底层服务器：

```bash
wgstack apply --config wgstack.json
```

如果只想上传文件，不立刻启用服务：

```bash
wgstack apply --config wgstack.json --activate=false
```

6. 输出面板手工操作说明：

```bash
wgstack guide --config wgstack.json
```

7. 运行实时检查：

```bash
wgstack health --config wgstack.json --live
```

8. 对齐 DNS 和入口漂移：

```bash
wgstack reconcile --config wgstack.json
```

只预览变更：

```bash
wgstack reconcile --config wgstack.json --dry-run
```

## SSH 认证

每个节点都可以独立配置 SSH 登录方式。

### 私钥登录

适合 `id_rsa.pem`、`id_ed25519` 等场景：

```json
"ssh": {
  "user": "root",
  "port": 22,
  "auth_method": "private_key",
  "private_key_path": "~/.ssh/id_rsa.pem",
  "insecure_ignore_host_key": true
}
```

如果私钥有口令：

```json
"ssh": {
  "user": "root",
  "port": 22,
  "auth_method": "private_key",
  "private_key_path": "~/.ssh/id_rsa.pem",
  "passphrase_env": "US_SSH_KEY_PASSPHRASE",
  "insecure_ignore_host_key": true
}
```

### 账密登录

```json
"ssh": {
  "user": "root",
  "port": 22,
  "auth_method": "password",
  "password_env": "US_SSH_PASSWORD",
  "insecure_ignore_host_key": true
}
```

也支持直接写 `password`，但更建议使用环境变量。

## Cloudflare

当前默认策略：

- 记录类型：`A`
- TTL：`120`
- `proxied = false`

`reconcile` 会检查并同步这三个域名：

- `domains.entry`
- `domains.panel`
- `domains.wireguard`

执行成功后，会把最近一次结果写入：

```text
.wgstack-state.json
```

## apply 会做什么

`apply` 默认会执行这些动作：

- 远端 preflight，要求 root 和 `systemctl`
- 自动安装缺失的 `wireguard`、`curl`
- 香港节点自动安装缺失的 `sing-box`
- 上传 WireGuard 和 sing-box 配置
- 为旧配置自动生成时间戳备份
- 在香港节点执行 `sing-box check`
- 启用并重启对应服务

默认远端路径：

- 美国：`/etc/wireguard/wg0.conf`
- 香港：`/etc/wireguard/wg0.conf`
- 香港：`/etc/sing-box/config.json`

## 面板边界

`wg-ddns` 不会直接改 3x-ui / x-panel 数据。

底层部署完成后，`wgstack guide` 会告诉你接下来该在面板中做什么：

- 新建 `hk-socks` 出站
- 新建香港专用节点
- 新建 `User -> hk-socks` 路由规则
- 保存并重启 Xray

## 当前支持范围

当前版本主要面向这一个标准场景：

- 1 台美国入口机
- 1 台香港出口机
- WireGuard
- sing-box
- Cloudflare
- 手工面板介入

## 项目内示例

- `wgstack.json.example`
- `wgstack.password-auth.json.example`
- `美国DMIT入口+香港VPS出口（WireGuard + 3x-ui + Xray-sing-box）配置手册.md`
