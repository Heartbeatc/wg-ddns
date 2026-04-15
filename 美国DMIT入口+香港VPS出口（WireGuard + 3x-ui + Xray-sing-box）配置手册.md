# 美国 DMIT 入口 + 香港 VPS 出口配置手册

## 1. 目标

本文记录一套已经跑通的方案：

- 客户端连接美国 DMIT VPS 上现有的 `3x-ui` 节点
- 美国 VPS 保持为入口机，不破坏现有环境
- 香港 VPS 作为最终出口机
- 美国与香港之间通过 `WireGuard` 建立内网隧道
- 美国 Xray 按指定节点对应的用户，将流量转发到香港 VPS
- 最终对外显示为香港 IP

适用场景：

- 美国入口速度好
- 香港出口 IP 质量高
- 希望通过不同节点链接切换不同出口

---

## 2. 最终拓扑

```text
客户端
  -> 美国 VPS 公网节点（3x-ui 入站）
  -> 美国 Xray 路由命中
  -> 美国 Xray 出站 hk-socks
  -> WireGuard 内网 10.66.66.2:10808
  -> 香港 VPS 上的 Xray/sing-box 入站
  -> 香港公网出口
```

关键原则：

- 客户端永远连接美国 VPS 的公网地址或域名
- `10.66.66.2` 只是美港隧道内的内网地址，不能出现在订阅节点中

---

## 3. 机器分工

### 美国 VPS

- 保留现有 `3x-ui`
- 作为客户端连接入口
- 运行 `WireGuard`
- 新增一个 `socks` 出站，指向香港 VPS 的隧道内地址
- 通过 Xray 路由规则，把某个节点用户的流量送到香港

### 香港 VPS

- 运行 `WireGuard`
- 运行 `Xray` 或 `sing-box`
- 在 `10.66.66.2:10808` 提供一个仅供美国 VPS 使用的 `socks` 入站
- 收到流量后直接从香港公网出口发出

---

## 4. 地址规划

本文用下面这组地址举例：

- 美国 VPS WireGuard 地址：`10.66.66.1`
- 香港 VPS WireGuard 地址：`10.66.66.2`
- 香港 VPS socks 入站：`10.66.66.2:10808`
- 美国 VPS WireGuard 监听端口：`51820/udp`

---

## 5. 美国 VPS 配置

### 5.1 安装 WireGuard

```bash
apt update
apt install -y wireguard
```

### 5.2 生成密钥

```bash
umask 077
wg genkey | tee /etc/wireguard/us.key | wg pubkey > /etc/wireguard/us.pub
cat /etc/wireguard/us.pub
```

### 5.3 写入 WireGuard 配置

美国 VPS 的 `/etc/wireguard/wg0.conf` 示例：

```ini
[Interface]
Address = 10.66.66.1/24
ListenPort = 51820
PrivateKey = 美国VPS私钥

[Peer]
PublicKey = 香港VPS公钥
AllowedIPs = 10.66.66.2/32
```

### 5.4 启动 WireGuard

```bash
systemctl enable wg-quick@wg0 --now
wg show
```

如果启用了防火墙，放行：

```bash
ufw allow 51820/udp
```

---

## 6. 香港 VPS 配置

### 6.1 安装 WireGuard

```bash
apt update
apt install -y wireguard
```

### 6.2 安装 Xray 或 sing-box

二选一即可。

如果使用 `sing-box`：

```bash
bash <(curl -fsSL https://sing-box.app/install.sh)
sing-box version
```

### 6.3 生成密钥

```bash
umask 077
wg genkey | tee /etc/wireguard/hk.key | wg pubkey > /etc/wireguard/hk.pub
cat /etc/wireguard/hk.pub
```

### 6.4 写入 WireGuard 配置

香港 VPS 的 `/etc/wireguard/wg0.conf` 示例：

```ini
[Interface]
Address = 10.66.66.2/24
PrivateKey = 香港VPS私钥
MTU = 1420

[Peer]
PublicKey = 美国VPS公钥
Endpoint = 美国VPS公网IP:51820
AllowedIPs = 10.66.66.1/32
PersistentKeepalive = 25
```

### 6.5 开启转发和 BBR

编辑 `/etc/sysctl.conf`，加入：

```conf
net.ipv4.ip_forward=1
net.ipv6.conf.all.forwarding=1
net.core.default_qdisc=fq
net.ipv4.tcp_congestion_control=bbr
```

应用配置：

```bash
sysctl -p
```

### 6.6 启动 WireGuard

```bash
systemctl enable wg-quick@wg0 --now
wg show
ip a show wg0
```

---

## 7. 香港 VPS 上的 Xray/sing-box 入站

这里的目标是：

- 只监听 `10.66.66.2`
- 只服务美国 VPS
- 不暴露公网

### 7.1 使用 sing-box 的最小配置

文件：`/etc/sing-box/config.json`

```json
{
  "log": {
    "level": "info"
  },
  "inbounds": [
    {
      "type": "socks",
      "tag": "socks-in",
      "listen": "10.66.66.2",
      "listen_port": 10808,
      "users": []
    }
  ],
  "outbounds": [
    {
      "type": "direct",
      "tag": "direct"
    }
  ],
  "route": {
    "final": "direct"
  }
}
```

校验并启动：

```bash
sing-box check -c /etc/sing-box/config.json
systemctl enable sing-box --now
systemctl restart sing-box
systemctl status sing-box
```

确认监听：

```bash
ss -lntp | grep 10808
```

应当看到监听在：

```text
10.66.66.2:10808
```

### 7.2 如果使用 Xray

