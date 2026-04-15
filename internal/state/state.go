package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"wg-ddns/internal/health"
)

type File struct {
	Version          int            `json:"version"`
	LastReconciledAt time.Time      `json:"last_reconciled_at"`
	LastObservedUSIP string         `json:"last_observed_us_ip"`
	LastDNSChanges   []string       `json:"last_dns_changes,omitempty"`
	LastProbes       []health.Probe `json:"last_probes,omitempty"`
}

func Save(path string, state File) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
