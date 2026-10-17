//go:build windows

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestInitDoesNotOpenOrLockDatabaseFiles(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "account", "db_storage")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(source, "contact.db")
	if err := os.WriteFile(locked, []byte("synthetic locked database placeholder"), 0600); err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(windows.StringToUTF16Ptr(locked), windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("could not establish synthetic exclusive database lock: %v", err)
	}
	defer windows.CloseHandle(h)

	configPath := filepath.Join(root, "config.json")
	code, _, stderr := invokeCLI(t, "--config", configPath, "init", "--data-dir", source, "--output-dir", filepath.Join(root, "exports"))
	if code != 0 {
		t.Fatalf("init inspected a locked database during configuration: %q", stderr)
	}
}
