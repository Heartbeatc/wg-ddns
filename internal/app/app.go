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
	"wg-ddns/internal/keygen"
	"wg-ddns/internal/model"
	"wg-ddns/internal/notify"
	"wg-ddns/internal/planner"
	"wg-ddns/internal/reconcile"
	"wg-ddns/internal/render"
	"wg-ddns/internal/wizard"
)

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return runInteractive(stdout)
	}

	switch args[0] {
	case "setup":
		return runSetupWizard(stdout)
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

func runInteractive(stdout io.Writer) error {
	if !wizard.IsTerminal() {
		printUsage(stdout)
		return nil
	}

	if _, err := os.Stat(config.DefaultPath); os.IsNotExist(err) {
		return runSetupWizard(stdout)
	}
	return runMenu(stdout)
}

func runSetupWizard(stdout io.Writer) error {
	project, rc, shouldDeploy, err := wizard.RunSetup(stdout)
	if err != nil {
		return err
	}

	if err := config.Save(config.DefaultPath, project); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	fmt.Fprintf(stdout, "\n配置已保存到 %s\n", config.DefaultPath)

	if !shouldDeploy {
		fmt.Fprintln(stdout, "\n你可以稍后运行 wgstack apply 来部署。")
		return nil
	}

	notif := notify.FromConfig(project.Notifications, stdout)

	fmt.Fprint(stdout, "\n--- 开始部署 ---\n\n")
	if err := deploy.Apply(project, stdout, true, rc); err != nil {
		notify.Fire(stdout, notif, notify.FormatApplyFailure(project.Project, err.Error()))
		fmt.Fprintf(stdout, "\n配置已保存到 %s，你可以修复问题后运行 wgstack apply 重试。\n", config.DefaultPath)
		return fmt.Errorf("部署失败: %w", err)
	}

	notify.Fire(stdout, notif, notify.FormatApplySuccess(project.Project, project.Nodes.US.Host, project.Nodes.HK.Host))

	fmt.Fprintln(stdout, "\n--- 部署完成 ---")
	fmt.Fprint(stdout, "\n底层部署已完成！接下来需要去面板中完成最后几步：\n\n")
	fmt.Fprint(stdout, guide.Render(project))
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "日常维护命令：")
	fmt.Fprintln(stdout, "  wgstack                  打开主菜单")
	fmt.Fprintln(stdout, "  wgstack health --live     检查连通性")
	fmt.Fprintln(stdout, "  wgstack reconcile         同步 DNS / 修复 IP 漂移")
	fmt.Fprintln(stdout, "  wgstack apply             重新部署")
	return nil
}

