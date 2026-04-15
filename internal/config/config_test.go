package config

import "testing"

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
	if err := ValidateDeploy(project); err != nil {
		t.Fatalf("ValidateDeploy(DefaultProject()) error = %v", err)
	}
}

func TestValidateDeployMissingAuth(t *testing.T) {
	project := DefaultProject()
	project.Nodes.US.SSH.PrivateKeyPath = ""

	if err := ValidateDeploy(project); err == nil {
		t.Fatal("ValidateDeploy() expected error for missing private key path")
	}
}

func TestValidateCloudflareTTL(t *testing.T) {
	project := DefaultProject()
	project.Cloudflare.TTL = 0

	if err := Validate(project); err == nil {
		t.Fatal("Validate() expected error for invalid cloudflare ttl")
	}
}
