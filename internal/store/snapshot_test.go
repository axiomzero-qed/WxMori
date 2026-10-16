package store_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

func TestPlaintextWALSnapshotLeavesSourceUntouched(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.MkdirAll(filepath.Join(source, "message"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "message", "message_0.db")
	db, err := store.Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA wal_autocheckpoint=0; CREATE TABLE example(value TEXT); INSERT INTO example VALUES('committed in WAL');`); err != nil {
		t.Fatal(err)
	}
	before, _, err := store.Fingerprint(source, false)
	if err != nil {
		t.Fatal(err)
	}
	dir, _, err := store.Snapshot(context.Background(), config.Config{DecryptedDir: source}, false)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	copyDB, err := store.Open(filepath.Join(dir, "message", "message_0.db"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer copyDB.Close()
	var text string
	if err = copyDB.QueryRow("SELECT value FROM example").Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != "committed in WAL" {
		t.Fatal("WAL data missing")
	}
	after, _, err := store.Fingerprint(source, false)
	if err != nil || before != after {
		t.Fatal("source was changed")
	}
}

func TestFailureCleanupAndUnsupportedSchema(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(source, "contact"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "contact", "contact.db")
	db, err := store.Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("CREATE TABLE contact(wrong_column TEXT)"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, err = store.Build(context.Background(), config.Config{DecryptedDir: source})
	if err == nil {
		t.Fatal("schema accepted")
	}
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Fatalf("plaintext left after failed import: %v %v", entries, err)
	}
	if err = os.WriteFile(path+"-wal", []byte("non-empty"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WXMORI_MASTER_KEY", "0000000000000000000000000000000000000000000000000000000000000000")
	_, _, err = store.Snapshot(context.Background(), config.Config{DataDir: source}, false)
	if err == nil {
		t.Fatal("encrypted WAL accepted")
	}
	entries, err = os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Fatal("snapshot left after WAL rejection")
	}
}

func TestRollbackJournalRejected(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "message_0.db"), []byte("incomplete"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "message_0.db-journal"), []byte("active"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err := store.Snapshot(context.Background(), config.Config{DecryptedDir: source}, false)
	if err == nil || !strings.Contains(err.Error(), "ROLLBACK_JOURNAL_PRESENT") {
		t.Fatalf("expected journal rejection, got %v", err)
	}
}
