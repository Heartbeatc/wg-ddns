package reconcile

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"wg-ddns/internal/cloudflare"
	"wg-ddns/internal/config"
	"wg-ddns/internal/health"
	"wg-ddns/internal/model"
	"wg-ddns/internal/sshclient"
	"wg-ddns/internal/state"
)

type Options struct {
	DryRun    bool
	StatePath string
}

// Result carries structured output from a reconcile run so the caller
// can build notifications without re-parsing stdout text.
type Result struct {
	EntryIP   string
	Changes   []string
	IPChanged bool
	Probes    []health.Probe
}

func Run(ctx context.Context, project model.Project, stdout io.Writer, opts Options, rc model.RunContext) (Result, error) {
	var result Result

	if err := config.ValidateDeploy(project, rc); err != nil {
		return result, err
	}

	entryClient, err := sshclient.DialOrLocal(project.Nodes.US, rc.EntryIsLocal)
	if err != nil {
		return result, err
	}
	defer entryClient.Close()

	entryIP, err := health.DetectPublicIPv4(entryClient, project.Checks.PublicIPCheckURL)
	if err != nil {
		return result, err
	}
	result.EntryIP = entryIP
	fmt.Fprintf(stdout, "检测到入口节点公网 IP: %s\n", entryIP)

	cf, err := cloudflare.New(project.Cloudflare)
	if err != nil {
		return result, err
	}

	names := project.Domains.Unique()
	changes, err := cf.EnsureDNSRecords(ctx, project.Cloudflare, names, entryIP, opts.DryRun)
	if err != nil {
		return result, err
	}

	result.Changes = formatChanges(changes)
	result.IPChanged = len(changes) > 0

	if len(changes) == 0 {
		fmt.Fprintln(stdout, "Cloudflare 记录已同步。")
	} else {
		for _, line := range result.Changes {
			fmt.Fprintf(stdout, "Cloudflare %s\n", line)
		}
	}

	if !opts.DryRun && len(changes) > 0 {
		exitClient, err := sshclient.DialOrLocal(project.Nodes.HK, rc.ExitIsLocal)
		if err != nil {
			return result, err
		}
		defer exitClient.Close()

		command := fmt.Sprintf("systemctl restart %s", project.Nodes.HK.WGService)
		fmt.Fprintf(stdout, "重启出口节点 WireGuard 以更新入口地址: %s\n", command)
		out, err := exitClient.RunShell(command)
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg != "" {
				return result, fmt.Errorf("%s 失败: %w: %s", command, err, msg)
			}
			return result, fmt.Errorf("%s 失败: %w", command, err)
		}
	}

	probes, err := health.RunLive(project, rc)
	if err != nil {
		return result, err
	}
	result.Probes = probes
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, health.RenderLive(probes))

	if !opts.DryRun {
		if err := state.Save(opts.StatePath, state.File{
			Version:          1,
			LastReconciledAt: time.Now().UTC(),
			LastObservedUSIP: entryIP,
			LastDNSChanges:   result.Changes,
			LastProbes:       probes,
		}); err != nil {
			return result, err
		}
		fmt.Fprintf(stdout, "\n同步状态已保存到 %s\n", filepath.Clean(opts.StatePath))
	}

	return result, nil
}

func formatChanges(changes []cloudflare.RecordChange) []string {
	lines := make([]string, 0, len(changes))
	for _, change := range changes {
		if change.Action == "create" {
			lines = append(lines, fmt.Sprintf("%s %s -> %s", change.Action, change.Name, change.After))
		} else {
			lines = append(lines, fmt.Sprintf("%s %s: %s => %s", change.Action, change.Name, change.Before, change.After))
		}
	}
	return lines
}
