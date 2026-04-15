package deploy

import (
	"encoding/base64"
	"testing"

	"wg-ddns/internal/config"
)

func validTestKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestBuildFiles(t *testing.T) {
	project := config.DefaultProject()
	k := validTestKey()
	project.Nodes.US.WGPrivateKey = k
	project.Nodes.US.WGPublicKey = k
	project.Nodes.HK.WGPrivateKey = k
	project.Nodes.HK.WGPublicKey = k

	files, err := BuildFiles(project)
	if err != nil {
		t.Fatalf("BuildFiles() error = %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("BuildFiles() returned %d files, want 3", len(files))
	}

	want := map[string]string{
		"entry:/etc/wireguard/wg0.conf":  "0600",
		"exit:/etc/wireguard/wg0.conf":   "0600",
		"exit:/etc/sing-box/config.json": "0644",
	}

	for _, file := range files {
		key := file.Node + ":" + file.Path
		mode, ok := want[key]
		if !ok {
			t.Fatalf("unexpected remote file %q", key)
		}
		if file.Mode != mode {
			t.Fatalf("remote file %q mode = %q, want %q", key, file.Mode, mode)
		}
		delete(want, key)
	}

	if len(want) != 0 {
		t.Fatalf("missing remote files: %#v", want)
	}
}
