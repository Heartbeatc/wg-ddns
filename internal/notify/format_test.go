package notify

import (
	"strings"
	"testing"
	"time"
)

func TestFormatApplySuccess(t *testing.T) {
	msg := FormatApplySuccess("my-project", "1.2.3.4", "5.6.7.8")
	if !strings.Contains(msg, "部署完成") {
		t.Fatal("should contain 部署完成")
	}
	if !strings.Contains(msg, "1.2.3.4") {
		t.Fatal("should contain entry host")
	}
	if !strings.Contains(msg, "5.6.7.8") {
		t.Fatal("should contain exit host")
	}
}

func TestFormatApplyFailure(t *testing.T) {
	msg := FormatApplyFailure("my-project", "connection refused")
	if !strings.Contains(msg, "部署失败") {
		t.Fatal("should contain 部署失败")
	}
	if !strings.Contains(msg, "connection refused") {
		t.Fatal("should contain error message")
	}
}

func TestFormatReconcileSuccessWithChanges(t *testing.T) {
	probes := []ProbeInfo{
		{Name: "DNS", Status: "PASS", Detail: "ok", Duration: 100 * time.Millisecond},
	}
	ipInfo := &IPInfo{CountryCode: "US", City: "San Jose", ISP: "DMIT"}
	msg := FormatReconcileSuccess("proj", "1.2.3.4", []string{"update a.com: 9.9.9.9 => 1.2.3.4"}, probes, ipInfo)

	if !strings.Contains(msg, "IP 变化") {
		t.Fatal("should contain IP 变化 when changes exist")
	}
	if !strings.Contains(msg, "1.2.3.4") {
		t.Fatal("should contain entry IP")
	}
	if !strings.Contains(msg, "via ipinfo.io") {
		t.Fatal("should attribute data source to ipinfo.io")
	}
	if !strings.Contains(msg, "US") {
		t.Fatal("should contain country code from ipinfo.io")
	}
	if !strings.Contains(msg, "iplark.com/1.2.3.4") {
		t.Fatal("should contain iplark quality link")
	}
	if !strings.Contains(msg, "100ms") {
		t.Fatal("should contain probe duration")
	}
}

func TestFormatReconcileSuccessNoEnrichment(t *testing.T) {
	msg := FormatReconcileSuccess("proj", "5.6.7.8", []string{"update a.com"}, nil, nil)
	if !strings.Contains(msg, "iplark.com/5.6.7.8") {
		t.Fatal("iplark link should appear even without enrichment data")
	}
	if strings.Contains(msg, "via ipinfo.io") {
		t.Fatal("should NOT show ipinfo.io attribution when enrichment is nil")
	}
}

func TestFormatReconcileSuccessNoChanges(t *testing.T) {
	msg := FormatReconcileSuccess("proj", "1.2.3.4", nil, nil, nil)
	if !strings.Contains(msg, "DNS 同步完成") {
		t.Fatal("should contain DNS 同步完成 when no changes")
	}
	if !strings.Contains(msg, "iplark.com/1.2.3.4") {
		t.Fatal("iplark link should always appear")
	}
}

func TestFormatReconcileFailure(t *testing.T) {
	msg := FormatReconcileFailure("proj", "API error")
	if !strings.Contains(msg, "reconcile 失败") {
		t.Fatal("should contain reconcile 失败")
	}
	if !strings.Contains(msg, "API error") {
		t.Fatal("should contain error message")
	}
}

func TestFormatHealthRunError(t *testing.T) {
	msg := FormatHealthRunError("proj", "SSH connection refused")
	if !strings.Contains(msg, "健康检查执行失败") {
		t.Fatal("should contain 健康检查执行失败")
	}
	if !strings.Contains(msg, "SSH connection refused") {
		t.Fatal("should contain error message")
	}
	if !strings.Contains(msg, "未进入探测阶段") {
		t.Fatal("should indicate probes didn't run")
	}
}

func TestFormatHealthFailure(t *testing.T) {
	probes := []ProbeInfo{
		{Name: "DNS", Status: "FAIL", Detail: "mismatch"},
		{Name: "WG", Status: "PASS", Detail: "ok", Duration: 50 * time.Millisecond},
	}
	msg := FormatHealthFailure("proj", probes)
	if !strings.Contains(msg, "健康检查失败") {
		t.Fatal("should contain 健康检查失败")
	}
	if !strings.Contains(msg, "FAIL") {
		t.Fatal("should contain FAIL status")
	}
	if !strings.Contains(msg, "PASS") {
		t.Fatal("should contain PASS status")
	}
}
