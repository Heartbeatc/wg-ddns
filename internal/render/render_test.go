package render

import (
	"strings"
	"testing"

	"wg-ddns/internal/config"
)

func TestGenerate(t *testing.T) {
	files, err := Generate(config.DefaultProject())
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
