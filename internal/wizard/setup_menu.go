package wizard

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"wg-ddns/internal/config"
	"wg-ddns/internal/model"
)

// RunSetupMenu runs the menu-based configuration flow. Configuration stays in
// memory until the user chooses save or deploy. If draft is nil, a new draft
// with defaults is created.
func RunSetupMenu(w io.Writer, draft *SetupDraft) (*SetupResult, error) {
	if !IsTerminal() {
		return nil, fmt.Errorf("部署向导需要在终端中运行。\n如需非交互模式，请使用: wgstack apply --config <配置文件>")
	}

	if draft == nil {
		var err error
		draft, err = NewSetupDraft()
		if err != nil {
			return nil, err
		}
	}
	p := NewPrompter(w)

	printWelcome(w)
	fmt.Fprint(w, "\n按回车开始 6 步引导...")
	p.WaitEnter()
	if p.Err() != nil {
		return nil, p.Err()
	}

	for {
		runGuidedSetup(w, p, draft)
		if p.Err() != nil {
			return nil, p.Err()
		}
		runAutomaticPreflight(w, draft)

		fmt.Fprintln(w, renderPanelTitle("配置摘要与下一步"))
		if done, act := runSummaryMenu(w, p, draft); done {
			return finalizeSetupResult(draft, act)
		}
	}
}

func runGuidedSetup(w io.Writer, p *Prompter, draft *SetupDraft) {
	printProgressHeader(w, 1, 6, "运行位置", "先确认你现在在哪台机器上运行 wgstack。")
	stepRunLocation(w, p, draft)
	if p.Err() != nil {
		return
	}

	printProgressHeader(w, 2, 6, "入口节点", "先填写当前可直连地址，再自动确认当前公网 IP。")
	stepEntryNode(w, p, draft)
	if p.Err() != nil {
		return
	}

	printProgressHeader(w, 3, 6, "出口节点", "先填写当前可直连地址，再自动确认当前公网 IP。")
	stepExitNode(w, p, draft)
	if p.Err() != nil {
		return
	}

	printProgressHeader(w, 4, 6, "Cloudflare", "填写 Zone 和 API Token。")
	stepCloudflare(w, p, draft)
	if p.Err() != nil {
		return
	}

	printProgressHeader(w, 5, 6, "域名", "默认只问一个入口业务域名；首次部署时会自动创建/更新 DNS。")
	stepDomains(w, p, draft)
	if p.Err() != nil {
		return
	}

	printProgressHeader(w, 6, 6, "自动化与检查", "设置出口 DDNS、入口自动修复，以及健康检查参数。")
	stepExitDDNS(w, p, draft)
	if p.Err() != nil {
		return
	}
	stepEntryAuto(w, p, draft)
	if p.Err() != nil {
		return
	}
	applyPanelHealthDefaults(w, draft)
}

func printProgressHeader(w io.Writer, step, total int, title, hint string) {
	fmt.Fprintln(w, renderProgress(step, total, title, hint))
}

func finalizeSetupResult(d *SetupDraft, act SetupAction) (*SetupResult, error) {
	return &SetupResult{
		Action:  act,
		Project: d.Project,
		RC:      d.RC,
	}, nil
}

func stepRunLocation(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("运行位置"))
	idx := p.Select("你当前在哪台机器上运行 wgstack？", RunLocationOptions)
	if p.Err() != nil {
		return
	}
	d.RC = model.RunContext{
		EntryIsLocal: idx == 1,
		ExitIsLocal:  idx == 2,
	}
	d.RCSet = true
	fmt.Fprintln(w)
}

func stepEntryNode(w io.Writer, p *Prompter, d *SetupDraft) {
	if !d.RCSet {
		fmt.Fprintln(w, "\n请先配置「运行位置」。")
		return
	}
	fmt.Fprintln(w, renderSectionTitle("入口节点"))
	ni := collectNodeInfoWithDefaults(w, p, "入口节点", d.RC.EntryIsLocal, d.Project.Nodes.US)
	if p.Err() != nil {
		return
	}
	applyNodeInput(&d.Project.Nodes.US, ni)
	fmt.Fprintln(w)
}

func stepExitNode(w io.Writer, p *Prompter, d *SetupDraft) {
	if !d.RCSet {
		fmt.Fprintln(w, "\n请先配置「运行位置」。")
		return
	}
	fmt.Fprintln(w, renderSectionTitle("出口节点"))
	ni := collectNodeInfoWithDefaults(w, p, "出口节点", d.RC.ExitIsLocal, d.Project.Nodes.HK)
	if p.Err() != nil {
		return
	}
	applyNodeInput(&d.Project.Nodes.HK, ni)
	fmt.Fprintln(w)
}