思路完全相同，只需要提供一个监听在 `10.66.66.2:10808` 的 `socks` 入站即可。

---

## 8. 先做底层联通验证

在美国 VPS 上测试：

```bash
ping 10.66.66.2
curl --socks5-hostname 10.66.66.2:10808 https://ifconfig.me
```

如果第二条命令返回的是香港 IP，说明下面三件事都已经正确：

- 美港 WireGuard 通了
- 香港 socks 入站通了
- 香港公网出口通了

这一步必须先成功，再去改 `3x-ui`。

---

## 9. 美国 3x-ui 中新增香港出站

进入美国 VPS 的 `Xray设置` -> `出站规则`，新增一个出站：

- 协议：`Socks`
- 标签：`hk-socks`
- 发送通过：留空
- 地址：`10.66.66.2`
- 端口：`10808`
- 用户名：留空
- 密码：留空
- Sockopts：关闭
- Mux：关闭

对应 JSON 大致如下：

```json
{
  "protocol": "socks",
  "settings": {
    "servers": [
      {
        "address": "10.66.66.2",
        "port": 10808,
        "users": []
      }
    ]
  },
  "tag": "hk-socks"
}
```

---

## 10. 美国 3x-ui 中新增香港专用节点

新增一个普通的美国公网入站，不要把香港内网地址填进入站配置。

### 10.1 入站建议参数

- 协议：`VLESS`
- 传输：`TCP (RAW)`
- 安全：`Reality`
- `uTLS`：`chrome`
- `Target`：如 `www.microsoft.com:443`
- `SNI`：如 `www.microsoft.com`
- `SpiderX`：`/`

### 10.2 不建议开启

- `External Proxy`
- `Proxy Protocol`
- `HTTP 伪装`
- `Sockopt`

### 10.3 版本限制

下面两项建议留空：

- `Min Client Ver`
- `Max Client Ver`

否则容易因为客户端版本不匹配导致连接失败。

### 10.4 客户端标识

这个节点下只放一个明确的客户端用户，便于做路由识别。

推荐做法：

- 客户端邮箱或用户字段设置为固定值，例如 `hk-test@local`

如果面板显示的是自动生成的用户标识，并且路由规则能识别，也可以直接使用该值。

---

## 11. 美国 3x-ui 中新增路由规则

进入 `Xray设置` -> `路由规则`，新增一条规则：

- `User`：填香港专用节点对应的客户端用户
- `Outbound Tag`：选择 `hk-socks`

其他字段全部留空：

- `Source IPs`
- `Source Port`
- `VLESS Route`
- `Network`
- `Protocol`
- `Attributes`
- `IP`
- `Domain`
- `Port`
- `Inbound Tags`
- `Balancer Tag`

这条规则的意思是：

```text
凡是这个用户对应的节点流量
全部走 hk-socks 出站
```

这样就实现了：

- 普通美国节点：美国直出
- 香港专用节点：香港出口

客户端只需要切换不同节点链接即可。

---

## 12. 保存并重启

美国 VPS 面板中完成以下操作：

1. 保存 Xray 配置
2. 重启 Xray

---

## 13. 最终验证

### 13.1 服务器侧验证

在美国 VPS 执行：

```bash
curl --socks5-hostname 10.66.66.2:10808 https://ifconfig.me
```

返回香港 IP，说明底层链路正常。

### 13.2 客户端验证

客户端连接香港专用节点后，访问：

- `https://ifconfig.me`
- `https://ipinfo.io`

如果显示香港 IP，说明整套链路已经完全生效。

---

## 14. 常见坑

### 14.1 把 `10.66.66.2` 填进入站

这是错误的。

原因：

- `10.66.66.2` 是 WireGuard 内网地址
- 客户端在公网无法直接访问它

正确做法是：

- 入站使用美国公网 IP 或域名
- `10.66.66.2:10808` 只用于美国 VPS 内部出站

### 14.2 在入站页面强行使用 `External Proxy`

本方案最终不依赖入站页面里的 `External Proxy`。

正确方式是：

- 新增美国公网入站
- 新增 `hk-socks` 出站
- 用 `User -> hk-socks` 路由规则完成转发

### 14.3 香港 sing-box 监听到了 `0.0.0.0`

不推荐。

应只监听：

```text
10.66.66.2:10808
```

### 14.4 `Min Client Ver` 和 `Max Client Ver` 填了值

测试阶段建议留空，否则可能出现客户端版本不符合导致的连接问题。

### 14.5 先改面板，后测隧道

顺序最好反过来：

1. 先测 WireGuard
2. 再测香港 socks 入站
3. 再加美国出站
4. 最后再配 3x-ui 路由

---

## 15. 推荐的复现顺序

1. 美国 VPS 安装 `WireGuard`
2. 香港 VPS 安装 `WireGuard`
3. 香港 VPS 安装 `sing-box` 或 `Xray`
4. 香港 VPS 提供 `10.66.66.2:10808` 的 `socks` 入站
5. 美国 VPS 通过 `curl --socks5-hostname` 验证香港出口
6. 美国 `3x-ui` 新增 `hk-socks` 出站
7. 美国 `3x-ui` 新增香港专用节点
8. 美国 `3x-ui` 按 `User -> hk-socks` 添加路由规则
9. 保存并重启 Xray
10. 客户端连接香港专用节点并验证出口 IP

---

## 16. 结果

完成后，可以同时保留两类节点：

- 美国直出节点
- 香港出口节点

最终效果：

- 客户端连接体验走美国入口
- 对外访问显示香港 IP
- 不需要把香港动态公网 IP 暴露给客户端
