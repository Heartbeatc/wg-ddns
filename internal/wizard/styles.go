package wizard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent  = lipgloss.Color("#63E6BE")
	colorAccent2 = lipgloss.Color("#8FD8FF")
	colorAccent3 = lipgloss.Color("#A7B0BA")
	colorMuted   = lipgloss.Color("#7C8794")
	colorText    = lipgloss.Color("#F2F5F7")
	colorTextDim = lipgloss.Color("#B9C1CA")
	colorBorder  = lipgloss.Color("#26313A")
	colorSuccess = lipgloss.Color("#8FE388")
	colorWarn    = lipgloss.Color("#F3C96B")
	colorPanel   = lipgloss.Color("#121A22")

	welcomeBoxStyle = lipgloss.NewStyle().
			PaddingTop(1).
			PaddingBottom(1).
			MarginBottom(1)

	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			MarginRight(1)

	logoSubStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	metaStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	bodyStyle = lipgloss.NewStyle().
			Foreground(colorText)

	softBodyStyle = lipgloss.NewStyle().
			Foreground(colorTextDim)

	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
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
			Background(colorPanel).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	tabIdleStyle = lipgloss.NewStyle().
			Foreground(colorTextDim).
			Padding(0, 1)

	activeTrailStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)
)

func renderWelcome(path string) string {
	title := lipgloss.JoinHorizontal(
		lipgloss.Center,
		logoStyle.Render("▌"),
		logoSubStyle.Render("wgstack"),
		helpStyle.Render("  Entry / Exit Deploy"),
	)
	meta := lipgloss.JoinVertical(
		lipgloss.Left,
		metaStyle.Render(path),
		softBodyStyle.Render("6 steps · WireGuard tunnel · Exit SOCKS · Cloudflare DNS"),
		helpStyle.Render("准备：入口/出口 root SSH、Cloudflare Token、1 个入口业务域名"),
	)
	return welcomeBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, meta))
}

func renderProgress(step, total int, title, hint string) string {
	steps := []string{"运行位置", "入口节点", "出口节点", "Cloudflare", "域名", "自动化"}
	tabs := make([]string, 0, len(steps))
	for i, s := range steps {
		n := i + 1
		switch {
		case n < step:
			tabs = append(tabs, progressDoneStyle.Render("✓ "+s))
		case n == step:
			tabs = append(tabs, tabActiveStyle.Render(fmt.Sprintf("%02d %s", n, s)))
		default:
			tabs = append(tabs, tabIdleStyle.Render(fmt.Sprintf("· %s", s)))
		}
	}
	head := lipgloss.JoinHorizontal(
		lipgloss.Center,
		badgeStyle.Render(fmt.Sprintf("步骤 %d/%d", step, total)),
		bodyStyle.Render(" · "),
		screenTitleStyle.Render(title),
	)
	bar := renderProgressBar(step, total, 34)
	row := lipgloss.JoinHorizontal(lipgloss.Left, tabs...)
	if hint == "" {
		return lipgloss.JoinVertical(lipgloss.Left, "", head, bar, row)
	}
	return lipgloss.JoinVertical(lipgloss.Left, "", head, bar, row, helpStyle.Render(hint))
}

func renderPanelTitle(title string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		screenTitleStyle.Render(title),
		helpStyle.Render(repeat("─", 24)),
	)
}

func renderSectionTitle(title string) string {
	return "\n" + sectionTitleStyle.Render("▌ "+title)
}

func renderProgressBar(step, total, width int) string {
	if total <= 0 {
		return ""
	}
	if step < 1 {
		step = 1
	}
	if step > total {
		step = total
	}
	filled := width * step / total
	if filled < 1 {
		filled = 1
	}
	return progressDoneStyle.Render(repeat("━", filled)) + progressTodoStyle.Render(repeat("━", width-filled))
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}
