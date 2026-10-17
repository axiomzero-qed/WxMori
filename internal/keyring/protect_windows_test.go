//go:build windows

package keyring

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestProtectedCaptureFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	var salt [16]byte
	var key [32]byte
	for i := range key {
		key[i] = byte(i + 71)
	}
	keys := map[[16]byte][32]byte{salt: key}
	for attempt := 0; attempt < 2; attempt++ {
		if e := SaveCaptured(path, "account", keys); e != nil {
			t.Fatal(e)
		}
	}
	raw, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(raw, []byte(hex.EncodeToString(key[:]))) {
		t.Fatal("plaintext key leaked to disk")
	}
	m, e := Decode(raw, "account")
	if e != nil || m.BySalt[salt] != key {
		t.Fatalf("DPAPI round trip failed %v", e)
	}
	m.Clear()
	raw[len(raw)/2] ^= 1
	if _, e = Decode(raw, "account"); e == nil {
		t.Fatal("tampered key file accepted")
	}
}
