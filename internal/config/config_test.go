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

func TestValidateExitDDNSRequiresSSHHost(t *testing.T) {
	project := DefaultProject()
	project.ExitDDNS.Enabled = true
	project.ExitDDNS.Domain = "ssh-exit.example.com"
	project.ExitDDNS.Interval = 300
	project.Nodes.HK.SSHHost = ""

	err := Validate(project)
	if err == nil {
		t.Fatal("Validate() expected error when exit_ddns enabled but ssh_host empty")
	}
	if !contains(err.Error(), "ssh_host") {
		t.Fatalf("error should mention ssh_host, got: %s", err)
	}
}

func TestValidateExitDDNSRequiresSSHHostMatch(t *testing.T) {
	project := DefaultProject()
	project.ExitDDNS.Enabled = true
	project.ExitDDNS.Domain = "ssh-exit.example.com"
	project.ExitDDNS.Interval = 300
	project.Nodes.HK.SSHHost = "other.example.com"

	err := Validate(project)
	if err == nil {
		t.Fatal("Validate() expected error when ssh_host != exit_ddns.domain")
	}
	if !contains(err.Error(), "不一致") {
		t.Fatalf("error should mention mismatch, got: %s", err)
	}
}

func TestValidateExitDDNSMatchingSSHHost(t *testing.T) {
	project := DefaultProject()
	project.ExitDDNS.Enabled = true
	project.ExitDDNS.Domain = "ssh-exit.example.com"
	project.ExitDDNS.Interval = 300
	project.Nodes.HK.SSHHost = "ssh-exit.example.com"

	if err := Validate(project); err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
}

func TestValidateExitDDNSDisabledNoSSHHostRequired(t *testing.T) {
	project := DefaultProject()
	project.ExitDDNS.Enabled = false
	project.Nodes.HK.SSHHost = ""

	if err := Validate(project); err != nil {
		t.Fatalf("Validate() unexpected error when DDNS disabled: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsStr(s, substr)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
