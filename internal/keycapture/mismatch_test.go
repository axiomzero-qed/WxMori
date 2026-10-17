package keycapture

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/fault"
)

func TestAccountMismatchRequiresAuthenticatedOtherContactAndMessage(t *testing.T) {
	first, key := fixtureTarget()
	first.Name = "contact/contact.db"
	salt := first.Salt
	salt[0] ^= 0x71
	second := encryptedTarget(key, salt)
	probe := NewProbe([]Target{first, second})
	diag := mismatchProbe{alternatives: []alternative{{config.Account{DirectoryID: "wxid_synthetic_abcd", DataDir: filepath.Join(t.TempDir(), "db_storage")}, probe}}}
	defer diag.Clear()
	start := time.Unix(10000, 0)
	var unrelated [32]byte
	diag.observe(unrelated)
	if len(diag.matches()) != 0 || diag.check(start.Add(time.Hour), 0) != nil {
		t.Fatal("ordinary AES misses were reported as account mismatch")
	}
	// One page is not enough evidence to stop an unrelated capture.
	probe.Targets = probe.Targets[:1]
	diag.observe(key)
	if len(diag.matches()) != 0 || diag.check(start, 0) != nil {
		t.Fatal("one database became a confirmed account mismatch")
	}
	probe.Targets = append(probe.Targets, second)
	clear(probe.seen)
	diag.observe(key)
	if len(diag.matches()) != 1 {
		t.Fatal("two authenticated database pages did not establish other-account evidence")
	}
	if err := diag.check(start, 0); err != nil {
		t.Fatal("no settling grace period")
	}
	if err := diag.check(start.Add(9*time.Second), 0); err != nil {
		t.Fatal("grace period ended early")
	}
	err := diag.check(start.Add(10*time.Second), 0)
	f, ok := err.(*fault.Error)
	if !ok || f.Code != "KEY_ACCOUNT_MISMATCH" {
		t.Fatalf("confirmed mismatch not diagnosed: %v", err)
	}
	b, _ := json.Marshal(f)
	if strings.Contains(string(b), hex.EncodeToString(key[:])) {
		t.Fatal("diagnostic exposed key material")
	}
	// A selected-account success suppresses the mismatch even with background
	// activity from another account; this must not stop a valid partial capture.
	if err := diag.check(start.Add(time.Hour), 1); err != nil {
		t.Fatal("selected-account success was ignored")
	}
	if err := diag.check(start.Add(time.Hour), 0); err != nil {
		t.Fatal("grace period was not reset")
	}
	manager := New(config.Config{})
	manager.fail(f)
	status := manager.Status()
	if status.State != "account_mismatch" || status.Error != f || status.ReadyForLogin {
		t.Fatal("manager lost explicit mismatch result")
	}
}

func TestLoginCandidateCacheKeepsRecentKeysAndClears(t *testing.T) {
	var cache candidateCache
	var first, last [32]byte
	for i := 0; i < 3000; i++ {
		var key [32]byte
		binary.LittleEndian.PutUint64(key[:], uint64(i))
		if i == 0 {
			first = key
		}
		last = key
		if !cache.add(key) {
			t.Fatal("new event ignored")
		}
	}
	if cache.count != 2048 || cache.present[first] || !cache.present[last] || cache.add(last) {
		t.Fatal("rolling cache saturated or repeated keys were accepted")
	}
	cache.clear()
	if cache.count != 0 || len(cache.present) != 0 {
		t.Fatal("cache retained candidates")
	}
	for _, key := range cache.values {
		if key != ([32]byte{}) {
			t.Fatal("cache memory was not cleared")
		}
	}
}

func TestMismatchReloadRetainsOnlyReauthenticatedEvidence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := filepath.Join(home, "Documents", "xwechat_files")
	selected := filepath.Join(root, "wxid_selected_abcd", "db_storage")
	other := filepath.Join(root, "wxid_other_7f3a", "db_storage")
	var selectedKey, otherKey [32]byte
	selectedKey[0] = 0x65
	otherKey[0] = 0x86
	write := func(dir string, key [32]byte, first byte) {
		t.Helper()
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		for i, name := range []string{"contact.db", "message_0.db"} {
			var salt [16]byte
			salt[0] = first + byte(i)
			page := encryptedTarget(key, salt)
			if err := os.WriteFile(filepath.Join(dir, name), page.Page, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(selected, selectedKey, 1)
	write(other, otherKey, 3)
	c := config.Config{DataDir: selected, Path: filepath.Join(t.TempDir(), "config.json")}
	diag := mismatchProbe{}
	defer diag.Clear()
	diag.reload(c)
	diag.observe(otherKey)
	start := time.Unix(1000, 0)
	if len(diag.matches()) != 1 || diag.check(start, 0) != nil {
		t.Fatal("synthetic other-account evidence unavailable")
	}
	// File-page refresh must preserve evidence after the recent-event cache
	// has moved on, but only if it authenticates again against current pages.
	diag.reload(c)
	if len(diag.matches()) != 1 || diag.check(start.Add(11*time.Second), 0) == nil {
		t.Fatal("valid evidence was lost during refresh")
	}
	write(other, selectedKey, 9)
	diag.reload(c)
	if len(diag.matches()) != 0 || diag.check(start.Add(time.Hour), 0) != nil {
		t.Fatal("stale foreign-account keys survived changed database salts")
	}
	if _, err := os.Stat(c.CapturedKeyFile()); !os.IsNotExist(err) {
		t.Fatal("diagnosis persisted another account's keys")
	}
}
