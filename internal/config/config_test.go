package config

import (
	"encoding/base64"
	"testing"
)

// validTestKey returns a valid base64-encoded 32-byte WireGuard key for testing.
func validTestKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, 32))
}

func TestValidateDefaultProject(t *testing.T) {
	if err := Validate(DefaultProject()); err != nil {
		t.Fatalf("Validate(DefaultProject()) error = %v", err)
	}
}

func TestValidateMissingFields(t *testing.T) {
	project := DefaultProject()
	project.Domains.Entry = ""

	if err := Validate(project); err == nil {
		t.Fatal("Validate() expected error for missing domains.entry")
	}
}

func TestValidateDeploy(t *testing.T) {
	project := DefaultProject()
	k := validTestKey()
	project.Nodes.US.WGPrivateKey = k
	project.Nodes.US.WGPublicKey = k
	project.Nodes.HK.WGPrivateKey = k
	project.Nodes.HK.WGPublicKey = k

	if err := ValidateDeploy(project); err != nil {
		t.Fatalf("ValidateDeploy() error = %v", err)
	}
}

func TestValidateDeployMissingAuth(t *testing.T) {
	project := DefaultProject()
	project.Nodes.US.SSH.PrivateKeyPath = ""

	if err := ValidateDeploy(project); err == nil {
		t.Fatal("ValidateDeploy() expected error for missing private key path")
	}
}

func TestValidateDeployMissingWGKeys(t *testing.T) {
	project := DefaultProject()
	// Keys are empty by default in DefaultProject
	if err := ValidateDeploy(project); err == nil {
		t.Fatal("ValidateDeploy() expected error for missing WG keys")
	}
}

func TestValidateDeployInvalidWGKey(t *testing.T) {
	project := DefaultProject()
	project.Nodes.US.WGPrivateKey = "not-valid-base64!!!"
	project.Nodes.US.WGPublicKey = validTestKey()
	project.Nodes.HK.WGPrivateKey = validTestKey()
	project.Nodes.HK.WGPublicKey = validTestKey()

	if err := ValidateDeploy(project); err == nil {
		t.Fatal("ValidateDeploy() expected error for invalid WG key format")
	}
}

func TestValidateCloudflareTTL(t *testing.T) {
	project := DefaultProject()
	project.Cloudflare.TTL = 0

	if err := Validate(project); err == nil {
		t.Fatal("Validate() expected error for invalid cloudflare ttl")
	}
}
