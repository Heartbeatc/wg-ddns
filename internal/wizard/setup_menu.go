package wizard

import (
	"errors"
	"fmt"
	"io"
	"os"
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
	fmt.Fprint(w, "\n按回车进入配置菜单...")
	p.WaitEnter()

	for {
		if p.Err() != nil {
			return nil, p.Err()
		}

		fmt.Fprintln(w, "\n========================================")
		fmt.Fprintln(w, "  配置菜单（可任意顺序修改，完成后保存或部署）")
		fmt.Fprintln(w, "========================================")
		fmt.Fprintln(w)

		opts := []string{
			"运行位置 — " + draft.statusRunLocation(),
			"入口节点 — " + draft.statusEntry(),
			"出口节点 — " + draft.statusExit(),
			"Cloudflare — " + draft.statusCloudflare(),
			"域名 — " + draft.statusDomains(),
			"出口管理 DDNS — " + draft.statusExitDDNS(),
			"入口自动修复 — " + draft.statusEntryAuto(),
			"面板与检查 — " + draft.statusPanel(),
			"逐项验证（SSH / Cloudflare / 域名）",
			"查看部署摘要（可从摘要跳转修改）",
			"开始部署（校验后保存并部署）",
			"保存并退出（完整配置写入 wgstack.json，未完成内容写入草稿）",
			"放弃（不保存）",
		}

		ch := p.Select("请选择一项:", opts)
		if p.Err() != nil {
			return nil, p.Err()
		}

		switch ch {
		case 0:
			stepRunLocation(w, p, draft)
		case 1:
			stepEntryNode(w, p, draft)
		case 2:
			stepExitNode(w, p, draft)
		case 3:
			stepCloudflare(w, p, draft)
		case 4:
			stepDomains(w, p, draft)
		case 5:
			stepExitDDNS(w, p, draft)
		case 6:
			stepEntryAuto(w, p, draft)
		case 7:
			stepPanelHealth(w, p, draft)
		case 8:
			runVerifySubmenu(w, p, draft)
		case 9:
			if done, act := runSummaryMenu(w, p, draft); done {
				return finalizeSetupResult(draft, act)
			}
		case 10:
			if tryDeploy(w, p, draft) {
				return finalizeSetupResult(draft, ActionDeploy)
			}
		case 11:
			if trySave(w, draft) {
				return finalizeSetupResult(draft, ActionSaveOnly)
			}
		case 12:
			fmt.Fprintln(w, "\n已放弃，未写入配置文件。")
			return &SetupResult{Action: ActionCancel}, nil
		}
	}
}

func finalizeSetupResult(d *SetupDraft, act SetupAction) (*SetupResult, error) {
	return &SetupResult{
		Action:  act,
		Project: d.Project,
		RC:      d.RC,
	}, nil
}

func stepRunLocation(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, "\n--- 运行位置 ---")
	fmt.Fprintln(w, "  本工具通过 SSH 远程配置目标节点；若在某台目标节点本机运行，可跳过该节点 SSH。")
	idx := p.Select("你当前在哪台机器上运行 wgstack？", RunLocationOptions)
	if p.Err() != nil {
		return
	}
	d.RC = model.RunContext{
		EntryIsLocal: idx == 1,
		ExitIsLocal:  idx == 2,
	}
	d.RCSet = true
	fmt.Fprintln(w, "  已保存（内存中，尚未写入文件）。")
}

func stepEntryNode(w io.Writer, p *Prompter, d *SetupDraft) {
	if !d.RCSet {
		fmt.Fprintln(w, "\n请先配置「运行位置」。")
		return
	}
	fmt.Fprintln(w, "\n--- 入口节点 ---")
	ni := collectNodeInfoWithDefaults(w, p, "入口节点", d.RC.EntryIsLocal, d.Project.Nodes.US)
	if p.Err() != nil {
		return
	}
	applyNodeInput(&d.Project.Nodes.US, ni)
	fmt.Fprintln(w, "  已保存（内存中）。")
}

func stepExitNode(w io.Writer, p *Prompter, d *SetupDraft) {
	if !d.RCSet {
		fmt.Fprintln(w, "\n请先配置「运行位置」。")
		return
	}
	fmt.Fprintln(w, "\n--- 出口节点 ---")
	ni := collectNodeInfoWithDefaults(w, p, "出口节点", d.RC.ExitIsLocal, d.Project.Nodes.HK)
	if p.Err() != nil {
		return
	}
	applyNodeInput(&d.Project.Nodes.HK, ni)
	fmt.Fprintln(w, "  已保存（内存中）。")
}

func applyNodeInput(n *model.Node, ni nodeInput) {
	n.Host = ni.host
	n.SSHHost = ni.sshHost
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
	fmt.Fprintln(w, "\n--- Cloudflare ---")
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
	fmt.Fprintln(w, "  已保存（内存中）。")
}

