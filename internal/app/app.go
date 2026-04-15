package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"wg-ddns/internal/config"
	"wg-ddns/internal/deploy"
	"wg-ddns/internal/guide"
	"wg-ddns/internal/health"
	"wg-ddns/internal/model"
	"wg-ddns/internal/planner"
	"wg-ddns/internal/reconcile"
	"wg-ddns/internal/render"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	switch args[0] {
	case "init":
		return runInit(args[1:], stdout)
	case "plan":
		return runPlan(args[1:], stdout)
	case "render":
		return runRender(args[1:], stdout)
	case "apply":
		return runApply(args[1:], stdout)
	case "guide":
		return runGuide(args[1:], stdout)
	case "health":
		return runHealth(args[1:], stdout)
	case "reconcile":
		return runReconcile(args[1:], stdout)
	case "help", "-h", "--help":
		printUsage(stdout)
		return nil
	default:
		printUsage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project := config.DefaultProject()
	if _, err := os.Stat(*path); err == nil {
		return fmt.Errorf("config already exists: %s", *path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := config.Save(*path, project); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Created sample config at %s\n", *path)
	return nil
}

func runPlan(args []string, stdout io.Writer) error {
	project, err := loadProject(args)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Project: %s\n\n", project.Project)
	fmt.Fprint(stdout, planner.Render(planner.Build(project)))
	return nil
}

func runRender(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	outDir := fs.String("out", "build", "output directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	files, err := render.Generate(project)
	if err != nil {
		return err
	}
	if err := render.WriteAll(*outDir, files); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Rendered %d files into %s\n", len(files), filepath.Clean(*outDir))
	for _, file := range files {
		fmt.Fprintf(stdout, "- %s\n", filepath.Join(filepath.Clean(*outDir), file.Path))
	}
	return nil
}

func runGuide(args []string, stdout io.Writer) error {
	project, err := loadProject(args)
	if err != nil {
		return err
	}

	fmt.Fprint(stdout, guide.Render(project))
	return nil
}

func runApply(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	activate := fs.Bool("activate", true, "enable and restart remote services after upload")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	return deploy.Apply(project, stdout, *activate)
}

func runHealth(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	live := fs.Bool("live", false, "run live checks over SSH and DNS")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	if *live {
		probes, err := health.RunLive(project)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Live health checks for %s\n\n", project.Project)
		fmt.Fprint(stdout, health.RenderLive(probes))
		return nil
	}

	fmt.Fprintf(stdout, "Expected health checks for %s\n\n", project.Project)
	fmt.Fprint(stdout, health.Render(health.Expected(project)))
	return nil
}

func runReconcile(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	dryRun := fs.Bool("dry-run", false, "show changes without updating Cloudflare or restarting services")
	statePath := fs.String("state", ".wgstack-state.json", "path to persist reconcile state")
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	return reconcile.Run(context.Background(), project, stdout, reconcile.Options{DryRun: *dryRun, StatePath: *statePath})
}

func loadProject(args []string) (model.Project, error) {
	fs := flag.NewFlagSet("command", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return model.Project{}, err
	}

	return config.Load(*path)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "wgstack <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  init    Create a sample JSON config")
	fmt.Fprintln(w, "  plan    Show the deployment plan")
	fmt.Fprintln(w, "  render  Render WireGuard and sing-box configs locally")
	fmt.Fprintln(w, "  apply   Upload configs to remote hosts over SSH")
	fmt.Fprintln(w, "  guide   Print manual panel steps")
	fmt.Fprintln(w, "  health  Print expected checks or run live probes")
	fmt.Fprintln(w, "  reconcile  Sync Cloudflare DNS and refresh HK WireGuard when needed")
}
