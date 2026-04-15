package deploy

import (
	"fmt"
	"io"
	"strings"

	"wg-ddns/internal/config"
	"wg-ddns/internal/model"
	"wg-ddns/internal/render"
	"wg-ddns/internal/sshclient"
)

type RemoteFile struct {
	Node       string
	Path       string
	Mode       string
	LocalLabel string
	Content    string
}

func BuildFiles(project model.Project) ([]RemoteFile, error) {
	rendered, err := render.Generate(project)
	if err != nil {
		return nil, err
	}

	var files []RemoteFile
	for _, file := range rendered {
		switch file.Path {
		case "out/entry/wg0.conf":
			files = append(files, RemoteFile{
				Node:       "entry",
				Path:       project.Nodes.US.WGConfigPath,
				Mode:       "0600",
				LocalLabel: file.Path,
				Content:    file.Content,
			})
		case "out/exit/wg0.conf":
			files = append(files, RemoteFile{
				Node:       "exit",
				Path:       project.Nodes.HK.WGConfigPath,
				Mode:       "0600",
				LocalLabel: file.Path,
				Content:    file.Content,
			})
		case "out/exit/sing-box.json":
			files = append(files, RemoteFile{
				Node:       "exit",
				Path:       project.Nodes.HK.ProxyConfigPath,
				Mode:       "0644",
				LocalLabel: file.Path,
				Content:    file.Content,
			})
		}
	}

	return files, nil
}

type nodeEntry struct {
	key     string
	label   string
	node    model.Node
	isLocal bool
}

func Apply(project model.Project, stdout io.Writer, activate bool, rc model.RunContext) error {
	if err := config.ValidateDeploy(project, rc); err != nil {
		return err
	}

	files, err := BuildFiles(project)
	if err != nil {
		return err
	}

	entries := []nodeEntry{
		{key: "entry", label: "入口节点", node: project.Nodes.US, isLocal: rc.EntryIsLocal},
		{key: "exit", label: "出口节点", node: project.Nodes.HK, isLocal: rc.ExitIsLocal},
	}

	for _, entry := range entries {
		if err := deployNode(stdout, entry, files, activate); err != nil {
			return err
		}
	}

	return nil
}

func deployNode(stdout io.Writer, entry nodeEntry, files []RemoteFile, activate bool) error {
	if entry.isLocal {
		fmt.Fprintf(stdout, "本机部署 %s\n", entry.label)
	} else {
		fmt.Fprintf(stdout, "连接 %s (%s)\n", entry.label, entry.node.SSHAddr())
	}

	client, err := sshclient.DialOrLocal(entry.node, entry.isLocal)
	if err != nil {
		return fmt.Errorf("无法连接 %s (%s): %w", entry.label, entry.node.SSHAddr(), err)
	}
	defer client.Close()

	if err := prepareNode(stdout, client, entry.key, entry.node); err != nil {
		return err
	}

	if err := uploadNodeFiles(stdout, client, entry.key, files); err != nil {
		return err
	}

	if err := validateNode(stdout, client, entry.key, entry.node); err != nil {
		return err
	}

	if activate && !entry.node.Deploy.UploadOnly {
		if err := activateNode(stdout, client, entry.key, entry.node); err != nil {
			return err
		}
	}

	return nil
}

func prepareNode(stdout io.Writer, client sshclient.Runner, nodeKey string, node model.Node) error {
	fmt.Fprintln(stdout, "  检查远程环境")
	if err := requireRoot(client); err != nil {
		return err
	}
	if err := requireSystemd(client); err != nil {
		return err
	}
	if node.Deploy.AutoInstall {
		if err := ensureWireGuard(stdout, client); err != nil {
			return err
		}
		if nodeKey == "exit" && strings.EqualFold(node.Proxy, "sing-box") {
			if err := ensureSingBox(stdout, client); err != nil {
				return err
			}
		}
	}
	return nil
}