func stepDomains(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, "\n--- 域名 ---")
	cfZone := strings.TrimSpace(d.Project.Cloudflare.Zone)
	if cfZone == "" {
		fmt.Fprintln(w, "  建议先配置 Cloudflare Zone，以便使用合理默认。")
	}

	fmt.Fprintln(w, "  多数情况下面板与代理入口共用同一域名，仅端口不同。")
	entryDef := strings.TrimSpace(d.Project.Domains.Entry)
	if entryDef == "" {
		entryDef = cfZone
	}
	entryDomain := p.LineWith("对外域名（面板和代理共用）", entryDef, validateDomain)
	if p.Err() != nil {
		return
	}

	panelSeparate := p.Confirm("面板访问域名是否与此不同？", false)
	var panelDomain string
	if panelSeparate {
		pd := strings.TrimSpace(d.Project.Domains.Panel)
		if pd == "" || pd == entryDomain {
			pd = "panel." + cfZone
		}
		panelDomain = p.LineWith("面板域名", pd, validateDomain)
	} else {
		panelDomain = entryDomain
	}
	if p.Err() != nil {
		return
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  WireGuard Endpoint 默认复用对外域名；单独填写也应为域名。")
	wgSeparate := p.Confirm("WireGuard Endpoint 使用单独域名？", false)
	var wgDomain string
	if wgSeparate {
		wd := strings.TrimSpace(d.Project.Domains.WireGuard)
		if wd == "" || wd == entryDomain {
			wd = "wg." + cfZone
		}
		wgDomain = p.LineWith("WireGuard Endpoint 域名", wd, validateDomain)
	} else {
		wgDomain = entryDomain
	}
	if p.Err() != nil {
		return
	}

	d.Project.Domains = model.Domains{
		Entry:     entryDomain,
		Panel:     panelDomain,
		WireGuard: wgDomain,
	}
	fmt.Fprintln(w, "  已保存（内存中）。")
}

func stepExitDDNS(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, "\n--- 出口管理 DDNS ---")
	fmt.Fprintln(w, "  家宽等动态公网 IP 场景下，可在出口节点自动维护 SSH 管理域名。")
	d.ExitDDNSTouched = true

	cfZone := strings.TrimSpace(d.Project.Cloudflare.Zone)
	if cfZone == "" {
		fmt.Fprintln(w, "  提示：尚未填写 Cloudflare Zone，默认域名后缀可能不完整。")
	}

	dynamic := p.Confirm("出口节点公网 IP 是否可能变化（如家宽动态 IP）？", d.Project.ExitDDNS.Enabled)
	if !dynamic {
		d.Project.ExitDDNS = model.ExitDDNS{Enabled: false, Interval: 300}
		fmt.Fprintln(w, "  已关闭出口管理 DDNS（内存中）。")
		return
	}

	enable := p.Confirm("启用出口 SSH 管理域名 DDNS？", true)
	if !enable {
		d.Project.ExitDDNS = model.ExitDDNS{Enabled: false, Interval: 300}
		fmt.Fprintln(w, "  已关闭（内存中）。")
		return
	}

	ddDef := strings.TrimSpace(d.Project.ExitDDNS.Domain)
	if ddDef == "" {
		ddDef = "ssh-exit." + cfZone
	}
	dd := p.LineWith("出口 SSH 管理域名", ddDef, validateDomain)
	if p.Err() != nil {
		return
	}
	d.Project.ExitDDNS = model.ExitDDNS{Enabled: true, Domain: dd, Interval: 300}
	d.Project.Nodes.HK.SSHHost = dd
	fmt.Fprintln(w, "  已启用，并已将出口 ssh_host 设为该域名（内存中）。")
}

func stepEntryAuto(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, "\n--- 入口自动修复 ---")
	fmt.Fprintln(w, "  入口节点定时检测 IP / DNS 漂移并自动修复。")
	d.EntryAutoTouched = true

	en := p.Confirm("启用入口节点 IP 与 DNS 自动修复？", d.Project.EntryAutoReconcile.Enabled)
	if en {
		d.Project.EntryAutoReconcile = model.AutoReconcile{Enabled: true, Interval: 300}
		fmt.Fprintln(w, "  已启用（默认间隔 300s，内存中）。")
	} else {
		d.Project.EntryAutoReconcile = model.AutoReconcile{Enabled: false, Interval: 300}
		fmt.Fprintln(w, "  已关闭（内存中）。")
	}
}

func stepPanelHealth(w io.Writer, p *Prompter, d *SetupDraft) {
	fmt.Fprintln(w, "\n--- 面板与检查 ---")
	ob := p.LineWith("面板出站标签", d.Project.PanelGuide.OutboundTag, nil)
	ru := p.LineWith("专用线路用户标识", d.Project.PanelGuide.RouteUser, nil)
	if p.Err() != nil {
		return
	}
	d.Project.PanelGuide.OutboundTag = ob
	d.Project.PanelGuide.RouteUser = ru

	fmt.Fprintln(w)
	fmt.Fprintln(w, "  出口地区代码用于健康检查；留空则不校验地区。")
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
	fmt.Fprintln(w, "  已保存（内存中）。")
}

func runSummaryMenu(w io.Writer, p *Prompter, d *SetupDraft) (done bool, act SetupAction) {
	for {
		fmt.Fprintln(w, "\n========================================")
		fmt.Fprintln(w, "  部署摘要")
		fmt.Fprintln(w, "========================================")
		printSummary(w, d.Project, d.RC)

		sub := []string{
			"返回配置主菜单",
			"确认部署（校验后保存并部署）",
			"运行位置",
			"入口节点",
			"出口节点",
			"Cloudflare",
			"域名",
			"出口管理 DDNS",
			"入口自动修复",
			"面板与检查",
		}
		ch := p.Select("请选择:", sub)
		if p.Err() != nil {
			return false, ActionCancel
		}
		switch ch {
		case 0:
			return false, ActionCancel
		case 1:
			if tryDeploy(w, p, d) {
				return true, ActionDeploy
			}
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
		fmt.Fprintf(w, "\n配置已写入 %s\n", config.DefaultPath)
		return true
	}

	if err := config.SaveDraft(config.DraftPath, d.Project); err != nil {
		fmt.Fprintf(w, "\n保存草稿失败：%v\n", err)
		return false
	}
	fmt.Fprintf(w, "\n配置尚未完整，已将当前进度保存到 %s\n", config.DraftPath)
	fmt.Fprintln(w, "你可以稍后再次运行 wgstack 继续编辑。")
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
