package cloudflare

import "testing"

func TestFirstError(t *testing.T) {
	if got := firstError(nil); got != "unknown error" {
		t.Fatalf("firstError(nil) = %q", got)
	}
	if got := firstError([]responseError{{Message: "boom"}}); got != "boom" {
		t.Fatalf("firstError() = %q", got)
	}
}