func applyNodeInput(n *model.Node, ni nodeInput) {
	n.Host = ni.host
	if ni.sshHostSet {
		n.SSHHost = ni.sshHost
	}
	n.SSH.User = ni.user
	n.SSH.AuthMethod = ni.authMethod
	n.SSH.Password = ni.password
	n.SSH.PrivateKeyPath = ni.keyPath
	if n.SSH.Port == 0 {
		n.SSH.Port = 22
	}
	n.SSH.InsecureIgnoreHostKey = true
}

func stepCloudflare(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("Cloudflare"))
	zoneDef := strings.TrimSpace(d.Project.Cloudflare.Zone)
	zone := p.LineWith("Cloudflare Zone 域名（主域名，如 example.com）", zoneDef, validateDomain)
	if p.Err() != nil {
		return
	}
	d.Project.Cloudflare.Zone = zone
	if d.Project.Cloudflare.TokenEnv == "" {
		d.Project.Cloudflare.TokenEnv = "CLOUDFLARE_API_TOKEN"
	}
	if d.Project.Cloudflare.RecordType == "" {
		d.Project.Cloudflare.RecordType = "A"
	}
	if d.Project.Cloudflare.TTL < 1 {
		d.Project.Cloudflare.TTL = 120
	}

	tok := p.PasswordOptional("Cloudflare API Token")
	if p.Err() != nil {
		return
	}
	if tok != "" {
		d.Project.Cloudflare.Token = tok
	} else if strings.TrimSpace(d.Project.Cloudflare.Token) == "" {
		d.Project.Cloudflare.Token = p.Password("Cloudflare API Token（必填）")
	}
	fmt.Fprintln(w)
}

func stepDomains(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("域名"))
	cfZone := strings.TrimSpace(d.Project.Cloudflare.Zone)
	if cfZone == "" {
		fmt.Fprintln(w, helpStyle.Render("  建议先配置 Cloudflare Zone，以便使用更合理的默认值。"))
	}

	fmt.Fprintln(w, helpStyle.Render("  默认只需要一个入口业务域名；面板、VLESS 和 WG 默认共用它。"))
	fmt.Fprintln(w, helpStyle.Render("  首次部署时，wgstack 会自动在 Cloudflare 上创建/更新这些记录。"))
	previousEntryDomain := strings.TrimSpace(d.Project.Domains.Entry)
	entryDef := previousEntryDomain
	if entryDef == "" {
		entryDef = cfZone
	}
	entryDomain := p.LineWith("入口业务域名", entryDef, validateDomain)
	if p.Err() != nil {
		return
	}

	shouldSyncEntrySSH := shouldSyncEntrySSHHost(d.Project.Nodes.US.SSHHost, previousEntryDomain)
	d.Project.Domains = model.Domains{
		Entry:     entryDomain,
		Panel:     entryDomain,
		WireGuard: entryDomain,
	}
	if shouldSyncEntrySSH {
		d.Project.Nodes.US.SSHHost = entryDomain
	}
	fmt.Fprintln(w, helpStyle.Render("  面板、代理入口和 WG Endpoint 将默认复用这个域名。"))
	fmt.Fprintln(w)
}

func shouldSyncEntrySSHHost(currentSSHHost, previousEntryDomain string) bool {
	currentSSHHost = strings.TrimSpace(currentSSHHost)
	previousEntryDomain = strings.TrimSpace(previousEntryDomain)
	return currentSSHHost == "" || (previousEntryDomain != "" && currentSSHHost == previousEntryDomain)
}

func stepExitDDNS(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("出口管理 DDNS"))
	fmt.Fprintln(w, helpStyle.Render("  默认启用，用于维护出口 SSH 管理域名。出口 IP 变化后，本工具仍能连上出口节点。"))
	fmt.Fprintln(w, helpStyle.Render("  首次部署时，wgstack 也会自动创建这个管理域名的 DNS 记录。"))
	d.ExitDDNSTouched = true

	cfZone := strings.TrimSpace(d.Project.Cloudflare.Zone)
	if cfZone == "" {
		fmt.Fprintln(w, helpStyle.Render("  尚未填写 Cloudflare Zone，默认域名后缀可能不完整。"))
	}

	ddDef := strings.TrimSpace(d.Project.ExitDDNS.Domain)
	if ddDef == "" {
		ddDef = domainDefault(d.Project.Nodes.HK.SSHHost, "ssh-exit."+cfZone)
	}
	dd := p.LineWith("出口 SSH 管理域名", ddDef, validateDomain)
	if p.Err() != nil {
		return
	}
	interval := promptIntervalSeconds(p, "出口 DDNS 检查间隔秒数", d.Project.ExitDDNS.Interval)
	if p.Err() != nil {
		return
	}
	d.Project.ExitDDNS = model.ExitDDNS{Enabled: true, Domain: dd, Interval: interval}
	d.Project.Nodes.HK.SSHHost = dd
	fmt.Fprintf(w, "%s\n", helpStyle.Render(fmt.Sprintf("  已启用出口管理 DDNS，检查间隔 %ds，并将出口 SSH 管理域名设为该地址。", interval)))
	fmt.Fprintln(w)
}

