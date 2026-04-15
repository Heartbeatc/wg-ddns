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

func Run(ctx context.Context, project model.Project, stdout io.Writer, opts Options) error {
	if err := config.ValidateDeploy(project); err != nil {
		return err
	}

	usClient, err := sshclient.Dial(project.Nodes.US)
	if err != nil {
		return err
	}
	defer usClient.Close()

	usIP, err := health.DetectPublicIPv4(usClient, project.Checks.PublicIPCheckURL)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Observed US public IP: %s\n", usIP)

	cf, err := cloudflare.New(project.Cloudflare)
	if err != nil {
		return err
	}

	names := []string{project.Domains.Entry, project.Domains.Panel, project.Domains.WireGuard}
	changes, err := cf.EnsureDNSRecords(ctx, project.Cloudflare, names, usIP, opts.DryRun)
	if err != nil {
		return err
	}

	if len(changes) == 0 {
		fmt.Fprintln(stdout, "Cloudflare records are already in sync.")
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
		hkClient, err := sshclient.Dial(project.Nodes.HK)
		if err != nil {
			return err
		}
		defer hkClient.Close()

		command := fmt.Sprintf("systemctl restart %s", project.Nodes.HK.WGService)
		fmt.Fprintf(stdout, "Refreshing HK WireGuard endpoint via %s\n", command)
		out, err := hkClient.RunShell(command)
		if err != nil {
			msg := strings.TrimSpace(out)
			if msg != "" {
				return fmt.Errorf("%s failed: %w: %s", command, err, msg)
			}
			return fmt.Errorf("%s failed: %w", command, err)
		}
	}

	probes, err := health.RunLive(project)
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
			LastObservedUSIP: usIP,
			LastDNSChanges:   changeLines,
			LastProbes:       probes,
		}); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "\nSaved reconcile state to %s\n", filepath.Clean(opts.StatePath))
	}

	return nil
}
