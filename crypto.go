package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

// BitLink optional shared-key transport encryption.
//
// When a 32-byte mesh key is configured (via Settings → "Mesh shared key"),
// every Packet's Data field is wrapped in AES-256-GCM before being broadcast
// and unwrapped on receive. The wrapped form prepends a 1-byte magic + the
// 12-byte nonce so a receiver can distinguish encrypted from legacy plaintext
// payloads and roll out gradually across a mesh.
//
// SECURITY NOTE: Sender / Recipient / Group fields remain plaintext. This is
// transport encryption, not full end-to-end privacy.

const (
	cryptoMagic    byte = 0xCE
	cryptoKeyBytes      = 32 // AES-256
	cryptoNonceLen      = 12 // GCM standard
)

var (
	meshKeyMu sync.RWMutex
	meshKey   []byte // nil = encryption disabled
)

// MeshKey returns the currently-configured key as a hex string, "" when disabled.
func MeshKey() string {
	meshKeyMu.RLock()
	defer meshKeyMu.RUnlock()
	if meshKey == nil {
		return ""
	}
	return hex.EncodeToString(meshKey)
}

// SetMeshKeyHex configures the mesh shared key from a 64-char hex string.
// Pass "" to disable encryption. Returns an error for malformed input.
func SetMeshKeyHex(s string) error {
	if s == "" {
		meshKeyMu.Lock()
		meshKey = nil
		meshKeyMu.Unlock()
		if prefs != nil {
			prefs.RemoveValue("crypto.meshKey")
		}
		return nil
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(raw) != cryptoKeyBytes {
		return errors.New("mesh key must be exactly 32 bytes (64 hex chars)")
	}
	meshKeyMu.Lock()
	meshKey = raw
	meshKeyMu.Unlock()
	if prefs != nil {
		prefs.SetString("crypto.meshKey", s)
	}
	return nil
}

// GenerateMeshKey returns a fresh random 32-byte key encoded as hex.
func GenerateMeshKey() string {
	b := make([]byte, cryptoKeyBytes)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func loadMeshKey() {
	if prefs == nil {
		return
	}
	if s := prefs.String("crypto.meshKey"); s != "" {
		_ = SetMeshKeyHex(s)
	}
}

func currentKey() []byte {
	meshKeyMu.RLock()
	defer meshKeyMu.RUnlock()
	if meshKey == nil {
		return nil
	}
	out := make([]byte, len(meshKey))
	copy(out, meshKey)
	return out
}

// tryEncryptData wraps data with AES-GCM under the current mesh key.
// Returns the original data + false when no key is configured.
func tryEncryptData(data []byte) ([]byte, bool) {
	key := currentKey()
	if key == nil {
		return data, false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return data, false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return data, false
	}
	nonce := make([]byte, cryptoNonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return data, false
	}
	ct := gcm.Seal(nil, nonce, data, nil)
	out := make([]byte, 0, 1+len(nonce)+len(ct))
	out = append(out, cryptoMagic)
	out = append(out, nonce...)
	out = append(out, ct...)
	return out, true
}

// tryDecryptData unwraps an AES-GCM payload with the current mesh key.
// Returns the plaintext + true on success; (nil, false) on any failure
// (no key, magic mismatch, auth failure, etc.). Callers should keep using
// the original payload as plaintext when this returns false, so peers
// without a key can still see legacy unencrypted packets.
func tryDecryptData(data []byte) ([]byte, bool) {
	if len(data) < 1+cryptoNonceLen+16 || data[0] != cryptoMagic {
		return nil, false
	}
	key := currentKey()
	if key == nil {
		return nil, false
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, false
	}
	nonce := data[1 : 1+cryptoNonceLen]
	ct := data[1+cryptoNonceLen:]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, false
	}
	return pt, true
}
