package security

import (
	"strings"
	"testing"
)

func TestPasswordHasherRoundTrip(t *testing.T) {
	t.Parallel()

	hasher := NewPasswordHasher(Argon2Params{
		Memory:      8 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	hash, err := hasher.Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	if strings.Contains(hash, "correct horse battery staple") {
		t.Fatal("Hash() contains the plaintext password")
	}
	if !hasher.Verify(hash, "correct horse battery staple") {
		t.Fatal("Verify() rejected the correct password")
	}
	if hasher.Verify(hash, "wrong password") {
		t.Fatal("Verify() accepted the wrong password")
	}
}

func TestPasswordHasherRejectsMalformedHash(t *testing.T) {
	t.Parallel()

	hasher := NewPasswordHasher(DefaultArgon2Params())
	if hasher.Verify("not-an-argon2-hash", "password") {
		t.Fatal("Verify() accepted a malformed hash")
	}
}
