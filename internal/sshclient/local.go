package sshclient

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// LocalClient executes commands and writes files on the local machine,
// providing the same Runner interface as the SSH-based Client.
type LocalClient struct{}

func NewLocal() *LocalClient {
	return &LocalClient{}
}

func (c *LocalClient) Close() error {
	return nil
}

func (c *LocalClient) RunShell(script string) (string, error) {
	cmd := exec.Command("sh", "-lc", script)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (c *LocalClient) Upload(remotePath string, content []byte, mode string) error {
	dir := filepath.Dir(remotePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	if _, err := os.Stat(remotePath); err == nil {
		ts := time.Now().UTC().Format("20060102T150405Z")
		data, readErr := os.ReadFile(remotePath)
		if readErr == nil {
			_ = os.WriteFile(remotePath+".bak."+ts, data, 0o600)
		}
	}

	m, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		m = 0o644
	}

	return os.WriteFile(remotePath, content, os.FileMode(m))
}
