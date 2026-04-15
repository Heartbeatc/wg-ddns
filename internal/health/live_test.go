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