func runMenu(stdout io.Writer) error {
	project, err := config.Load(config.DefaultPath)
	if err != nil {
		fmt.Fprintf(stdout, "无法读取配置文件 %s：%v\n\n", config.DefaultPath, err)
		fmt.Fprintln(stdout, "运行 wgstack setup 重新配置。")
		return nil
	}

	p := wizard.NewPrompter(stdout)

	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "========================================")
	fmt.Fprintln(stdout, "  wgstack 主菜单")
	fmt.Fprintln(stdout, "========================================")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "当前配置: %s\n", config.DefaultPath)
	fmt.Fprintf(stdout, "  入口节点: %s@%s\n", project.Nodes.US.SSH.User, project.Nodes.US.SSHAddr())
	fmt.Fprintf(stdout, "  出口节点: %s@%s\n", project.Nodes.HK.SSH.User, project.Nodes.HK.SSHAddr())
	fmt.Fprintf(stdout, "  对外域名: %s\n", project.Domains.Entry)
	fmt.Fprintln(stdout)

	choice := p.Select("请选择操作:", []string{
		"重新配置（运行部署向导）",
		"部署到服务器",
		"检查连通性",
		"同步 DNS / 修复 IP 漂移",
		"查看面板操作说明",
		"退出",
	})

	if p.Err() != nil {
		return p.Err()
	}

	notif := notify.FromConfig(project.Notifications, stdout)

	switch choice {
	case 0:
		return runSetupWizard(stdout)
	case 1:
		rc := wizard.AskRunContext(p)
		if p.Err() != nil {
			return p.Err()
		}
		if err := deploy.Apply(project, stdout, true, rc); err != nil {
			notify.Fire(stdout, notif, notify.FormatApplyFailure(project.Project, err.Error()))
			return err
		}
		notify.Fire(stdout, notif, notify.FormatApplySuccess(project.Project, project.Nodes.US.Host, project.Nodes.HK.Host))
	case 2:
		rc := wizard.AskRunContext(p)
		if p.Err() != nil {
			return p.Err()
		}
		probes, liveErr := health.RunLive(project, rc)
		if liveErr != nil {
			notify.Fire(stdout, notif, notify.FormatHealthRunError(project.Project, liveErr.Error()))
			return liveErr
		}
		fmt.Fprint(stdout, "\n连通性检查结果：\n\n")
		fmt.Fprint(stdout, health.RenderLive(probes))
		notifyHealthIfFailed(stdout, notif, project.Project, probes)
	case 3:
		rc := wizard.AskRunContext(p)
		if p.Err() != nil {
			return p.Err()
		}
		result, runErr := reconcile.Run(context.Background(), project, stdout, reconcile.Options{StatePath: ".wgstack-state.json"}, rc)
		handleReconcileResult(stdout, notif, project, result, runErr)
		if runErr != nil {
			return runErr
		}
	case 4:
		fmt.Fprintln(stdout)
		fmt.Fprint(stdout, guide.Render(project))
	case 5:
		// exit
	}
	return nil
}

func runInit(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if _, err := os.Stat(*path); err == nil {
		return fmt.Errorf("配置文件已存在: %s", *path)
	} else if !os.IsNotExist(err) {
		return err
	}

	project := config.DefaultProject()

	entryKey, err := keygen.Generate()
	if err != nil {
		return err
	}
	exitKey, err := keygen.Generate()
	if err != nil {
		return err
	}
	project.Nodes.US.WGPrivateKey = entryKey.PrivateKey
	project.Nodes.US.WGPublicKey = entryKey.PublicKey
	project.Nodes.HK.WGPrivateKey = exitKey.PrivateKey
	project.Nodes.HK.WGPublicKey = exitKey.PublicKey

	if err := config.Save(*path, project); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "已生成配置模板: %s\n", *path)
	fmt.Fprintln(stdout, "请编辑该文件填入真实的节点信息，然后运行 wgstack apply")
	return nil
}

func runPlan(args []string, stdout io.Writer) error {
	project, err := loadProject(args)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "项目: %s\n\n", project.Project)
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

	fmt.Fprintf(stdout, "已渲染 %d 个文件到 %s\n", len(files), filepath.Clean(*outDir))
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
	localEntry, localExit := addRunContextFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	notif := notify.FromConfig(project.Notifications, stdout)
	rc := buildRunContext(localEntry, localExit)
	if err := deploy.Apply(project, stdout, *activate, rc); err != nil {
		notify.Fire(stdout, notif, notify.FormatApplyFailure(project.Project, err.Error()))
		return err
	}
	notify.Fire(stdout, notif, notify.FormatApplySuccess(project.Project, project.Nodes.US.Host, project.Nodes.HK.Host))
	return nil
}

func runHealth(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	live := fs.Bool("live", false, "run live checks over SSH and DNS")
	localEntry, localExit := addRunContextFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	if *live {
		rc := buildRunContext(localEntry, localExit)
		notif := notify.FromConfig(project.Notifications, stdout)
		probes, liveErr := health.RunLive(project, rc)
		if liveErr != nil {
			notify.Fire(stdout, notif, notify.FormatHealthRunError(project.Project, liveErr.Error()))
			return liveErr
		}
		fmt.Fprintf(stdout, "%s 实时健康检查\n\n", project.Project)
		fmt.Fprint(stdout, health.RenderLive(probes))
		notifyHealthIfFailed(stdout, notif, project.Project, probes)
		return nil
	}

	fmt.Fprintf(stdout, "%s 预期健康检查项\n\n", project.Project)
	fmt.Fprint(stdout, health.Render(health.Expected(project)))
	return nil
}