func uploadNodeFiles(stdout io.Writer, client sshclient.Runner, nodeKey string, files []RemoteFile) error {
	for _, file := range files {
		if file.Node != nodeKey {
			continue
		}
		fmt.Fprintf(stdout, "  上传 %s -> %s\n", file.LocalLabel, file.Path)
		if err := client.Upload(file.Path, []byte(file.Content), file.Mode); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(stdout io.Writer, client sshclient.Runner, nodeKey string, node model.Node) error {
	if nodeKey == "exit" && strings.EqualFold(node.Proxy, "sing-box") {
		fmt.Fprintf(stdout, "  验证 sing-box 配置 %s\n", node.ProxyConfigPath)
		if err := runCmd(client, fmt.Sprintf("sing-box check -c %s", quoteArg(node.ProxyConfigPath))); err != nil {
			return fmt.Errorf("sing-box 配置验证失败: %w", err)
		}
	}
	return nil
}

func activateNode(stdout io.Writer, client sshclient.Runner, nodeKey string, node model.Node) error {
	commands := []string{
		fmt.Sprintf("systemctl enable --now %s", node.WGService),
		fmt.Sprintf("systemctl restart %s", node.WGService),
	}
	if nodeKey == "exit" && node.ProxyService != "" {
		commands = append(commands,
			fmt.Sprintf("systemctl enable --now %s", node.ProxyService),
			fmt.Sprintf("systemctl restart %s", node.ProxyService),
		)
	}

	for _, command := range commands {
		fmt.Fprintf(stdout, "  执行 %s\n", command)
		if err := runCmd(client, command); err != nil {
			return err
		}
	}
	return nil
}

// runCmd executes a shell command and wraps any error with the command text
// and captured stderr/stdout.
func runCmd(client sshclient.Runner, command string) error {
	out, err := client.RunShell(command)
	if err != nil {
		msg := strings.TrimSpace(out)
		if msg != "" {
			return fmt.Errorf("%s 失败: %w: %s", command, err, msg)
		}
		return fmt.Errorf("%s 失败: %w", command, err)
	}
	return nil
}

func requireRoot(client sshclient.Runner) error {
	out, err := client.RunShell("id -u")
	if err != nil {
		return fmt.Errorf("环境检查: 无法获取 uid: %w: %s", err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "0" {
		return fmt.Errorf("环境检查: 需要 root 权限，当前 uid=%s", strings.TrimSpace(out))
	}
	return nil
}

func requireSystemd(client sshclient.Runner) error {
	out, err := client.RunShell("command -v systemctl >/dev/null 2>&1 && echo ok")
	if err != nil {
		return fmt.Errorf("环境检查: systemctl 不可用: %w: %s", err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "ok" {
		return fmt.Errorf("环境检查: systemctl 不可用")
	}
	return nil
}

func ensureWireGuard(stdout io.Writer, client sshclient.Runner) error {
	fmt.Fprintln(stdout, "  确认 WireGuard 已安装")
	script := `
if command -v wg >/dev/null 2>&1 && command -v curl >/dev/null 2>&1; then
  exit 0
fi
if command -v apt-get >/dev/null 2>&1; then
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y wireguard curl iproute2
elif command -v dnf >/dev/null 2>&1; then
  dnf install -y wireguard-tools curl iproute
elif command -v yum >/dev/null 2>&1; then
  yum install -y epel-release || true
  yum install -y wireguard-tools curl iproute
elif command -v apk >/dev/null 2>&1; then
  apk add --no-cache wireguard-tools curl iproute2
else
  echo "unsupported package manager" >&2
  exit 1
fi
command -v wg >/dev/null 2>&1
command -v curl >/dev/null 2>&1
`
	out, err := client.RunShell(script)
	if err != nil {
		return fmt.Errorf("安装 WireGuard 失败: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func ensureSingBox(stdout io.Writer, client sshclient.Runner) error {
	fmt.Fprintln(stdout, "  确认 sing-box 已安装")
	script := `
if command -v sing-box >/dev/null 2>&1; then
  exit 0
fi
if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required before installing sing-box" >&2
  exit 1
fi
curl -fsSL https://sing-box.app/install.sh | sh
command -v sing-box >/dev/null 2>&1
`
	out, err := client.RunShell(script)
	if err != nil {
		return fmt.Errorf("安装 sing-box 失败: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func quoteArg(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
