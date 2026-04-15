package cloudflare

import (
	"os"
	"testing"

	"wg-ddns/internal/model"
)

func TestFirstError(t *testing.T) {
	if got := firstError(nil); got != "unknown error" {
		t.Fatalf("firstError(nil) = %q", got)
	}
	if got := firstError([]responseError{{Message: "boom"}}); got != "boom" {
		t.Fatalf("firstError() = %q", got)
	}
}

func TestResolveTokenEnvPriority(t *testing.T) {
	const envKey = "TEST_CF_TOKEN_RESOLVE"
	os.Setenv(envKey, "env-token-value")
	defer os.Unsetenv(envKey)

	cfg := model.Cloudflare{
		Token:    "file-token-value",
		TokenEnv: envKey,
	}
	token, source, err := ResolveToken(cfg)
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "env-token-value" {
		t.Errorf("token = %q, want env-token-value", token)
	}
	if source != "环境变量 "+envKey {
		t.Errorf("source = %q, want env var source", source)
	}
}

func TestResolveTokenFallbackToFile(t *testing.T) {
	const envKey = "TEST_CF_TOKEN_EMPTY"
	os.Unsetenv(envKey)

	cfg := model.Cloudflare{
		Token:    "file-token-value",
		TokenEnv: envKey,
	}
	token, source, err := ResolveToken(cfg)
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "file-token-value" {
		t.Errorf("token = %q, want file-token-value", token)
	}
	if source != "配置文件 cloudflare.token" {
		t.Errorf("source = %q, want config file source", source)
	}
}

func TestResolveTokenBothEmpty(t *testing.T) {
	const envKey = "TEST_CF_TOKEN_NONE"
	os.Unsetenv(envKey)

	cfg := model.Cloudflare{
		Token:    "",
		TokenEnv: envKey,
	}
	_, _, err := ResolveToken(cfg)
	if err == nil {
		t.Fatal("ResolveToken() expected error when both empty")
	}
}

func TestResolveTokenTrimSpace(t *testing.T) {
	const envKey = "TEST_CF_TOKEN_SPACES"
	os.Setenv(envKey, "  trimmed-token  ")
	defer os.Unsetenv(envKey)

	cfg := model.Cloudflare{TokenEnv: envKey}
	token, _, err := ResolveToken(cfg)
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "trimmed-token" {
		t.Errorf("token = %q, want trimmed-token", token)
	}
}

func TestResolveTokenNoEnvKey(t *testing.T) {
	cfg := model.Cloudflare{
		Token:    "direct",
		TokenEnv: "",
	}
	token, _, err := ResolveToken(cfg)
	if err != nil {
		t.Fatalf("ResolveToken() error = %v", err)
	}
	if token != "direct" {
		t.Errorf("token = %q, want direct", token)
	}
}
