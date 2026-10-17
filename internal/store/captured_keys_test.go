package store_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

func TestSnapshotUsesCapturedKeysForEachDatabase(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "db_storage")
	contactDir, messageDir := filepath.Join(source, "contact"), filepath.Join(source, "message")
	if err := os.MkdirAll(contactDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(messageDir, 0700); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(root, "fixture.sqlite")
	db, err := sql.Open("sqlite", plain)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`PRAGMA page_size=4096; PRAGMA user_version=23;`); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	// SQLite's reserved-byte count tells readers that each encrypted page has an 80-byte trailer.
	raw, err := os.ReadFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) < cryptodb.PageSize || len(raw)%cryptodb.PageSize != 0 {
		t.Fatalf("unexpected fixture size %d", len(raw))
	}
	raw[20] = cryptodb.Reserve
	// The empty schema b-tree has no cells; lower its content boundary to the new usable end.
	if len(raw) == cryptodb.PageSize {
		binary.BigEndian.PutUint16(raw[105:107], uint16(cryptodb.BodySize))
	}
	var salt1, salt2 [16]byte
	var key1, key2 [32]byte
	for i := 0; i < 16; i++ {
		salt1[i], salt2[i] = byte(i+11), byte(i+101)
	}
	for i := 0; i < 32; i++ {
		key1[i], key2[i] = byte(i+31), byte(i+71)
	}
	enc := encryptSyntheticSQLite(raw, salt1, key1)
	checkEnc, checkPlain := filepath.Join(root, "check.enc"), filepath.Join(root, "check.sqlite")
	if err = os.WriteFile(checkEnc, enc, 0600); err != nil {
		t.Fatal(err)
	}
	if err = cryptodb.DecryptFileWithKey(context.Background(), checkEnc, checkPlain, key1[:]); err != nil {
		t.Fatalf("fixture decrypt failed: %v", err)
	}
	decoded, err := os.ReadFile(checkPlain)
	if err != nil {
		t.Fatal(err)
	}
	for off := 0; off < len(raw); off += cryptodb.PageSize {
		if !bytes.Equal(raw[off:off+cryptodb.BodySize], decoded[off:off+cryptodb.BodySize]) {
			t.Fatalf("fixture decrypted body differs on page at %d", off)
		}
	}
	encPath := filepath.Join(contactDir, "contact.db")
	if err = os.WriteFile(encPath, enc, 0600); err != nil {
		t.Fatal(err)
	}
	// A second encrypted database has a different salt and its own captured key.
	raw2 := append([]byte(nil), raw...)
	enc2 := encryptSyntheticSQLite(raw2, salt2, key2)
	if err = os.WriteFile(filepath.Join(messageDir, "message_0.db"), enc2, 0600); err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{hex.EncodeToString(salt1[:]): hex.EncodeToString(key1[:]), hex.EncodeToString(salt2[:]): hex.EncodeToString(key2[:])}
	data, err := json.Marshal(keyring.File{Version: 1, Account: keyring.AccountID(source), DatabaseKeys: keys})
	if err != nil {
		t.Fatal(err)
	}
	c := config.Config{DataDir: source, Path: filepath.Join(root, "config", "config.json")}
	keyPath := c.CapturedKeyFile()
	if err = os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WXMORI_MASTER_KEY", "")
	dir, _, err := store.Snapshot(context.Background(), c, false)
	if err != nil {
		t.Fatalf("snapshot with captured per-database keys failed: %v", err)
	}
	defer os.RemoveAll(dir)
	for _, rel := range []string{filepath.Join("contact", "contact.db"), filepath.Join("message", "message_0.db")} {
		copyDB, openErr := store.Open(filepath.Join(dir, rel), true)
		if openErr != nil {
			t.Fatalf("decrypted snapshot %s did not open: %v", rel, openErr)
		}
		var got int
		queryErr := copyDB.QueryRow("PRAGMA user_version").Scan(&got)
		copyDB.Close()
		if queryErr != nil || got != 23 {
			t.Fatalf("decrypted snapshot %s data mismatch: %d %v", rel, got, queryErr)
		}
	}
}

func TestSnapshotKeyErrorsDoNotExposeSecrets(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "db_storage")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	var salt [16]byte
	var key [32]byte
	for i := range salt {
		salt[i] = byte(i + 3)
	}
	for i := range key {
		key[i] = byte(i + 90)
	}
	page := encryptSyntheticSQLite(append(bytes.Repeat([]byte{0}, cryptodb.PageSize-1), 0), salt, key)
	db := filepath.Join(source, "contact.db")
	if err := os.WriteFile(db, page, 0600); err != nil {
		t.Fatal(err)
	}
	c := config.Config{DataDir: source, Path: filepath.Join(root, "config", "config.json")}
	t.Setenv("WXMORI_MASTER_KEY", "")
	_, _, err := store.Snapshot(context.Background(), c, false)
	if err == nil || !strings.Contains(err.Error(), "KEY_REQUIRED") {
		t.Fatalf("expected missing-key failure, got %v", err)
	}
	if strings.Contains(err.Error(), hex.EncodeToString(key[:])) || strings.Contains(err.Error(), hex.EncodeToString(salt[:])) {
		t.Fatal("missing-key error exposed secret material")
	}

	// A present but incorrect key must fail page authentication without echoing either value.
	wrong := key
	wrong[0] ^= 0xff
	data, _ := json.Marshal(keyring.File{Version: 1, Account: keyring.AccountID(source), DatabaseKeys: map[string]string{hex.EncodeToString(salt[:]): hex.EncodeToString(wrong[:])}})
	if err = os.MkdirAll(filepath.Dir(c.CapturedKeyFile()), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(c.CapturedKeyFile(), data, 0600); err != nil {
		t.Fatal(err)
	}
	_, _, err = store.Snapshot(context.Background(), c, false)
	if err == nil || !strings.Contains(err.Error(), "HMAC_FAILED") {
		t.Fatalf("expected authentication failure, got %v", err)
	}
	if strings.Contains(err.Error(), hex.EncodeToString(wrong[:])) || strings.Contains(err.Error(), hex.EncodeToString(salt[:])) {
		t.Fatal("wrong-key error exposed secret material")
	}
}

func encryptSyntheticSQLite(plain []byte, salt [16]byte, encKey [32]byte) []byte {
	out := make([]byte, len(plain))
	copy(out, plain)
	block, _ := aes.NewCipher(encKey[:])
	macKey := cryptodb.MACKey(encKey[:], salt[:])
	defer clear(macKey)
	for offset, number := 0, uint32(1); offset < len(plain); offset, number = offset+cryptodb.PageSize, number+1 {
		page := out[offset : offset+cryptodb.PageSize]
		start := 0
		if number == 1 {
			start = 16
			copy(page[:16], salt[:])
		}
		iv := bytes.Repeat([]byte{byte(number + 19)}, aes.BlockSize)
		copy(page[cryptodb.BodySize:cryptodb.BodySize+aes.BlockSize], iv)
		cipher.NewCBCEncrypter(block, iv).CryptBlocks(page[start:cryptodb.BodySize], plain[offset+start:offset+cryptodb.BodySize])
		h := hmac.New(sha512.New, macKey)
		h.Write(page[start : cryptodb.PageSize-64])
		var pageNo [4]byte
		binary.LittleEndian.PutUint32(pageNo[:], number)
		h.Write(pageNo[:])
		copy(page[cryptodb.PageSize-64:], h.Sum(nil))
	}
	return out
}
