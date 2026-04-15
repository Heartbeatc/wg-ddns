package keygen

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

type KeyPair struct {
	PrivateKey string
	PublicKey  string
}

func Generate() (KeyPair, error) {
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return KeyPair{}, fmt.Errorf("generate random key: %w", err)
	}

	// Curve25519 clamping (same as wg genkey)
	private[0] &= 248
	private[31] &= 127
	private[31] |= 64

	var public [32]byte
	curve25519.ScalarBaseMult(&public, &private)

	return KeyPair{
		PrivateKey: base64.StdEncoding.EncodeToString(private[:]),
		PublicKey:  base64.StdEncoding.EncodeToString(public[:]),
	}, nil
}

// ValidateKey checks that a WireGuard key is valid base64 encoding of exactly
// 32 bytes. Returns nil if valid.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("密钥为空")
	}
	data, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("密钥不是有效的 base64 编码: %w", err)
	}
	if len(data) != 32 {
		return fmt.Errorf("密钥长度不正确（需要 32 字节，实际 %d 字节）", len(data))
	}
	return nil
}
