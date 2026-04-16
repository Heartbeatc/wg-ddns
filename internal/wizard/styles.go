package wizard

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent  = lipgloss.Color("#59E1C5")
	colorAccent2 = lipgloss.Color("#7BC6FF")
	colorAccent3 = lipgloss.Color("#B58CFF")
	colorMuted   = lipgloss.Color("#7E8A97")
	colorText    = lipgloss.Color("#F3F6F8")
	colorTextDim = lipgloss.Color("#C7D1D8")
	colorBorder  = lipgloss.Color("#2D3943")
	colorSuccess = lipgloss.Color("#7EE081")
	colorWarn    = lipgloss.Color("#F5C26B")

	welcomeBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
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
			Background(colorAccent2).
			Padding(0, 1)

	tabIdleStyle = lipgloss.NewStyle().
			Foreground(colorTextDim).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	activeTrailStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)
)

func renderWelcome(path string) string {
	brand := lipgloss.JoinVertical(
		lipgloss.Left,
		logoStyle.Render("wgstack"),
		logoSubStyle.Render("Entry / Exit Deploy"),
		helpStyle.Render("WireGuard · sing-box · Cloudflare"),
	)
	meta := lipgloss.JoinVertical(
		lipgloss.Left,
		metaStyle.Render(path),
		softBodyStyle.Render("6 steps · root SSH · Cloudflare Token · 1 个入口业务域名"),
	)
	return welcomeBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, brand, "", meta))
}

func renderProgress(step, total int, title, hint string) string {
	head := lipgloss.JoinHorizontal(
		lipgloss.Center,
		badgeStyle.Render(fmt.Sprintf("步骤 %d/%d", step, total)),
		"  ",
		screenTitleStyle.Render("当前步骤"),
		"  ",
		bodyStyle.Render(title),
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
