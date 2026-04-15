package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	err := Save(path, File{
		Version:          1,
		LastReconciledAt: time.Unix(1, 0).UTC(),
		LastObservedUSIP: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Save() wrote empty file")
	}
}
