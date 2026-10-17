package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/fault"
)

func errorCode(err error) string {
	var f *fault.Error
	if errors.As(err, &f) {
		return f.Code
	}
	return ""
}

func TestSaveWithOptionsRequiresExplicitAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	original := config.Config{DecryptedDir: "first-source", OutputDir: "first-exports", Timezone: "UTC"}
	if err := config.SaveWithOptions(path, original, false); err != nil {
		t.Fatalf("initial save failed: %v", err)
	}
	originalBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	replacement := config.Config{DecryptedDir: "second-source", OutputDir: "second-exports", Timezone: "Asia/Shanghai"}
	err = config.SaveWithOptions(path, replacement, false)
	if errorCode(err) != "CONFIG_EXISTS" {
		t.Fatalf("expected CONFIG_EXISTS without replace, got %v", err)
	}
	afterCollision, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterCollision) != string(originalBytes) {
		t.Fatal("config collision changed the original file")
	}

	if err = config.SaveWithOptions(path, replacement, true); err != nil {
		t.Fatalf("explicit replacement failed: %v", err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DecryptedDir != filepath.Join(filepath.Dir(path), replacement.DecryptedDir) || loaded.OutputDir != filepath.Join(filepath.Dir(path), replacement.OutputDir) || loaded.Timezone != replacement.Timezone {
		t.Fatalf("replacement config was not persisted: %+v", loaded)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".wxmori-config-") {
			t.Fatalf("replacement left temporary file %q", entry.Name())
		}
	}
}

func TestSaveWrapperDefaultsToNoReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	first := config.Config{DecryptedDir: "source-one", OutputDir: "exports", Timezone: "UTC"}
	second := config.Config{DecryptedDir: "source-two", OutputDir: "exports", Timezone: "UTC"}
	if err := config.Save(path, first); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = config.Save(path, second); errorCode(err) != "CONFIG_EXISTS" {
		t.Fatalf("Save should preserve the no-replace default, got %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("default Save collision modified the existing config")
	}
}

func TestSaveWithOptionsFailedReplacementLeavesExistingPathUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(path, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	err := config.SaveWithOptions(path, config.Config{DecryptedDir: "synthetic", Timezone: "UTC"}, true)
	if errorCode(err) != "CONFIG_WRITE_FAILED" {
		t.Fatalf("expected atomic replacement failure to be reported, got %v", err)
	}
	contents, readErr := os.ReadFile(sentinel)
	if readErr != nil || string(contents) != "unchanged" {
		t.Fatalf("failed replacement damaged existing path: %v", readErr)
	}
	entries, readErr := os.ReadDir(filepath.Dir(path))
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".wxmori-config-") {
			t.Fatalf("failed replacement left temporary file %q", entry.Name())
		}
	}
}
