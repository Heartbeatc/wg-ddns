# wg-ddns

`wg-ddns` 是一个面向下面这类场景的轻量 CLI：

- 美国 VPS 作为入口机
- 香港 VPS 作为出口机
- 两端通过 WireGuard 建隧道
- 香港机使用 sing-box 提供私网 SOCKS 入站
- Cloudflare 托管入口域名、面板域名、WireGuard 域名
- 3x-ui / x-panel 继续人工配置

当前版本先完成第一阶段骨架：

- 项目配置模型
- 本地渲染 WireGuard / sing-box 配置
- 部署计划输出
- 通过 SSH 下发配置并重启服务
- 面板人工操作指引输出
- 基础健康检查项输出
- Cloudflare DNS 对齐
- 入口 IP 漂移后的 reconcile

## 为什么先做 CLI

这个项目的核心难点不是界面，而是：

- 配置模型是否稳定
- 两台 VPS 的角色边界是否清楚
- 入口 IP 漂移时的收敛逻辑是否正确
- 最终交付给用户的操作指引是否足够明确

所以第一阶段先做 CLI，后面再加 SSH 部署、Cloudflare 更新、reconcile、自愈逻辑。

## 当前命令

```bash
go run ./cmd/wgstack init
go run ./cmd/wgstack plan --config wgstack.json
go run ./cmd/wgstack render --config wgstack.json --out build
go run ./cmd/wgstack apply --config wgstack.json
go run ./cmd/wgstack guide --config wgstack.json
go run ./cmd/wgstack health --config wgstack.json
go run ./cmd/wgstack health --config wgstack.json --live
go run ./cmd/wgstack reconcile --config wgstack.json
```

## 推荐使用流程

1. 初始化示例配置

```bash
go run ./cmd/wgstack init
```

2. 修改生成的 `wgstack.json`

至少替换这些字段：

- `domains.entry`
- `domains.panel`
- `domains.wireguard`
- `nodes.us.host`
- `nodes.hk.host`
- `panel_guide.outbound_tag`
- `panel_guide.route_user`

仓库里也提供了两份可直接参考的示例：

- `wgstack.json.example`：私钥登录
- `wgstack.password-auth.json.example`：账密登录

3. 查看部署计划

```bash
go run ./cmd/wgstack plan --config wgstack.json
```

4. 本地渲染配置产物

```bash
go run ./cmd/wgstack render --config wgstack.json --out build
```

产物包括：

- `build/out/us/wg0.conf`
- `build/out/hk/wg0.conf`
- `build/out/hk/sing-box.json`

5. 输出面板人工操作指南

```bash
go run ./cmd/wgstack guide --config wgstack.json
```

6. 通过 SSH 下发配置

```bash
go run ./cmd/wgstack apply --config wgstack.json
```

如果你只想上传文件，不立刻启用/重启服务：

```bash
go run ./cmd/wgstack apply --config wgstack.json --activate=false
```

7. 运行实时探测

```bash
go run ./cmd/wgstack health --config wgstack.json --live
```

8. 对齐 Cloudflare 和入口漂移

```bash
go run ./cmd/wgstack reconcile --config wgstack.json
```

如果只想看将发生什么：

```bash
go run ./cmd/wgstack reconcile --config wgstack.json --dry-run
```

## SSH 认证

每个节点独立配置 SSH。

### 私钥登录

适合 `id_rsa.pem`、`id_ed25519` 这类场景：

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

也支持直接把 `password` 写进配置，但开源项目里更推荐 `password_env`。

## 当前 apply 行为

`apply` 现在会做这些事：

- 远端 preflight：要求 root、要求 `systemctl`
- 默认自动安装缺失的 `wireguard`、`curl`
- 香港节点默认自动安装缺失的 `sing-box`
- 连接美国和香港节点
- 上传美国 `wg0.conf`
- 上传香港 `wg0.conf`
- 上传香港 `sing-box` 配置
- 为远端旧文件自动留一份时间戳备份
- 上传后先做 `sing-box check`
- 执行 `systemctl enable --now` 和 `systemctl restart`

默认远端目标路径：

- 美国：`/etc/wireguard/wg0.conf`
- 香港：`/etc/wireguard/wg0.conf`
- 香港：`/etc/sing-box/config.json`

## 当前 reconcile 行为

`reconcile` 现在会做这些事：

- SSH 登录美国入口机，探测当前公网 IPv4
- 检查 Cloudflare 中的入口域名、面板域名、WireGuard 域名
- 缺失则创建，漂移则更新
- 如果 DNS 发生变化，重启香港侧的 WireGuard 服务，让 endpoint 重新解析
- 最后跑一轮 live health probes
- 成功后把最近一次 reconcile 结果写到 `.wgstack-state.json`

默认的 Cloudflare 策略是：

- `record_type = "A"`
- `ttl = 120`
- `proxied = false`

## 当前边界

当前版本还没有实现：

- 更细粒度的 WireGuard peer endpoint 定向刷新
- Cloudflare 之外的 DNS 提供商
- 基于本地 daemon 的定时 reconcile
- 面板 API 级自动化

这些会是下一阶段的开发重点。
