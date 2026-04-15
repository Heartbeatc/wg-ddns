package address

import "testing"

func TestCIDRIP(t *testing.T) {
	if got := CIDRIP("10.66.66.2/24"); got != "10.66.66.2" {
		t.Fatalf("CIDRIP() = %q, want %q", got, "10.66.66.2")
	}
}

func TestHostAndPort(t *testing.T) {
	if got := Host("10.66.66.2:10808"); got != "10.66.66.2" {
		t.Fatalf("Host() = %q, want %q", got, "10.66.66.2")
	}
	if got := Port("10.66.66.2:10808"); got != "10808" {
		t.Fatalf("Port() = %q, want %q", got, "10808")
	}
}
