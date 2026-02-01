package skill

import (
	"bytes"
	"testing"
	"time"
)

// TestNewSiderefToken verifies token creation with deterministic HMAC.
func TestNewSiderefToken(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"

	token := NewSiderefToken(skillName, deviceSecret)

	if token.SkillName != skillName {
		t.Errorf("got %q, want %q", token.SkillName, skillName)
	}
	if token.TokenVersion != 0 {
		t.Errorf("got version %d, want 0", token.TokenVersion)
	}
	if token.ExpiresAt != 0 {
		t.Errorf("got expiry %d, want 0", token.ExpiresAt)
	}
	// Token should be non-zero (HMAC result)
	if token.Token == [32]byte{} {
		t.Error("token is all zeros, expected HMAC result")
	}
}

// TestSiderefTokenDeterministic verifies same inputs always produce same token.
func TestSiderefTokenDeterministic(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"

	token1 := NewSiderefToken(skillName, deviceSecret)
	token2 := NewSiderefToken(skillName, deviceSecret)

	if !bytes.Equal(token1.Token[:], token2.Token[:]) {
		t.Error("same inputs should produce same token (HMAC is deterministic)")
	}
}

// TestSiderefTokenUnforgeable verifies different secrets produce different tokens.
func TestSiderefTokenUnforgeable(t *testing.T) {
	deviceSecret1 := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	deviceSecret2 := [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	skillName := "glucose-monitor"

	token1 := NewSiderefToken(skillName, deviceSecret1)
	token2 := NewSiderefToken(skillName, deviceSecret2)

	if bytes.Equal(token1.Token[:], token2.Token[:]) {
		t.Error("different secrets should produce different tokens")
	}
}

// TestVerifySideref_Valid checks successful token verification.
func TestVerifySideref_Valid(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"

	token := NewSiderefToken(skillName, deviceSecret)

	err := VerifySideref(token, skillName, deviceSecret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestVerifySideref_WrongName rejects token with mismatched skill name.
func TestVerifySideref_WrongName(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	token := NewSiderefToken("glucose-monitor", deviceSecret)

	err := VerifySideref(token, "heart-monitor", deviceSecret)
	if err == nil {
		t.Error("expected error for wrong name, got nil")
	}
	if err.Error() != "sideref: skill name mismatch (got \"glucose-monitor\", want \"heart-monitor\")" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestVerifySideref_WrongSecret rejects token with wrong device secret.
func TestVerifySideref_WrongSecret(t *testing.T) {
	deviceSecret1 := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	deviceSecret2 := [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	skillName := "glucose-monitor"

	token := NewSiderefToken(skillName, deviceSecret1)

	err := VerifySideref(token, skillName, deviceSecret2)
	if err == nil {
		t.Error("expected error for wrong secret, got nil")
	}
	if err.Error() != "sideref: token verification failed (forged or wrong device)" {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestVerifySideref_NilToken rejects nil token.
func TestVerifySideref_NilToken(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	err := VerifySideref(nil, "glucose-monitor", deviceSecret)
	if err == nil {
		t.Error("expected error for nil token")
	}
}

// TestWithExpiration sets and verifies expiration time.
func TestWithExpiration(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	futureTime := uint32(time.Now().Unix()) + 3600 // 1 hour from now
	expiringToken := token.WithExpiration(futureTime)

	if expiringToken.ExpiresAt != futureTime {
		t.Errorf("got expiry %d, want %d", expiringToken.ExpiresAt, futureTime)
	}

	// Should still verify as valid
	err := VerifySideref(expiringToken, skillName, deviceSecret)
	if err != nil {
		t.Fatalf("unexpected error for future expiry: %v", err)
	}
}

// TestWithExpiration_Expired rejects expired token.
func TestWithExpiration_Expired(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	pastTime := uint32(time.Now().Unix()) - 3600 // 1 hour ago
	expiredToken := token.WithExpiration(pastTime)

	err := VerifySideref(expiredToken, skillName, deviceSecret)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

// TestWithVersion increments token version for revocation tracking.
func TestWithVersion(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	versionedToken := token.WithVersion(5)

	if versionedToken.TokenVersion != 5 {
		t.Errorf("got version %d, want 5", versionedToken.TokenVersion)
	}

	// Should still verify
	err := VerifySideref(versionedToken, skillName, deviceSecret)
	if err != nil {
		t.Fatalf("unexpected error after version bump: %v", err)
	}
}

// TestMarshalUnmarshalSideref roundtrip serialization.
func TestMarshalUnmarshalSideref(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"

	token := NewSiderefToken(skillName, deviceSecret)
	token = token.WithVersion(3)
	buf := token.MarshalSideref()

	decoded, err := UnmarshalSideref(buf)
	if err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.SkillName != token.SkillName {
		t.Errorf("name mismatch: got %q, want %q", decoded.SkillName, token.SkillName)
	}
	if !bytes.Equal(decoded.Token[:], token.Token[:]) {
		t.Error("token mismatch after roundtrip")
	}
	if decoded.TokenVersion != token.TokenVersion {
		t.Errorf("version mismatch: got %d, want %d", decoded.TokenVersion, token.TokenVersion)
	}
}

// TestMarshalSideref_BufferSize verifies correct buffer size.
func TestMarshalSideref_BufferSize(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	buf := token.MarshalSideref()

	// Should be: 1 (name_len) + 1 (trit) + 1 (version) + 4 (expiry) + 32 (token) + len(name)
	expectedSize := 39 + len(skillName)
	if len(buf) != expectedSize {
		t.Errorf("got buffer size %d, want %d", len(buf), expectedSize)
	}
}

// TestUnmarshalSideref_TooSmall rejects buffers that are too small.
func TestUnmarshalSideref_TooSmall(t *testing.T) {
	buf := make([]byte, 30) // Too small (need 39+ bytes)

	_, err := UnmarshalSideref(buf)
	if err == nil {
		t.Error("expected error for buffer too small")
	}
}

// TestUnmarshalSideref_InvalidNameLength rejects invalid name length.
func TestUnmarshalSideref_InvalidNameLength(t *testing.T) {
	buf := make([]byte, 39)
	buf[0] = 100 // Name length too large

	_, err := UnmarshalSideref(buf)
	if err == nil {
		t.Error("expected error for invalid name length")
	}
}

// TestMarshalCompactV2 roundtrip with skill data.
func TestMarshalCompactV2(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	trit := uint8(1)

	token := NewSiderefToken(skillName, deviceSecret)
	cf := MarshalCompactV2(skillName, trit, token)

	if cf.NameLen != uint8(len(skillName)) {
		t.Errorf("got name length %d, want %d", cf.NameLen, len(skillName))
	}
	if cf.Trit != trit {
		t.Errorf("got trit %d, want %d", cf.Trit, trit)
	}
	if !bytes.Equal(cf.Token[:], token.Token[:]) {
		t.Error("token mismatch in compact format")
	}
}

// TestDeviceSecretFromBytes creates device secret from input bytes.
func TestDeviceSecretFromBytes(t *testing.T) {
	input := []byte("device-secret-key")
	secret := DeviceSecretFromBytes(input)

	if len(secret) != 16 {
		t.Errorf("got secret length %d, want 16", len(secret))
	}

	// Should be deterministic
	secret2 := DeviceSecretFromBytes(input)
	if secret != secret2 {
		t.Error("device secret generation not deterministic")
	}
}

// TestDeviceSecretFromBytes_Short pads short input.
func TestDeviceSecretFromBytes_Short(t *testing.T) {
	input := []byte("short")
	secret := DeviceSecretFromBytes(input)

	if len(secret) != 16 {
		t.Errorf("got secret length %d, want 16", len(secret))
	}
	// First 5 bytes should match input
	if !bytes.Equal(secret[:5], input) {
		t.Error("secret should be padded version of input")
	}
}

// TestString returns readable token representation.
func TestString(t *testing.T) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	str := token.String()
	if len(str) == 0 {
		t.Error("expected non-empty string representation")
	}
	if !bytes.Contains([]byte(str), []byte(skillName)) {
		t.Errorf("string should contain skill name: %s", str)
	}
}

// BenchmarkNewSiderefToken measures token creation performance.
func BenchmarkNewSiderefToken(b *testing.B) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewSiderefToken(skillName, deviceSecret)
	}
}

// BenchmarkVerifySideref measures token verification performance.
func BenchmarkVerifySideref(b *testing.B) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = VerifySideref(token, skillName, deviceSecret)
	}
}

// BenchmarkMarshalSideref measures serialization performance.
func BenchmarkMarshalSideref(b *testing.B) {
	deviceSecret := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	skillName := "glucose-monitor"
	token := NewSiderefToken(skillName, deviceSecret)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = token.MarshalSideref()
	}
}
