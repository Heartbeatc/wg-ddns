package health

import (
	"testing"

	"wg-ddns/internal/model"
)

func TestIsPublicIPv4(t *testing.T) {
	cases := []struct {
		input string
		want  bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"203.0.113.1", true},
		{"100.100.100.100", false},
		{"100.64.0.1", false},
		{"100.127.255.254", false},
		{"10.0.0.1", false},
		{"10.255.255.255", false},
		{"172.16.0.1", false},
		{"172.31.255.255", false},
		{"172.15.0.1", true},
		{"172.32.0.1", true},
		{"192.168.0.1", false},
		{"192.168.255.255", false},
		{"127.0.0.1", false},
		{"127.1.2.3", false},
		{"169.254.0.1", false},
		{"169.254.255.255", false},
		{"0.0.0.0", false},
		{"0.1.2.3", false},
		{"224.0.0.1", false},
		{"239.255.255.255", false},
		{"240.0.0.1", false},
		{"255.255.255.255", false},
		{"::1", false},
		{"2001:db8::1", false},
		{"not-an-ip", false},
		{"", false},
		{"192.167.1.1", true},
		{"100.63.255.255", true},
		{"11.0.0.1", true},
		{"99.99.99.99", true},
	}

	for _, tc := range cases {
		got := IsPublicIPv4(tc.input)
		if got != tc.want {
			t.Errorf("IsPublicIPv4(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseUnix(t *testing.T) {
	if v, err := parseUnix("1700000000"); err != nil || v != 1700000000 {
		t.Fatalf("parseUnix(\"1700000000\") = (%d, %v)", v, err)
	}
	if _, err := parseUnix("abc"); err == nil {
		t.Fatal("parseUnix(\"abc\") expected error")
	}
}

func TestExpectedEmptyExitLocation(t *testing.T) {
	project := testProject("")
	checks := Expected(project)
	for _, c := range checks {
		if c.Name == "出口验证" {
			if c.Detail == "" {
				t.Fatal("Expected non-empty detail for egress check with empty ExitLocation")
			}
			return
		}
	}
	t.Fatal("Expected to find 出口验证 check")
}

func TestExpectedWithExitLocation(t *testing.T) {
	project := testProject("JP")
	checks := Expected(project)
	for _, c := range checks {
		if c.Name == "出口验证" {
			if c.Detail == "" {
				t.Fatal("Expected non-empty detail for egress check with JP ExitLocation")
			}
			return
		}
	}
	t.Fatal("Expected to find 出口验证 check")
}

func TestParseSvcStatusSuccess(t *testing.T) {
	output := "Result=success\nExecMainStatus=0\n"
	result, exitCode := parseSvcStatus(output)
	if result != "success" {
		t.Fatalf("result = %q, want %q", result, "success")
	}
	if exitCode != "0" {
		t.Fatalf("exitCode = %q, want %q", exitCode, "0")
	}
}

func TestParseSvcStatusFailure(t *testing.T) {
	output := "Result=exit-code\nExecMainStatus=1\n"
	result, exitCode := parseSvcStatus(output)
	if result != "exit-code" {
		t.Fatalf("result = %q, want %q", result, "exit-code")
	}
	if exitCode != "1" {
		t.Fatalf("exitCode = %q, want %q", exitCode, "1")
	}
}

func TestParseSvcStatusEmpty(t *testing.T) {
	result, exitCode := parseSvcStatus("")
	if result != "" || exitCode != "" {
		t.Fatalf("expected empty values, got result=%q exitCode=%q", result, exitCode)
	}
}

func TestParseSvcStatusPartial(t *testing.T) {
	output := "Result=exit-code\n"
	result, exitCode := parseSvcStatus(output)
	if result != "exit-code" {
		t.Fatalf("result = %q, want %q", result, "exit-code")
	}
	if exitCode != "" {
		t.Fatalf("exitCode = %q, want empty", exitCode)
	}
}

func testProject(exitLocation string) model.Project {
	return model.Project{
		Project: "test",
		Domains: model.Domains{
			Entry:     "entry.example.com",
			Panel:     "panel.example.com",
			WireGuard: "entry.example.com",
		},
		Nodes: model.Nodes{
			US: model.Node{
				Host:      "1.2.3.4",
				WGAddress: "10.66.66.1/24",
			},
			HK: model.Node{
				Host:        "5.6.7.8",
				WGAddress:   "10.66.66.2/24",
				SocksListen: "10.66.66.2:10808",
			},
		},
		Checks: model.HealthCheck{
			TestURL:      "https://ifconfig.me",
			ExitCheckURL: "https://ifconfig.me/country_code",
			ExitLocation: exitLocation,
		},
	}
}
