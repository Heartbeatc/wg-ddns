package wizard

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent  = lipgloss.Color("#59E1C5")
	colorAccent2 = lipgloss.Color("#7BC6FF")
	colorAccent3 = lipgloss.Color("#B58CFF")
	colorAccent4 = lipgloss.Color("#FF6FAE")
	colorMuted   = lipgloss.Color("#7E8A97")
	colorText    = lipgloss.Color("#F3F6F8")
	colorTextDim = lipgloss.Color("#C7D1D8")
	colorBorder  = lipgloss.Color("#2D3943")
	colorPanel   = lipgloss.Color("#171C24")
	colorSuccess = lipgloss.Color("#7EE081")
	colorWarn    = lipgloss.Color("#F5C26B")

	welcomeBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorPanel).
			Padding(1, 2).
			MarginBottom(1)

	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			PaddingRight(2)

	logoSubStyle = lipgloss.NewStyle().
			Foreground(colorAccent3).
			Bold(true)

	metaStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	bodyStyle = lipgloss.NewStyle().
			Foreground(colorText)

	softBodyStyle = lipgloss.NewStyle().
			Foreground(colorTextDim)

	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(colorAccent3).
				Bold(true)

	screenTitleStyle = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	progressDoneStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	progressTodoStyle = lipgloss.NewStyle().
				Foreground(colorBorder)

	badgeStyle = lipgloss.NewStyle().
			Foreground(colorAccent2).
			Bold(true)

	promptLabelStyle = lipgloss.NewStyle().
				Foreground(colorAccent2).
				Bold(true)

	optionIndexStyle = lipgloss.NewStyle().
				Foreground(colorAccent)

	optionTextStyle = lipgloss.NewStyle().
			Foreground(colorText)

	successTextStyle = lipgloss.NewStyle().
				Foreground(colorSuccess)

	warnTextStyle = lipgloss.NewStyle().
			Foreground(colorWarn)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorAccent3).
			Padding(0, 1)

	tabIdleStyle = lipgloss.NewStyle().
			Foreground(colorTextDim).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	chipStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorAccent4).
			Padding(0, 1)

	chipAltStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorAccent3).
			Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorBorder).
			Padding(0, 1)

	windowBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

func renderWelcome(path string) string {
	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		windowBarStyle.Render("● "),
		lipgloss.NewStyle().Foreground(colorWarn).Render("● "),
		lipgloss.NewStyle().Foreground(colorSuccess).Render("●"),
		"  ",
		metaStyle.Render("./wgstack"),
	)
	brand := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(
			lipgloss.Top,
			logoStyle.Render("▌"),
			" ",
			lipgloss.JoinVertical(
				lipgloss.Left,
				logoStyle.Render("wgstack"),
				logoSubStyle.Render("Entry / Exit Deploy"),
			),
		),
		helpStyle.Render("WireGuard · sing-box · Cloudflare"),
	)
	chips := lipgloss.JoinHorizontal(
		lipgloss.Left,
		chipStyle.Render("WG Tunnel"),
		" ",
		chipAltStyle.Render("Exit SOCKS"),
		" ",
		tabIdleStyle.Render("Auto Repair"),
	)
	meta := lipgloss.JoinVertical(
		lipgloss.Left,
		metaStyle.Render(path),
		softBodyStyle.Render("6 steps · 入口业务域名默认复用到面板和 WG"),
		statusBarStyle.Render("root SSH · Cloudflare Token · 1 个入口业务域名"),
	)
	return welcomeBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, header, "", brand, "", chips, "", meta))
}

func renderProgress(step, total int, title, hint string) string {
	head := lipgloss.JoinHorizontal(
		lipgloss.Center,
		badgeStyle.Render(fmt.Sprintf("步骤 %d/%d", step, total)),
		"  ",
		screenTitleStyle.Render(title),
	)
	steps := []string{"运行位置", "入口节点", "出口节点", "Cloudflare", "域名", "自动化"}
	tabs := make([]string, 0, len(steps))
	for i, s := range steps {
		if i == step-1 {
			tabs = append(tabs, tabActiveStyle.Render(s))
		} else {
			tabs = append(tabs, tabIdleStyle.Render(s))
		}
	}
	row := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	if hint == "" {
		return lipgloss.JoinVertical(lipgloss.Left, "", head, row)
	}
	return lipgloss.JoinVertical(lipgloss.Left, "", head, row, helpStyle.Render(hint))
}

func renderPanelTitle(title string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		screenTitleStyle.Render(title),
		helpStyle.Render(repeat("─", 28)),
	)
}

func renderSectionTitle(title string) string {
	return "\n" + sectionTitleStyle.Render("◆ "+title)
}

func renderSavedNote(text string) string {
	return successTextStyle.Render("  " + text)
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}
