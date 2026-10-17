package keycapture

import (
	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIncompleteDatabaseDoesNotDisappearFromCapture(t *testing.T) {
	dir := t.TempDir()
	target, _ := fixtureTarget()
	if err := os.WriteFile(filepath.Join(dir, "contact.db"), target.Page, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "message_0.db"), target.Page[:64], 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := targets(dir); err == nil {
		t.Fatal("partly created database was silently excluded")
	} else if f, ok := err.(*fault.Error); !ok || f.Code != "CAPTURE_DATABASE_NOT_READY" {
		t.Fatalf("unexpected error %v", err)
	}
}

func TestPublishOnlyCurrentVerifiedKeysAndReplacesChangedMaterial(t *testing.T) {
	if !platformSupported() {
		t.Skip("DPAPI capture persistence is Windows-only")
	}
	m := New(config.Config{DataDir: t.TempDir(), Path: filepath.Join(t.TempDir(), "config.json")})
	target, key := fixtureTarget()
	unknown := target.Salt
	unknown[0] ^= 0x80
	keys := map[[16]byte][32]byte{target.Salt: key, unknown: key}
	var saved [32]byte
	check := func(expected [32]byte) {
		t.Helper()
		b, err := os.ReadFile(m.c.CapturedKeyFile())
		if err != nil {
			t.Fatal(err)
		}
		material, err := keyring.Decode(b, keyring.AccountID(m.c.DataDir))
		if err != nil {
			t.Fatal(err)
		}
		defer material.Clear()
		if len(material.BySalt) != 1 || material.BySalt[target.Salt] != expected {
			t.Fatal("saved key set is stale or includes unrelated material")
		}
	}
	if err := m.publish([]Target{target}, keys, &saved); err != nil {
		t.Fatal(err)
	}
	check(key)
	previous := saved
	key[0] ^= 1
	target = encryptedTarget(key, target.Salt)
	keys[target.Salt] = key
	if err := m.publish([]Target{target}, keys, &saved); err != nil {
		t.Fatal(err)
	}
	if previous == saved {
		t.Fatal("changed key material was ignored because the count stayed the same")
	}
	check(key)
}

func TestManagerStopAndTimeoutWithoutReadingRealWeChat(t *testing.T) {
	if !platformSupported() {
		t.Skip("Windows capture is intentionally unavailable here")
	}
	dir := t.TempDir()
	target, _ := fixtureTarget()
	for _, name := range []string{"contact.db", "message_0.db"} {
		if e := os.WriteFile(filepath.Join(dir, name), target.Page, 0600); e != nil {
			t.Fatal(e)
		}
	}
	m := New(config.Config{DataDir: dir, Path: filepath.Join(t.TempDir(), "config.json")})
	defer m.Close()
	// The test backend must never enumerate or read the user's real processes.
	m.enumerate = func() ([]*process, int, error) { return nil, 0, nil }
	if _, e := m.Start(601 * time.Second); e == nil {
		t.Fatal("unbounded duration accepted")
	}
	s, e := m.Start(time.Second)
	if e != nil || !s.Running {
		t.Fatalf("start failed %v", e)
	}
	firstStart := s.StartedAt
	s, e = m.Start(time.Second)
	if e != nil || s.StartedAt != firstStart {
		t.Fatal("duplicate start replaced a running job")
	}
	if s = m.Stop(); s.Running || s.State != "stopped" {
		t.Fatalf("stop failed %+v", s)
	}
	if _, e = m.Start(time.Second); e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(4 * time.Second)
	for m.Status().Running && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	s = m.Status()
	if s.Running || s.State != "timeout" || s.VerifiedDatabases != 0 {
		t.Fatalf("timeout looked successful %+v", s)
	}
	if _, e = os.Stat(m.c.CapturedKeyFile()); !os.IsNotExist(e) {
		t.Fatal("unverified keys were saved")
	}
}
