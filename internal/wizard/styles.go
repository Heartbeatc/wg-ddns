package wizard

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	colorAccent  = lipgloss.Color("#59E1C5")
	colorAccent2 = lipgloss.Color("#7BC6FF")
	colorMuted   = lipgloss.Color("#7E8A97")
	colorText    = lipgloss.Color("#F3F6F8")
	colorBorder  = lipgloss.Color("#2D3943")
	colorSuccess = lipgloss.Color("#7EE081")
	colorWarn    = lipgloss.Color("#F5C26B")

	welcomeBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			MarginBottom(1)

	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorAccent).
			PaddingRight(2)

	logoSubStyle = lipgloss.NewStyle().
			Foreground(colorAccent2).
			Bold(true)

	metaStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	bodyStyle = lipgloss.NewStyle().
			Foreground(colorText)

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
)

func renderWelcome(path string) string {
	logo := lipgloss.JoinVertical(lipgloss.Left,
		logoStyle.Render("▗▄▖  wgstack"),
		logoSubStyle.Render("▐▌▐▌ Entry / Exit Deploy"),
		logoSubStyle.Render("▝▚▄▞▘ WireGuard · sing-box · Cloudflare"),
	)
	meta := lipgloss.JoinVertical(lipgloss.Left,
		metaStyle.Render(path),
		"",
		bodyStyle.Render("自动完成: 建立 WG 隧道、部署出口 SOCKS、下发配置、启动服务"),
		bodyStyle.Render("需要准备: 入口/出口 root SSH、Cloudflare Token、一个入口业务域名"),
		helpStyle.Render("接下来会用 6 步完成配置；中途离开，下次会自动继续。"),
	)
	return welcomeBoxStyle.Render(lipgloss.JoinVertical(lipgloss.Left, logo, "", meta))
}

func renderProgress(step, total int, title, hint string) string {
	barWidth := 24
	done := step * barWidth / total
	if done < 1 {
		done = 1
	}
	if done > barWidth {
		done = barWidth
	}

	bar := progressDoneStyle.Render(repeat("━", done)) +
		progressTodoStyle.Render(repeat("━", barWidth-done))

	head := lipgloss.JoinHorizontal(
		lipgloss.Center,
		badgeStyle.Render(fmt.Sprintf("步骤 %d/%d", step, total)),
		"  ",
		screenTitleStyle.Render(title),
	)
	if hint == "" {
		return lipgloss.JoinVertical(lipgloss.Left, "", head, bar)
	}
	return lipgloss.JoinVertical(lipgloss.Left, "", head, bar, helpStyle.Render(hint))
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
	return "\n" + sectionTitleStyle.Render(title)
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
