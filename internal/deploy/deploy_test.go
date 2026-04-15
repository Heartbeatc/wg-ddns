package deploy

import (
	"testing"

	"wg-ddns/internal/config"
)

func TestBuildFiles(t *testing.T) {
	files, err := BuildFiles(config.DefaultProject())
	if err != nil {
		t.Fatalf("BuildFiles() error = %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("BuildFiles() returned %d files, want 3", len(files))
	}

	want := map[string]string{
		"us:/etc/wireguard/wg0.conf":   "0600",
		"hk:/etc/wireguard/wg0.conf":   "0600",
		"hk:/etc/sing-box/config.json": "0644",
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
