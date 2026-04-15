package keygen

import (
	"encoding/base64"
	"testing"
)

func TestGenerate(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if err := ValidateKey(kp.PrivateKey); err != nil {
		t.Fatalf("generated private key is invalid: %v", err)
	}
	if err := ValidateKey(kp.PublicKey); err != nil {
		t.Fatalf("generated public key is invalid: %v", err)
	}

	if kp.PrivateKey == kp.PublicKey {
		t.Fatal("private and public keys should not be equal")
	}
}

func TestGenerateUniqueness(t *testing.T) {
	kp1, _ := Generate()
	kp2, _ := Generate()
	if kp1.PrivateKey == kp2.PrivateKey {
		t.Fatal("two generated key pairs should have different private keys")
	}
}

func TestValidateKey(t *testing.T) {
	valid := base64.StdEncoding.EncodeToString(make([]byte, 32))
	if err := ValidateKey(valid); err != nil {
		t.Fatalf("ValidateKey(valid) error = %v", err)
	}
}

func TestValidateKeyEmpty(t *testing.T) {
	if err := ValidateKey(""); err == nil {
		t.Fatal("ValidateKey(\"\") expected error")
	}
}

func TestValidateKeyBadBase64(t *testing.T) {
	if err := ValidateKey("not-valid-base64!!!"); err == nil {
		t.Fatal("ValidateKey(bad base64) expected error")
	}
}

func TestValidateKeyWrongLength(t *testing.T) {
	short := base64.StdEncoding.EncodeToString(make([]byte, 16))
	if err := ValidateKey(short); err == nil {
		t.Fatal("ValidateKey(16 bytes) expected error")
	}
}
