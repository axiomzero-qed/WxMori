//go:build windows

package keyring

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestCapturedMetadataDoesNotReturnKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "protected.json")
	var salt [16]byte
	var key [32]byte
	for i := range key {
		key[i] = byte(0xa1 + i)
	}
	salt[0] = 3
	account := AccountID(t.TempDir())
	if err := SaveCaptured(path, account, map[[16]byte][32]byte{salt: key}); err != nil {
		t.Fatal(err)
	}
	info, err := InspectCaptured(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.AccountHash != account || info.CapturedAt == "" || info.DatabaseKeyCount != 1 || !info.Protected {
		t.Fatal("captured metadata was incomplete")
	}
	b, _ := json.Marshal(info)
	if strings.Contains(string(b), hex.EncodeToString(key[:])) || strings.Contains(string(b), "database_keys") || strings.Contains(string(b), "protected\":\"") {
		t.Fatal("key metadata included secret material")
	}
}
