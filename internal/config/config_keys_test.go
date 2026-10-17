package config_test

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
)

func TestKeysLoadsAccountBoundCapturedPerSaltKeys(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "account-a", "db_storage")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	c := config.Config{DataDir: source, Path: filepath.Join(root, "wxmori", "config.json")}
	var salt [16]byte
	var key [32]byte
	for i := range salt {
		salt[i] = byte(i + 1)
	}
	for i := range key {
		key[i] = byte(i + 51)
	}
	file := keyring.File{Version: 1, Account: keyring.AccountID(source), DatabaseKeys: map[string]string{hex.EncodeToString(salt[:]): hex.EncodeToString(key[:])}}
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatal(err)
	}
	path := c.CapturedKeyFile()
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WXMORI_MASTER_KEY", "")
	m, err := c.Keys()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Clear()
	if len(m.Master) != 0 || m.BySalt[salt] != key {
		t.Fatal("captured per-salt key was not loaded without a master key")
	}

	other := config.Config{DataDir: filepath.Join(root, "account-b", "db_storage"), Path: c.Path, KeyFile: path}
	_, err = other.Keys()
	if err == nil || !strings.Contains(err.Error(), "KEY_ACCOUNT_MISMATCH") {
		t.Fatalf("key file was not account-bound: %v", err)
	}
	if strings.Contains(err.Error(), hex.EncodeToString(key[:])) || strings.Contains(err.Error(), hex.EncodeToString(salt[:])) {
		t.Fatal("key error exposed secret material")
	}
}

func TestKeysMissingAndManualMasterCompatibility(t *testing.T) {
	root := t.TempDir()
	c := config.Config{DataDir: filepath.Join(root, "account", "db_storage"), Path: filepath.Join(root, "cfg", "config.json")}
	t.Setenv("WXMORI_MASTER_KEY", "")
	_, err := c.Keys()
	if err == nil || !strings.Contains(err.Error(), "KEY_REQUIRED") {
		t.Fatalf("expected missing-key error, got %v", err)
	}
	secret := strings.Repeat("ab", 32)
	t.Setenv("WXMORI_MASTER_KEY", secret)
	m, err := c.Keys()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Clear()
	if got := hex.EncodeToString(m.Master); got != secret {
		t.Fatal("manual master key compatibility failed")
	}
}
