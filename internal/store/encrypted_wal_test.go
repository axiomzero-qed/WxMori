package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

func TestSnapshotReplaysSyntheticEncryptedWALWithoutChangingSource(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "db_storage")
	if err := os.MkdirAll(filepath.Join(source, "contact"), 0700); err != nil {
		t.Fatal(err)
	}
	plainPath := filepath.Join(root, "empty.sqlite")
	db, err := store.Open(plainPath, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("PRAGMA page_size=4096; PRAGMA user_version=23"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	plain, err := os.ReadFile(plainPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(plain) != cryptodb.PageSize {
		t.Fatalf("expected one synthetic SQLite page, got %d bytes", len(plain))
	}
	plain[20] = cryptodb.Reserve
	binary.BigEndian.PutUint16(plain[105:107], uint16(cryptodb.BodySize))

	var salt [16]byte
	var key [32]byte
	for i := range salt {
		salt[i] = byte(0x70 + i)
	}
	for i := range key {
		key[i] = byte(0x20 + i)
	}
	dbPath := filepath.Join(source, "contact", "contact.db")
	if err = os.WriteFile(dbPath, encryptSyntheticSQLite(plain, salt, key), 0600); err != nil {
		t.Fatal(err)
	}
	updated := append([]byte(nil), plain...)
	binary.BigEndian.PutUint32(updated[60:64], 42)
	walBytes := syntheticCommittedWAL(encryptSyntheticSQLite(updated, salt, key))
	walPath := dbPath + "-wal"
	if err = os.WriteFile(walPath, walBytes, 0600); err != nil {
		t.Fatal(err)
	}
	dbHashBefore, walHashBefore := fileHash(t, dbPath), fileHash(t, walPath)

	keysJSON, err := json.Marshal(keyring.File{
		Version: 1, Account: keyring.AccountID(source),
		DatabaseKeys: map[string]string{hex.EncodeToString(salt[:]): hex.EncodeToString(key[:])},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := config.Config{DataDir: source, Path: filepath.Join(root, "config", "config.json")}
	if err = os.MkdirAll(filepath.Dir(c.CapturedKeyFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(c.CapturedKeyFile(), keysJSON, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WXMORI_MASTER_KEY", "")
	dir, _, report, err := store.SnapshotWithReport(context.Background(), c, false)
	if err != nil {
		t.Fatalf("snapshot with synthetic encrypted WAL failed: %v", err)
	}
	defer os.RemoveAll(dir)
	if report.EncryptedWALFiles != 1 || report.CommittedFrames != 1 {
		t.Fatalf("unexpected snapshot report: %+v", report)
	}
	copyPath := filepath.Join(dir, "contact", "contact.db")
	if err = store.Integrity(context.Background(), copyPath); err != nil {
		t.Fatalf("replayed snapshot failed SQLite integrity check: %v", err)
	}
	copyDB, err := store.Open(copyPath, true)
	if err != nil {
		t.Fatal(err)
	}
	var version int
	err = copyDB.QueryRow("PRAGMA user_version").Scan(&version)
	copyDB.Close()
	if err != nil || version != 42 {
		t.Fatalf("snapshot user_version=%d, err=%v; want committed WAL value 42", version, err)
	}
	if got := fileHash(t, dbPath); got != dbHashBefore {
		t.Fatal("snapshot changed the source database")
	}
	if got := fileHash(t, walPath); got != walHashBefore {
		t.Fatal("snapshot changed the source WAL")
	}
}

func syntheticCommittedWAL(encryptedPage []byte) []byte {
	const magic = uint32(0x377f0682)
	wal := make([]byte, 32+24+cryptodb.PageSize)
	binary.BigEndian.PutUint32(wal[0:], magic)
	binary.BigEndian.PutUint32(wal[4:], 3007000)
	binary.BigEndian.PutUint32(wal[8:], cryptodb.PageSize)
	binary.BigEndian.PutUint32(wal[12:], 1)
	copy(wal[16:24], []byte("synthWAL"))
	order := binary.LittleEndian
	checksum := sqliteWALChecksum(order, [2]uint32{}, wal[:24])
	binary.BigEndian.PutUint32(wal[24:], checksum[0])
	binary.BigEndian.PutUint32(wal[28:], checksum[1])
	frame := wal[32:]
	binary.BigEndian.PutUint32(frame[0:], 1)
	binary.BigEndian.PutUint32(frame[4:], 1)
	copy(frame[8:16], wal[16:24])
	copy(frame[24:], encryptedPage)
	checksum = sqliteWALChecksum(order, checksum, frame[:8], encryptedPage)
	binary.BigEndian.PutUint32(frame[16:], checksum[0])
	binary.BigEndian.PutUint32(frame[20:], checksum[1])
	return wal
}

func sqliteWALChecksum(order binary.ByteOrder, sum [2]uint32, parts ...[]byte) [2]uint32 {
	for _, part := range parts {
		for i := 0; i+8 <= len(part); i += 8 {
			sum[0] += order.Uint32(part[i:]) + sum[1]
			sum[1] += order.Uint32(part[i+4:]) + sum[0]
		}
	}
	return sum
}

func fileHash(t *testing.T, path string) [32]byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256.Sum256(b)
}