func stepEntryAuto(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("入口自动修复"))
	fmt.Fprintln(w, helpStyle.Render("  默认启用。入口节点会定时检查 IP / DNS 漂移，并在需要时自动修复。"))
	d.EntryAutoTouched = true
	interval := promptIntervalSeconds(p, "入口自动修复检查间隔秒数", d.Project.EntryAutoReconcile.Interval)
	if p.Err() != nil {
		return
	}
	d.Project.EntryAutoReconcile = model.AutoReconcile{Enabled: true, Interval: interval}
	fmt.Fprintf(w, "%s\n", helpStyle.Render(fmt.Sprintf("  已启用入口自动修复，检查间隔 %ds。", interval)))
	fmt.Fprintln(w)
}

func promptIntervalSeconds(p *Prompter, prompt string, current int) int {
	if current < 60 {
		current = 60
	}
	val := p.LineWith(prompt, strconv.Itoa(current), validateIntervalSeconds)
	if p.Err() != nil {
		return current
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return current
	}
	return n
}

func validateIntervalSeconds(v string) string {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return "请输入数字，例如 60"
	}
	if n < 60 {
		return "检查间隔不能小于 60 秒"
	}
	return ""
}

func domainDefault(current, fallback string) string {
	current = strings.TrimSpace(current)
	if current != "" && net.ParseIP(current) == nil {
		return current
	}
	return fallback
}

func applyPanelHealthDefaults(w io.Writer, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("面板与检查"))
	if strings.TrimSpace(d.Project.PanelGuide.OutboundTag) == "" {
		d.Project.PanelGuide.OutboundTag = "exit-socks"
	}
	if strings.TrimSpace(d.Project.PanelGuide.RouteUser) == "" {
		d.Project.PanelGuide.RouteUser = "exit-user@local"
	}
	if strings.TrimSpace(d.Project.Checks.TestURL) == "" {
		d.Project.Checks.TestURL = "https://ifconfig.me"
	}
	if strings.TrimSpace(d.Project.Checks.ExitCheckURL) == "" {
		d.Project.Checks.ExitCheckURL = "https://api.ipify.org"
	}
	if strings.TrimSpace(d.Project.Checks.PublicIPCheckURL) == "" {
		d.Project.Checks.PublicIPCheckURL = "https://api.ipify.org"
	}
	fmt.Fprintf(w, "  已采用默认面板出站标签：%s\n", d.Project.PanelGuide.OutboundTag)
	fmt.Fprintf(w, "  已采用默认专用线路用户：%s\n", d.Project.PanelGuide.RouteUser)
	if strings.TrimSpace(d.Project.Checks.ExitLocation) == "" {
		fmt.Fprintln(w, "  出口地区校验：未启用（可在摘要页进入「面板与检查」修改）。")
	} else {
		fmt.Fprintf(w, "  出口地区校验：%s\n", d.Project.Checks.ExitLocation)
	}
	fmt.Fprintln(w)
}

func stepPanelHealth(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, renderSectionTitle("面板与检查"))
	ob := p.LineWith("面板出站标签", d.Project.PanelGuide.OutboundTag, nil)
	ru := p.LineWith("专用线路用户标识", d.Project.PanelGuide.RouteUser, nil)
	if p.Err() != nil {
		return
	}
	d.Project.PanelGuide.OutboundTag = ob
	d.Project.PanelGuide.RouteUser = ru

	fmt.Fprintln(w)
	fmt.Fprintln(w, helpStyle.Render("  出口地区代码仅用于健康检查；留空则不校验地区。"))
	loc := p.OptionalLine("出口地区代码（留空跳过）")
	if p.Err() != nil {
		return
	}
	d.Project.Checks.ExitLocation = loc
	if d.Project.Checks.TestURL == "" {
		d.Project.Checks.TestURL = "https://ifconfig.me"
	}
	if d.Project.Checks.ExitCheckURL == "" {
		d.Project.Checks.ExitCheckURL = "https://api.ipify.org"
	}
	if d.Project.Checks.PublicIPCheckURL == "" {
		d.Project.Checks.PublicIPCheckURL = "https://api.ipify.org"
	}
	fmt.Fprintln(w)
}