func runReconcile(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("reconcile", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	path := fs.String("config", config.DefaultPath, "config file path")
	dryRun := fs.Bool("dry-run", false, "show changes without updating Cloudflare or restarting services")
	statePath := fs.String("state", ".wgstack-state.json", "path to persist reconcile state")
	localEntry, localExit := addRunContextFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}

	project, err := config.Load(*path)
	if err != nil {
		return err
	}

	notif := notify.FromConfig(project.Notifications, stdout)
	rc := buildRunContext(localEntry, localExit)
	result, runErr := reconcile.Run(context.Background(), project, stdout, reconcile.Options{DryRun: *dryRun, StatePath: *statePath}, rc)
	handleReconcileResult(stdout, notif, project, result, runErr)
	return runErr
}

// handleReconcileResult sends the appropriate notification after a reconcile run.
func handleReconcileResult(stdout io.Writer, notif notify.Notifier, project model.Project, result reconcile.Result, err error) {
	if notify.IsNop(notif) {
		return
	}

	if err != nil {
		notify.Fire(stdout, notif, notify.FormatReconcileFailure(project.Project, err.Error()))
		return
	}

	if !result.IPChanged {
		return
	}

	var ipInfo *notify.IPInfo
	info, lookupErr := notify.LookupIP(context.Background(), result.EntryIP)
	if lookupErr == nil {
		ipInfo = &info
	}

	notify.Fire(stdout, notif, notify.FormatReconcileSuccess(
		project.Project,
		result.EntryIP,
		result.Changes,
		probesToInfos(result.Probes),
		ipInfo,
	))
}

// notifyHealthIfFailed sends a notification when any probe has failed.
func notifyHealthIfFailed(stdout io.Writer, notif notify.Notifier, project string, probes []health.Probe) {
	if notify.IsNop(notif) {
		return
	}
	hasFail := false
	for _, p := range probes {
		if p.Status == "FAIL" {
			hasFail = true
			break
		}
	}
	if !hasFail {
		return
	}
	notify.Fire(stdout, notif, notify.FormatHealthFailure(project, probesToInfos(probes)))
}

func probesToInfos(probes []health.Probe) []notify.ProbeInfo {
	infos := make([]notify.ProbeInfo, len(probes))
	for i, p := range probes {
		infos[i] = notify.ProbeInfo{
			Name:     p.Name,
			Status:   p.Status,
			Detail:   p.Detail,
			Duration: p.Duration,
		}
	}
	return infos
}

// addRunContextFlags registers --local-entry and --local-exit on a FlagSet.
func addRunContextFlags(fs *flag.FlagSet) (localEntry, localExit *bool) {
	localEntry = fs.Bool("local-entry", false, "当前机器即入口节点，跳过入口节点 SSH")
	localExit = fs.Bool("local-exit", false, "当前机器即出口节点，跳过出口节点 SSH")
	return
}

func buildRunContext(localEntry, localExit *bool) model.RunContext {
	return model.RunContext{
		EntryIsLocal: *localEntry,
		ExitIsLocal:  *localExit,
	}
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
	fmt.Fprintln(w, "wgstack - 代理底层部署工具")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "用法:")
	fmt.Fprintln(w, "  wgstack              进入交互式主菜单")
	fmt.Fprintln(w, "  wgstack setup        运行部署向导")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "高级命令:")
	fmt.Fprintln(w, "  init         生成配置模板")
	fmt.Fprintln(w, "  plan         查看部署计划")
	fmt.Fprintln(w, "  render       生成本地配置文件")
	fmt.Fprintln(w, "  apply        部署到服务器")
	fmt.Fprintln(w, "  guide        查看面板操作说明")
	fmt.Fprintln(w, "  health       运行健康检查")
	fmt.Fprintln(w, "  reconcile    同步 DNS / 修复 IP 漂移")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "节点本机运行时可用参数:")
	fmt.Fprintln(w, "  --local-entry    当前机器即入口节点，跳过入口节点 SSH")
	fmt.Fprintln(w, "  --local-exit     当前机器即出口节点，跳过出口节点 SSH")
}
