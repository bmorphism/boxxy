//go:build tinygo

// Package skill provides Sideref token support for OCAPN capability binding.
// This file works with embedded.go for medical device firmware builds.
package skill

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"time"
)

// SiderefToken represents an unforgeable object capability reference.
// It binds a skill identity to a device-specific secret, preventing forgery.
// Based on OCAPN (Object Capability Network) principles.
type SiderefToken struct {
	SkillName    string   // Canonical name (e.g., "glucose-monitor")
	DeviceID     [16]byte // Device secret (device-unique, bound at provisioning)
	Token        [32]byte // HMAC-SHA256(name || device_id)
	TokenVersion uint8    // Revocation support (0-255)
	ExpiresAt    uint32   // Unix timestamp, 0 = never expires
}

// NewSiderefToken creates an unforgeable capability reference.
// Requires device secret (bound at provisioning time).
// The token cannot be forged without the device secret.
func NewSiderefToken(skillName string, deviceSecret [16]byte) *SiderefToken {
	h := hmac.New(sha256.New, deviceSecret[:])
	h.Write([]byte(skillName))
	token := [32]byte{}
	copy(token[:], h.Sum(nil))

	return &SiderefToken{
		SkillName:    skillName,
		DeviceID:     deviceSecret,
		Token:        token,
		TokenVersion: 0,
		ExpiresAt:    0, // Never expires by default
	}
}

