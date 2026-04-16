package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseTagForRef(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"", "edge"},
		{"main", "edge"},
		{"v0.1.0", "v0.1.0"},
		{"dev", "dev"},
	}
	for _, tt := range tests {
		if got := releaseTagForRef(tt.ref); got != tt.want {
			t.Fatalf("releaseTagForRef(%q)=%q want %q", tt.ref, got, tt.want)
		}
	}
}

func TestDisplayRef(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"", "main/latest"},
		{"main", "main/latest"},
		{"v0.1.0", "v0.1.0"},
		{"dev", "dev"},
	}
	for _, tt := range tests {
		if got := displayRef(tt.ref); got != tt.want {
			t.Fatalf("displayRef(%q)=%q want %q", tt.ref, got, tt.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	got := assetName("edge", "linux", "amd64")
	if got != "wgstack_edge_linux_amd64.tar.gz" {
		t.Fatalf("assetName()=%q", got)
	}
}

func TestReleaseURL(t *testing.T) {
	got := releaseURL("Heartbeatc", "wg-ddns", "edge", "wgstack_edge_linux_amd64.tar.gz")
	want := "https://github.com/Heartbeatc/wg-ddns/releases/download/edge/wgstack_edge_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("releaseURL()=%q want %q", got, want)
	}
}

func TestExtractBinaryTarGz(t *testing.T) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	content := []byte("fake-binary")
	if err := tw.WriteHeader(&tar.Header{Name: "wgstack", Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "wgstack")
	if err := extractBinaryTarGz(bytes.NewReader(buf.Bytes()), "wgstack", target); err != nil {
		t.Fatalf("extractBinaryTarGz() error = %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Fatalf("binary content = %q want %q", got, content)
	}
}
