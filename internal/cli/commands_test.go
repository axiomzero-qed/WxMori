package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/buildinfo"
	"github.com/axiomzero-qed/WxMori/internal/demodata"
)

func invokeCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Main(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestVersionFlagAliases(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"-v"}, {"version"}, {"--version", "status"}} {
		code, out, _ := invokeCLI(t, args...)
		if code != 0 || strings.TrimSpace(out) != buildinfo.Version {
			t.Errorf("args %v: code=%d output=%q, want version %q", args, code, out, buildinfo.Version)
		}
	}
}

func TestInitRequiresForceToReplaceConfig(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "db_storage")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(root, "config.json")
	args := []string{"--config", configPath, "init", "--decrypted-dir", source, "--output-dir", filepath.Join(root, "exports")}
	if code, _, _ := invokeCLI(t, args...); code != 0 {
		t.Fatalf("initial init returned %d", code)
	}
	original, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	replacementSource := filepath.Join(root, "replacement-source")
	if err := os.MkdirAll(replacementSource, 0700); err != nil {
		t.Fatal(err)
	}
	args[4] = replacementSource
	code, _, stderr := invokeCLI(t, args...)
	if code == 0 || !strings.Contains(stderr, "CONFIG_EXISTS") {
		t.Fatalf("replacement without --force: code=%d error=%q", code, stderr)
	}
	unchanged, err := os.ReadFile(configPath)
	if err != nil || !bytes.Equal(original, unchanged) {
		t.Fatalf("collision changed original config: read err=%v", err)
	}
	args = append(args, "--force")
	code, _, stderr = invokeCLI(t, args...)
	if code != 0 {
		t.Fatalf("forced replacement failed: %q", stderr)
	}
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		DecryptedDir string `json:"decrypted_dir"`
	}
	if err = json.Unmarshal(data, &got); err != nil || got.DecryptedDir != args[4] {
		t.Fatalf("forced replacement source = %q, err=%v", got.DecryptedDir, err)
	}
}

func TestAccountsCustomRootAndInspectCommands(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "synthetic_xwechat", "wxid_synthetic_1234", "db_storage")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	code, out, stderr := invokeCLI(t, "accounts", "--root", filepath.Dir(filepath.Dir(source)))
	if code != 0 {
		t.Fatalf("accounts: %q", stderr)
	}
	var accounts struct {
		Accounts []struct {
			DataDir string `json:"data_dir"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(out), &accounts); err != nil {
		t.Fatal(err)
	}
	if len(accounts.Accounts) != 1 || accounts.Accounts[0].DataDir != source {
		t.Fatalf("custom-root account discovery returned %d accounts", len(accounts.Accounts))
	}

	demoConfig, err := demodata.Create(filepath.Join(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{{"status", "--inspect"}, {"whoami"}} {
		code, out, stderr = invokeCLI(t, append([]string{"--config", demoConfig}, command...)...)
		if code != 0 {
			t.Fatalf("%s: %q", strings.Join(command, " "), stderr)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("%s JSON: %v", command[0], err)
		}
		if result["whoami"] == nil || result["history"] == nil {
			t.Fatalf("%s omitted identity/history inspection", command[0])
		}
	}
}

func TestExportAllConflictsWithSelectedSession(t *testing.T) {
	root := t.TempDir()
	demoConfig, err := demodata.Create(filepath.Join(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := invokeCLI(t, "--config", demoConfig, "export", "--all-sessions", "--session", "synthetic-session")
	if code == 0 || !strings.Contains(stderr, "CONFLICTING_SESSIONS") {
		t.Fatalf("expected clear all/session conflict, code=%d error=%q", code, stderr)
	}
}
