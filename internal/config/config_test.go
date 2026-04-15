package config

import (
	"encoding/base64"
	"testing"

	"wg-ddns/internal/model"
)

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

	rc := model.RunContext{}
	if err := ValidateDeploy(project, rc); err != nil {
		t.Fatalf("ValidateDeploy() error = %v", err)
	}
}

func TestValidateDeployMissingAuth(t *testing.T) {
	project := DefaultProject()
	project.Nodes.US.SSH.PrivateKeyPath = ""

	rc := model.RunContext{}
	if err := ValidateDeploy(project, rc); err == nil {
		t.Fatal("ValidateDeploy() expected error for missing private key path")
	}
}

func TestValidateDeployMissingWGKeys(t *testing.T) {
	project := DefaultProject()
	rc := model.RunContext{}
	if err := ValidateDeploy(project, rc); err == nil {
		t.Fatal("ValidateDeploy() expected error for missing WG keys")
	}
}

func TestValidateDeployInvalidWGKey(t *testing.T) {
	project := DefaultProject()
	project.Nodes.US.WGPrivateKey = "not-valid-base64!!!"
	project.Nodes.US.WGPublicKey = validTestKey()
	project.Nodes.HK.WGPrivateKey = validTestKey()
	project.Nodes.HK.WGPublicKey = validTestKey()

	rc := model.RunContext{}
	if err := ValidateDeploy(project, rc); err == nil {
		t.Fatal("ValidateDeploy() expected error for invalid WG key format")
	}
}

func TestValidateDeployLocalEntrySkipsSSH(t *testing.T) {
	project := DefaultProject()
	k := validTestKey()
	project.Nodes.US.WGPrivateKey = k
	project.Nodes.US.WGPublicKey = k
	project.Nodes.HK.WGPrivateKey = k
	project.Nodes.HK.WGPublicKey = k

	project.Nodes.US.SSH.AuthMethod = ""
	project.Nodes.US.SSH.PrivateKeyPath = ""

	rc := model.RunContext{EntryIsLocal: true}
	if err := ValidateDeploy(project, rc); err != nil {
		t.Fatalf("ValidateDeploy() with local entry node error = %v", err)
	}
}

func TestValidateDeployRemoteEntryRequiresSSH(t *testing.T) {
	project := DefaultProject()
	k := validTestKey()
	project.Nodes.US.WGPrivateKey = k
	project.Nodes.US.WGPublicKey = k
	project.Nodes.HK.WGPrivateKey = k
	project.Nodes.HK.WGPublicKey = k

	project.Nodes.US.SSH.AuthMethod = ""
	project.Nodes.US.SSH.PrivateKeyPath = ""

	rc := model.RunContext{} // not local
	if err := ValidateDeploy(project, rc); err == nil {
		t.Fatal("ValidateDeploy() expected error for remote entry node with no SSH auth")
	}
}

func TestValidateCloudflareTTL(t *testing.T) {
	project := DefaultProject()
	project.Cloudflare.TTL = 0

	if err := Validate(project); err == nil {
		t.Fatal("Validate() expected error for invalid cloudflare ttl")
	}
}

func TestDefaultProjectExitLocationEmpty(t *testing.T) {
	project := DefaultProject()
	if project.Checks.ExitLocation != "" {
		t.Fatalf("DefaultProject().Checks.ExitLocation = %q, want empty", project.Checks.ExitLocation)
	}
}
