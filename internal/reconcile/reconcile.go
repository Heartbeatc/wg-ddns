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

func Run(ctx context.Context, project model.Project, stdout io.Writer, opts Options, rc model.RunContext) error {
	if err := config.ValidateDeploy(project, rc); err != nil {
		return err
	}

	entryClient, err := sshclient.DialOrLocal(project.Nodes.US, rc.EntryIsLocal)
	if err != nil {
		return err
	}
	defer entryClient.Close()

	entryIP, err := health.DetectPublicIPv4(entryClient, project.Checks.PublicIPCheckURL)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "检测到入口节点公网 IP: %s\n", entryIP)

	cf, err := cloudflare.New(project.Cloudflare)
	if err != nil {
		return err
	}

	names := project.Domains.Unique()
	changes, err := cf.EnsureDNSRecords(ctx, project.Cloudflare, names, entryIP, opts.DryRun)
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		fmt.Fprintln(stdout, "Cloudflare 记录已同步。")
	} else {
		for _, change := range changes {
			if change.Action == "create" {
				fmt.Fprintf(stdout, "Cloudflare %s %s -> %s\n", change.Action, change.Name, change.After)
			} else {
				fmt.Fprintf(stdout, "Cloudflare %s %s: %s => %s\n", change.Action, change.Name, change.Before, change.After)
			}
		}
	}

	if !opts.DryRun && len(changes) > 0 {
		exitClient, err := sshclient.DialOrLocal(project.Nodes.HK, rc.ExitIsLocal)
		if err != nil {
			return err
		}
		defer exitClient.Close()

		command := fmt.Sprintf("systemctl restart %s", project.Nodes.HK.WGService)
		fmt.Fprintf(stdout, "重启出口节点 WireGuard 以更新入口地址: %s\n", command)
		out, err := exitClient.RunShell(command)
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg != "" {
				return fmt.Errorf("%s 失败: %w: %s", command, err, msg)
			}
			return fmt.Errorf("%s 失败: %w", command, err)
		}
	}

	probes, err := health.RunLive(project, rc)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout)
	fmt.Fprint(stdout, health.RenderLive(probes))

	if !opts.DryRun {
		var changeLines []string
		for _, change := range changes {
			if change.Action == "create" {
				changeLines = append(changeLines, fmt.Sprintf("%s %s -> %s", change.Action, change.Name, change.After))
			} else {
				changeLines = append(changeLines, fmt.Sprintf("%s %s: %s => %s", change.Action, change.Name, change.Before, change.After))
			}
		}
		if err := state.Save(opts.StatePath, state.File{
			Version:          1,
			LastReconciledAt: time.Now().UTC(),
			LastObservedUSIP: entryIP,
			LastDNSChanges:   changeLines,
			LastProbes:       probes,
		}); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "\n同步状态已保存到 %s\n", filepath.Clean(opts.StatePath))
	}

	return nil
}
