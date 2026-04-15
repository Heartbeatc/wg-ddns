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
		case "out/us/wg0.conf":
			files = append(files, RemoteFile{
				Node:       "us",
				Path:       project.Nodes.US.WGConfigPath,
				Mode:       "0600",
				LocalLabel: file.Path,
				Content:    file.Content,
			})
		case "out/hk/wg0.conf":
			files = append(files, RemoteFile{
				Node:       "hk",
				Path:       project.Nodes.HK.WGConfigPath,
				Mode:       "0600",
				LocalLabel: file.Path,
				Content:    file.Content,
			})
		case "out/hk/sing-box.json":
			files = append(files, RemoteFile{
				Node:       "hk",
				Path:       project.Nodes.HK.ProxyConfigPath,
				Mode:       "0644",
				LocalLabel: file.Path,
				Content:    file.Content,
			})
		}
	}

	return files, nil
}

func Apply(project model.Project, stdout io.Writer, activate bool) error {
	if err := config.ValidateDeploy(project); err != nil {
		return err
	}

	files, err := BuildFiles(project)
	if err != nil {
		return err
	}

	nodes := map[string]model.Node{
		"us": project.Nodes.US,
		"hk": project.Nodes.HK,
	}

	for key, node := range nodes {
		fmt.Fprintf(stdout, "Connecting to %s (%s)\n", key, node.Host)
		client, err := sshclient.Dial(node)
		if err != nil {
			return err
		}

		if err := prepareNode(stdout, client, key, node); err != nil {
			client.Close()
			return err
		}

		if err := uploadNodeFiles(stdout, client, key, files); err != nil {
			client.Close()
			return err
		}

		if err := validateNode(stdout, client, key, node); err != nil {
			client.Close()
			return err
		}

		if activate && !node.Deploy.UploadOnly {
			if err := activateNode(stdout, client, key, node); err != nil {
				client.Close()
				return err
			}
		}

		if err := client.Close(); err != nil {
			return err
		}
	}

	return nil
}

func prepareNode(stdout io.Writer, client *sshclient.Client, nodeKey string, node model.Node) error {
	fmt.Fprintln(stdout, "  Running remote preflight")
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
		if nodeKey == "hk" && strings.EqualFold(node.Proxy, "sing-box") {
			if err := ensureSingBox(stdout, client); err != nil {
				return err
			}
		}
	}
	return nil
}

func uploadNodeFiles(stdout io.Writer, client *sshclient.Client, nodeKey string, files []RemoteFile) error {
	for _, file := range files {
		if file.Node != nodeKey {
			continue
		}
		fmt.Fprintf(stdout, "  Uploading %s -> %s\n", file.LocalLabel, file.Path)
		if err := client.Upload(file.Path, []byte(file.Content), file.Mode); err != nil {
			return err
		}
	}
	return nil
}

func validateNode(stdout io.Writer, client *sshclient.Client, nodeKey string, node model.Node) error {
	if nodeKey == "hk" && strings.EqualFold(node.Proxy, "sing-box") {
		fmt.Fprintf(stdout, "  Validating sing-box config %s\n", node.ProxyConfigPath)
		out, err := client.RunShell(fmt.Sprintf("sing-box check -c %s", quoteArg(node.ProxyConfigPath)))
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg != "" {
				return fmt.Errorf("sing-box check failed: %w: %s", err, msg)
			}
			return fmt.Errorf("sing-box check failed: %w", err)
		}
	}
	return nil
}

func activateNode(stdout io.Writer, client *sshclient.Client, nodeKey string, node model.Node) error {
	commands := []string{
		fmt.Sprintf("systemctl enable --now %s", node.WGService),
		fmt.Sprintf("systemctl restart %s", node.WGService),
	}
	if nodeKey == "hk" && node.ProxyService != "" {
		commands = append(commands,
			fmt.Sprintf("systemctl enable --now %s", node.ProxyService),
			fmt.Sprintf("systemctl restart %s", node.ProxyService),
		)
	}

	for _, command := range commands {
		fmt.Fprintf(stdout, "  Running %s\n", command)
		out, err := client.RunShell(command)
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg != "" {
				return fmt.Errorf("%s failed: %w: %s", command, err, msg)
			}
			return fmt.Errorf("%s failed: %w", command, err)
		}
	}
	return nil
}

func requireRoot(client *sshclient.Client) error {
	out, err := client.RunShell("id -u")
	if err != nil {
		return fmt.Errorf("remote preflight: cannot determine uid: %w: %s", err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "0" {
		return fmt.Errorf("remote preflight: expected root login, got uid=%s", strings.TrimSpace(out))
	}
	return nil
}

func requireSystemd(client *sshclient.Client) error {
	out, err := client.RunShell("command -v systemctl >/dev/null 2>&1 && echo ok")
	if err != nil {
		return fmt.Errorf("remote preflight: systemctl not available: %w: %s", err, strings.TrimSpace(out))
	}
	if strings.TrimSpace(out) != "ok" {
		return fmt.Errorf("remote preflight: systemctl not available")
	}
	return nil
}

func ensureWireGuard(stdout io.Writer, client *sshclient.Client) error {
	fmt.Fprintln(stdout, "  Ensuring WireGuard tools are installed")
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
		return fmt.Errorf("ensure wireguard tools: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func ensureSingBox(stdout io.Writer, client *sshclient.Client) error {
	fmt.Fprintln(stdout, "  Ensuring sing-box is installed")
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
		return fmt.Errorf("ensure sing-box: %w: %s", err, strings.TrimSpace(out))
	}
	return nil
}

func quoteArg(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
