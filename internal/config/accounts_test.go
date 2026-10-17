package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
)

func TestDescribeAccountUsesFileMetadataAndDoesNotUnprotectKeyring(t *testing.T) {
	root := t.TempDir()
	storage := filepath.Join(root, "wxid_sample_7f3a", "db_storage")
	if err := os.MkdirAll(storage, 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"contact.db":       "abc",
		"message_0.db":     "1234567",
		"message_0.db-wal": "wal",
	} {
		if err := os.WriteFile(filepath.Join(storage, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	configPath := filepath.Join(root, "config", "config.json")
	keyringPath := (config.Config{DataDir: storage, Path: configPath}).CapturedKeyFile()
	if err := os.MkdirAll(filepath.Dir(keyringPath), 0700); err != nil {
		t.Fatal(err)
	}
	// Availability is a metadata signal only. This intentionally is not a valid
	// protected keyring, so the implementation must not try to unprotect it.
	if err := os.WriteFile(keyringPath, []byte("not a keyring and contains no key material"), 0600); err != nil {
		t.Fatal(err)
	}

	a := config.DescribeAccount(storage, configPath)
	if a.DirectoryID != "wxid_sample_7f3a" || a.InternalID != "wxid_sample" {
		t.Fatalf("unexpected directory identity metadata: directory=%q internal=%q", a.DirectoryID, a.InternalID)
	}
	if !a.MetadataComplete || !a.KeyringAvailable {
		t.Fatalf("metadata/keyring availability was not reported: %+v", a)
	}
	if a.DatabaseCount != 2 || a.CoreDatabaseCount != 2 || a.DatabaseBytes != int64(len("abc1234567")) || a.MessageDatabaseBytes != int64(len("1234567")) {
		t.Fatalf("unexpected database metadata: %+v", a)
	}
	if a.LastActiveAt == "" {
		t.Fatal("last file change metadata was not reported")
	}
}

func TestAccountsUsesExplicitSyntheticRootAndDeduplicatesSelectedSource(t *testing.T) {
	root := t.TempDir()
	for _, id := range []string{"account-one_1234", "account-two_abcd"} {
		storage := filepath.Join(root, id, "db_storage")
		if err := os.MkdirAll(storage, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(storage, "contact.db"), []byte("synthetic"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	selected := filepath.Join(root, "account-one_1234", "db_storage")
	accounts := config.Accounts(selected, root, filepath.Join(t.TempDir(), "config.json"))
	if len(accounts) != 2 {
		t.Fatalf("expected two synthetic accounts after de-duplication, got %d", len(accounts))
	}
	seen := map[string]bool{}
	for _, account := range accounts {
		seen[account.DataDir] = true
	}
	if !seen[filepath.Clean(selected)] || !seen[filepath.Join(root, "account-two_abcd", "db_storage")] {
		t.Fatalf("explicit root returned unexpected account paths")
	}
}
