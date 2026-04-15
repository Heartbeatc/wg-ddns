package health

import "testing"

func TestParseUnix(t *testing.T) {
	got, err := parseUnix("1713170000")
	if err != nil {
		t.Fatalf("parseUnix() error = %v", err)
	}
	if got != 1713170000 {
		t.Fatalf("parseUnix() = %d", got)
	}
}

func TestParseUnixRejectsNonDigits(t *testing.T) {
	if _, err := parseUnix("abc"); err == nil {
		t.Fatal("parseUnix() expected error")
	}
}

func TestIsPublicIPv4(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"1.2.3.4", true},
		{"8.8.8.8", true},
		{"203.0.113.1", true},
		{"45.76.100.200", true},

		// Private (RFC 1918)
		{"10.0.0.1", false},
		{"10.255.255.255", false},
		{"172.16.0.1", false},
		{"172.31.255.255", false},
		{"192.168.0.1", false},
		{"192.168.1.100", false},

		// Loopback
		{"127.0.0.1", false},
		{"127.255.255.255", false},

		// Link-local
		{"169.254.0.1", false},
		{"169.254.255.255", false},

		// CGNAT
		{"100.64.0.1", false},
		{"100.127.255.255", false},

		// Non-CGNAT 100.x
		{"100.63.255.255", true},
		{"100.128.0.0", true},

		// Multicast / reserved
		{"224.0.0.1", false},
		{"240.0.0.1", false},
		{"255.255.255.255", false},

		// Zero network
		{"0.0.0.0", false},

		// Not CGNAT but 172.x outside /12
		{"172.15.255.255", true},
		{"172.32.0.0", true},

		// Invalid inputs
		{"", false},
		{"not-an-ip", false},
		{"::1", false},
		{"2001:db8::1", false},
	}

	for _, tt := range tests {
		got := IsPublicIPv4(tt.ip)
		if got != tt.want {
			t.Errorf("IsPublicIPv4(%q) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}
