package notify

import (
	"fmt"
	"strings"
	"time"
)

// ProbeInfo is a notification-layer mirror of health.Probe,
// kept separate to avoid circular imports.
type ProbeInfo struct {
	Name     string
	Status   string
	Detail   string
	Duration time.Duration
}

// IPInfo holds enriched IP metadata from an external lookup.
type IPInfo struct {
	Country     string
	CountryCode string
	City        string
	ISP         string
	Org         string
	AS          string
}

// FormatApplySuccess formats a deploy-success notification.
func FormatApplySuccess(project, entryHost, exitHost string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[部署完成] %s\n\n", project)
	fmt.Fprintf(&b, "入口节点: %s\n", entryHost)
	fmt.Fprintf(&b, "出口节点: %s\n", exitHost)
	b.WriteString("\n部署成功完成。")
	return b.String()
}

// FormatApplyFailure formats a deploy-failure notification.
func FormatApplyFailure(project, errMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[部署失败] %s\n\n", project)
	fmt.Fprintf(&b, "错误: %s", errMsg)
	return b.String()
}

// FormatReconcileSuccess formats a reconcile-success notification.
// ipInfo may be nil if enrichment was skipped or failed.
func FormatReconcileSuccess(project, entryIP string, changes []string, probes []ProbeInfo, ipInfo *IPInfo) string {
	var b strings.Builder
	ipChanged := len(changes) > 0

	if ipChanged {
		fmt.Fprintf(&b, "[入口 IP 变化] %s\n\n", project)
	} else {
		fmt.Fprintf(&b, "[DNS 同步完成] %s\n\n", project)
	}
	fmt.Fprintf(&b, "入口 IP: %s\n", entryIP)

	if ipChanged {
		b.WriteString("\nDNS 更新:\n")
		for _, c := range changes {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}

	if ipInfo != nil {
		b.WriteString("\nIP 信息 (via ipinfo.io): ")
		parts := []string{}
		if ipInfo.CountryCode != "" {
			parts = append(parts, ipInfo.CountryCode)
		}
		if ipInfo.City != "" {
			parts = append(parts, ipInfo.City)
		}
		if ipInfo.ISP != "" {
			parts = append(parts, ipInfo.ISP)
		}
		b.WriteString(strings.Join(parts, " / "))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "详细质量查询: https://iplark.com/%s\n", entryIP)

	if len(probes) > 0 {
		b.WriteString("\n")
		writeProbes(&b, probes)
	}

	b.WriteString("\nreconcile 执行成功")
	return b.String()
}

// FormatReconcileFailure formats a reconcile-failure notification.
func FormatReconcileFailure(project, errMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[reconcile 失败] %s\n\n", project)
	fmt.Fprintf(&b, "错误: %s", errMsg)
	return b.String()
}

// FormatHealthFailure formats a notification for health check failures.
func FormatHealthFailure(project string, probes []ProbeInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[健康检查失败] %s\n\n", project)
	writeProbes(&b, probes)
	return b.String()
}

// FormatHealthRunError formats a notification for when the health check
// command itself fails before any probes can run (e.g. SSH connection failure,
// public IP detection failure).
func FormatHealthRunError(project, errMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[健康检查执行失败] %s\n\n", project)
	fmt.Fprintf(&b, "健康检查未能完成，未进入探测阶段。\n\n")
	fmt.Fprintf(&b, "错误: %s", errMsg)
	return b.String()
}

func writeProbes(b *strings.Builder, probes []ProbeInfo) {
	for _, p := range probes {
		dur := ""
		if p.Duration > 0 {
			dur = fmt.Sprintf(" (%s)", p.Duration.Round(time.Millisecond))
		}
		fmt.Fprintf(b, "- [%s] %s: %s%s\n", p.Status, p.Name, p.Detail, dur)
	}
}
