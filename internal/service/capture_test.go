package service_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
	"github.com/axiomzero-qed/WxMori/internal/service"
)

func TestStatusDoesNotTreatPartialKeyringAsReady(t *testing.T) {
	t.Setenv("WXMORI_MASTER_KEY", "")
	dir := t.TempDir()
	var salts [2][16]byte
	for i, name := range []string{"contact.db", "message_0.db"} {
		for j := range salts[i] {
			salts[i][j] = byte(i*16 + j + 1)
		}
		if err := os.WriteFile(filepath.Join(dir, name), salts[i][:], 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := config.Config{DataDir: dir, Path: filepath.Join(t.TempDir(), "config.json"), KeyFile: filepath.Join(t.TempDir(), "keys.json"), Timezone: "UTC"}
	secret := strings.Repeat("a7", 32)
	f := keyring.File{Version: 1, Account: keyring.AccountID(dir), DatabaseKeys: map[string]string{hex.EncodeToString(salts[0][:]): secret}}
	write := func() {
		t.Helper()
		b, _ := json.Marshal(f)
		if err := os.WriteFile(c.KeyFile, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write()
	a := service.New(c, nil)
	defer a.Close()
	check := func() map[string]any {
		t.Helper()
		v, err := a.Status(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(v)
		if strings.Contains(string(b), secret) {
			t.Fatal("status revealed a key")
		}
		return asMap(t, v)
	}
	v := check()
	if v["ready"] != false || v["matching_database_keys"] != float64(1) || v["required_database_keys"] != float64(2) || v["setup_error"].(map[string]any)["code"] != "DATABASE_KEY_MISSING" {
		t.Fatalf("partial keyring appeared ready: %v", v)
	}
	f.DatabaseKeys[hex.EncodeToString(salts[1][:])] = secret
	write()
	if v = check(); v["ready"] != true || v["matching_database_keys"] != float64(2) {
		t.Fatalf("complete key material was not detected: %v", v)
	}
	// This is setup readiness only: the first data call must still authenticate
	// every page and validate SQLite, and rejects these deliberately short files.
	if _, err := a.Sessions(context.Background(), service.ListArgs{}); err == nil {
		t.Fatal("status readiness bypassed data validation")
	}
}
