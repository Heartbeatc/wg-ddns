package sshclient

import "wg-ddns/internal/model"

// Runner abstracts command execution and file transfer.
// Both *Client (remote via SSH) and *LocalClient (local execution) satisfy this.
type Runner interface {
	RunShell(script string) (string, error)
	Upload(remotePath string, content []byte, mode string) error
	Close() error
}

// DialOrLocal returns a Runner for the given node.
// isLocal is a runtime-only flag (from RunContext or wizard) — it is never
// read from the persisted config to avoid cross-machine misuse.
func DialOrLocal(node model.Node, isLocal bool) (Runner, error) {
	if isLocal {
		return NewLocal(), nil
	}
	return Dial(node)
}
