package render

import (
	"encoding/base64"
	"strings"
	"testing"

	"wg-ddns/internal/config"
)

func validTestKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestGenerate(t *testing.T) {
	project := config.DefaultProject()
	k := validTestKey()
	project.Nodes.US.WGPrivateKey = k
	project.Nodes.US.WGPublicKey = k
	project.Nodes.HK.WGPrivateKey = k
	project.Nodes.HK.WGPublicKey = k

	files, err := Generate(project)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(files) != 3 {
		t.Fatalf("Generate() rendered %d files, want 3", len(files))
	}

	found := false
	for _, file := range files {
		if file.Path == "out/hk/sing-box.json" {
			found = true
			if !strings.Contains(file.Content, `"listen": "10.66.66.2"`) {
				t.Fatalf("sing-box config missing listen IP: %s", file.Content)
			}
			if !strings.Contains(file.Content, `"listen_port": 10808`) {
				t.Fatalf("sing-box config missing listen port: %s", file.Content)
			}
		}
	}

	if !found {
		t.Fatal("Generate() did not render out/hk/sing-box.json")
	}
}

func TestGenerateRejectsEmptyKeys(t *testing.T) {
	project := config.DefaultProject()
	// Keys are empty by default
	_, err := Generate(project)
	if err == nil {
		t.Fatal("Generate() expected error for empty WG keys")
	}
}
