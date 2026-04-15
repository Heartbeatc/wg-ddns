package sshclient

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"wg-ddns/internal/model"
)

type Client struct {
	client *ssh.Client
}

func Dial(node model.Node) (*Client, error) {
	auth, err := authMethod(node.SSH)
	if err != nil {
		return nil, fmt.Errorf("prepare ssh auth for %s: %w", node.Host, err)
	}

	hostKeyCallback, err := hostKeyCallback(node.SSH)
	if err != nil {
		return nil, fmt.Errorf("prepare host key callback for %s: %w", node.Host, err)
	}

	cfg := &ssh.ClientConfig{
		User:            node.SSH.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	addr := net.JoinHostPort(node.Host, strconv.Itoa(node.SSH.Port))
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	return &Client{client: c}, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}

func (c *Client) Run(command string) (string, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	out, err := session.CombinedOutput(command)
	return string(out), err
}

func (c *Client) RunShell(script string) (string, error) {
	return c.Run("sh -lc " + shellQuote(script))
}

func (c *Client) Upload(remotePath string, content []byte, mode string) error {
	session, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = bytes.NewReader(content)
	timestamp := time.Now().UTC().Format("20060102T150405Z")
	script := fmt.Sprintf(
		"set -eu\nmkdir -p %s\nif [ -f %s ]; then cp %s %s; fi\ntmp=$(mktemp)\ncat > \"$tmp\"\ninstall -m %s \"$tmp\" %s\nrm -f \"$tmp\"\n",
		shellQuote(path.Dir(remotePath)),
		shellQuote(remotePath),
		shellQuote(remotePath),
		shellQuote(remotePath+".bak."+timestamp),
		mode,
		shellQuote(remotePath),
	)

	out, err := session.CombinedOutput("sh -lc " + shellQuote(script))
	if err != nil {
		return fmt.Errorf("upload %s: %w: %s", remotePath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func authMethod(cfg model.SSH) (ssh.AuthMethod, error) {
	switch cfg.AuthMethod {
	case "password":
		password := cfg.Password
		if password == "" && cfg.PasswordEnv != "" {
			password = os.Getenv(cfg.PasswordEnv)
		}
		if password == "" {
			return nil, fmt.Errorf("password auth selected but no password available")
		}
		return ssh.Password(password), nil
	case "private_key":
		keyPath, err := expandPath(cfg.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key %s: %w", keyPath, err)
		}

		passphrase := cfg.PrivateKeyPassphrase
		if passphrase == "" && cfg.PassphraseEnv != "" {
			passphrase = os.Getenv(cfg.PassphraseEnv)
		}

		var signer ssh.Signer
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key %s: %w", keyPath, err)
		}

		return ssh.PublicKeys(signer), nil
	default:
		return nil, fmt.Errorf("unsupported auth method %q", cfg.AuthMethod)
	}
}

func hostKeyCallback(cfg model.SSH) (ssh.HostKeyCallback, error) {
	if cfg.KnownHostsPath != "" {
		path, err := expandPath(cfg.KnownHostsPath)
		if err != nil {
			return nil, err
		}
		return knownhosts.New(path)
	}
	if cfg.InsecureIgnoreHostKey {
		return ssh.InsecureIgnoreHostKey(), nil
	}
	return nil, fmt.Errorf("either known_hosts_path or insecure_ignore_host_key=true is required")
}

func expandPath(v string) (string, error) {
	if v == "" {
		return "", nil
	}
	if strings.HasPrefix(v, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return path.Join(home, strings.TrimPrefix(v, "~/")), nil
	}
	return v, nil
}

func shellQuote(v string) string {
	return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'"
}