// VerifySideref checks if a token is valid for a skill name and device.
// Returns nil if valid, error otherwise.
// Uses constant-time comparison to prevent timing attacks.
func VerifySideref(token *SiderefToken, expectedName string, deviceSecret [16]byte) error {
	if token == nil {
		return fmt.Errorf("sideref: token is nil")
	}

	// Check name match
	if token.SkillName != expectedName {
		return fmt.Errorf("sideref: skill name mismatch (got %q, want %q)", token.SkillName, expectedName)
	}

	// Recompute HMAC and compare with constant-time comparison
	h := hmac.New(sha256.New, deviceSecret[:])
	h.Write([]byte(expectedName))
	expectedToken := [32]byte{}
	copy(expectedToken[:], h.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	if !hmac.Equal(token.Token[:], expectedToken[:]) {
		return fmt.Errorf("sideref: token verification failed (forged or wrong device)")
	}

	// Check expiration if set
	if token.ExpiresAt > 0 {
		now := uint32(time.Now().Unix())
		if now > token.ExpiresAt {
			return fmt.Errorf("sideref: token expired at %d", token.ExpiresAt)
		}
	}

	return nil
}

// WithExpiration sets an expiration time on the token (Unix timestamp).
// Returns a new token with the same values but different expiration.
func (t *SiderefToken) WithExpiration(expiresAtUnix uint32) *SiderefToken {
	newToken := *t
	newToken.ExpiresAt = expiresAtUnix
	return &newToken
}

// WithVersion increments the token version for revocation tracking.
func (t *SiderefToken) WithVersion(version uint8) *SiderefToken {
	newToken := *t
	newToken.TokenVersion = version
	return &newToken
}

// MarshalSideref encodes token to compact format for BLE/IEEE 11073 advertisement.
// Format: [skill_name_len:1][trit:1][token_version:1][expires_at:4][token:32][name:...]
// Used for firmware advertisement over BLE or custom medical protocols.
func (t *SiderefToken) MarshalSideref() []byte {
	trit := ComputeTriEmbedded(t.SkillName)
	buf := make([]byte, 40+len(t.SkillName))
	buf[0] = byte(len(t.SkillName))
	buf[1] = trit
	buf[2] = t.TokenVersion
	binary.BigEndian.PutUint32(buf[3:7], t.ExpiresAt)
	copy(buf[7:39], t.Token[:])
	copy(buf[39:], []byte(t.SkillName))
	return buf
}

// UnmarshalSideref decodes Sideref token from compact format.
// Returns error if buffer is too small or invalid.
func UnmarshalSideref(buf []byte) (*SiderefToken, error) {
	if len(buf) < 39 {
		return nil, fmt.Errorf("sideref: buffer too small (need 39+ bytes, got %d)", len(buf))
	}

	nameLen := int(buf[0])
	if len(buf) < 39+nameLen {
		return nil, fmt.Errorf("sideref: buffer too small for name (need %d bytes, got %d)", 39+nameLen, len(buf))
	}

	token := &SiderefToken{
		TokenVersion: buf[2],
		ExpiresAt:    binary.BigEndian.Uint32(buf[3:7]),
	}
	copy(token.Token[:], buf[7:39])
	token.SkillName = string(buf[39 : 39+nameLen])

	return token, nil
}

// String returns a human-readable representation of the token.
// Truncates token for display safety.
func (t *SiderefToken) String() string {
	tokenHex := fmt.Sprintf("%x", t.Token[:8])
	if t.ExpiresAt == 0 {
		return fmt.Sprintf("SiderefToken{%s@%x... v%d never-expires}", t.SkillName, tokenHex, t.TokenVersion)
	}
	expTime := time.Unix(int64(t.ExpiresAt), 0)
	return fmt.Sprintf("SiderefToken{%s@%x... v%d expires=%s}", t.SkillName, tokenHex, t.TokenVersion, expTime.Format(time.RFC3339))
}

// CompactFormatV2 represents Sideref token in wire format for BLE/medical protocols.
// 2-byte header [name_len:1][trit:1] followed by 40 bytes of metadata + name string.
type CompactFormatV2 struct {
	NameLen      uint8
	Trit         uint8
	TokenVersion uint8
	ExpiresAt    uint32
	Token        [32]byte // HMAC-SHA256 Sideref token
	Name         [64]byte // Truncated to 64 bytes max
}

// MarshalCompactV2 encodes skill to CompactFormatV2.
func MarshalCompactV2(name string, trit uint8, sideref *SiderefToken) CompactFormatV2 {
	cf := CompactFormatV2{
		NameLen:      uint8(len(name)),
		Trit:         trit,
		TokenVersion: 0,
		ExpiresAt:    0,
	}
	if sideref != nil {
		cf.TokenVersion = sideref.TokenVersion
		cf.ExpiresAt = sideref.ExpiresAt
		copy(cf.Token[:], sideref.Token[:])
	}
	if len(name) > 64 {
		copy(cf.Name[:], name[:64])
	} else {
		copy(cf.Name[:], name)
	}
	return cf
}

// UnmarshalCompactV2 decodes CompactFormatV2 into skill components.
func UnmarshalCompactV2(cf CompactFormatV2) (*SiderefToken, error) {
	if cf.NameLen > 64 || cf.Trit > 2 {
		return nil, fmt.Errorf("invalid compact format: NameLen=%d Trit=%d", cf.NameLen, cf.Trit)
	}
	name := string(cf.Name[:cf.NameLen])
	s := &SiderefToken{
		SkillName:    name,
		Token:        cf.Token,
		TokenVersion: cf.TokenVersion,
		ExpiresAt:    cf.ExpiresAt,
	}

	return s, nil
}

// DeviceSecretFromBytes creates a 16-byte device secret from input.
// Pads or truncates as needed (not cryptographically secure for key derivation).
// For production: use secure random generation or key derivation function.
func DeviceSecretFromBytes(input []byte) [16]byte {
	secret := [16]byte{}
	if len(input) >= 16 {
		copy(secret[:], input[:16])
	} else {
		copy(secret[:len(input)], input)
		// Pad with zeros (NOT cryptographically secure)
		for i := len(input); i < 16; i++ {
			secret[i] = 0
		}
	}
	return secret
}

// SECURITY_NOTE on ComputeTriEmbedded (from embedded.go):
// This hash is deterministic and public, suitable only for capability
// classification. NOT suitable for security-critical operations
// (authentication, attestation, firmware signing). For those, use
// ATECC608A secure element or other hardware security modules.
