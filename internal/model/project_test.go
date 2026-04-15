package model

import "testing"

func TestNodeSSHAddr(t *testing.T) {
	tests := []struct {
		name string
		node Node
		want string
	}{
		{
			name: "ssh_host set",
			node: Node{Host: "1.2.3.4", SSHHost: "ssh.example.com"},
			want: "ssh.example.com",
		},
		{
			name: "ssh_host empty falls back to host",
			node: Node{Host: "1.2.3.4"},
			want: "1.2.3.4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.SSHAddr(); got != tt.want {
				t.Errorf("SSHAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDomainsUnique(t *testing.T) {
	d := Domains{Entry: "a.com", Panel: "a.com", WireGuard: "b.com"}
	got := d.Unique()
	if len(got) != 2 {
		t.Errorf("Unique() returned %d items, want 2", len(got))
	}
}