func runSummaryMenu(w io.Writer, p *Prompter, d *SetupDraft) (done bool, act SetupAction) {
	for {
		fmt.Fprintln(w, renderPanelTitle("部署摘要"))
		printSummary(w, d.Project, d.RC)

		sub := []string{
			"开始部署",
			"重新运行预检（SSH / Cloudflare / DNS）",
			"运行位置",
			"入口节点",
			"出口节点",
			"Cloudflare",
			"域名",
			"出口管理 DDNS",
			"入口自动修复",
			"面板与检查",
			"重新走一遍 6 步引导",
			"保存并退出",
			"放弃（不保存）",
		}
		ch := p.Select("请选择:", sub)
		if p.Err() != nil {
			return false, ActionCancel
		}
		switch ch {
		case 0:
			if tryDeploy(w, p, d) {
				return true, ActionDeploy
			}
		case 1:
			runVerifySubmenu(w, p, d)
		case 2:
			stepRunLocation(w, p, d)
		case 3:
			stepEntryNode(w, p, d)
		case 4:
			stepExitNode(w, p, d)
		case 5:
			stepCloudflare(w, p, d)
		case 6:
			stepDomains(w, p, d)
		case 7:
			stepExitDDNS(w, p, d)
		case 8:
			stepEntryAuto(w, p, d)
		case 9:
			stepPanelHealth(w, p, d)
		case 10:
			return false, ActionCancel
		case 11:
			if trySave(w, d) {
				return true, ActionSaveOnly
			}
		case 12:
			fmt.Fprintln(w, "\n已放弃，未写入配置文件。")
			return true, ActionCancel
		}
	}
}

func trySave(w io.Writer, d *SetupDraft) bool {
	if err := config.Validate(d.Project); err == nil {
		if err := config.Save(config.DefaultPath, d.Project); err != nil {
			fmt.Fprintf(w, "\n保存失败：%v\n", err)
			return false
		}
		if err := os.Remove(config.DraftPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(w, "\n配置已写入 %s，但清理草稿失败：%v\n", config.DefaultPath, err)
			return false
		}
		fmt.Fprintf(w, "\n配置已保存。\n  正式配置: %s\n", config.DefaultPath)
		return true
	}

	if err := config.SaveDraft(config.DraftPath, d.Project); err != nil {
		fmt.Fprintf(w, "\n保存草稿失败：%v\n", err)
		return false
	}
	fmt.Fprintln(w, "\n当前设置还没填完，但进度已经保存。")
	fmt.Fprintln(w, "下次再次运行 wgstack 会自动继续。")
	return true
}

func tryDeploy(w io.Writer, p *Prompter, d *SetupDraft) bool {
	if err := config.Validate(d.Project); err != nil {
		fmt.Fprintf(w, "\n配置校验失败：%v\n", err)
		return false
	}
	if err := config.ValidateDeploy(d.Project, d.RC); err != nil {
		fmt.Fprintf(w, "\n部署前检查失败：%v\n", err)
		return false
	}
	if !runAutomaticPreflight(w, d) {
		return false
	}
	if !p.Confirm("\n确认保存到 wgstack.json 并开始部署？", true) {
		fmt.Fprintln(w, "已取消部署。")
		return false
	}
	if p.Err() != nil {
		return false
	}
	if err := config.Save(config.DefaultPath, d.Project); err != nil {
		fmt.Fprintf(w, "\n保存失败：%v\n", err)
		return false
	}
	if err := os.Remove(config.DraftPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(w, "\n已写入 %s，但清理草稿失败：%v\n", config.DefaultPath, err)
		return false
	}
	return true
}

func runAutomaticPreflight(w io.Writer, d *SetupDraft) bool {
	fmt.Fprintln(w, renderPanelTitle("自动预检"))
	fmt.Fprintln(w, "正在验证 Cloudflare、自动创建/更新 DNS，并检查入口/出口 SSH。")
	if err := VerifyAll(w, d.Project, d.RC); err != nil {
		fmt.Fprintf(w, "\n自动预检失败：%v\n", err)
		fmt.Fprintln(w, "请在下面的摘要菜单里返回对应步骤修改；再次开始部署时会自动重试。")
		return false
	}
	return true
}
