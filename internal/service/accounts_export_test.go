package service_test

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/service"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

func TestStatusInspectDoesNotInferIdentityFromNicknameAndWarnsOnSparseHistory(t *testing.T) {
	root := t.TempDir()
	storage := filepath.Join(root, "wxid_candidate_abcd", "db_storage")
	if err := os.MkdirAll(storage, 0700); err != nil {
		t.Fatal(err)
	}
	createSmallSyntheticAccount(t, storage, "wxid_other", "wxid_candidate")
	a := service.New(config.Config{
		DecryptedDir: storage,
		OutputDir:    filepath.Join(root, "exports"),
		Timezone:     "UTC",
		Path:         filepath.Join(root, "config.json"),
	}, nil)
	defer a.Close()

	v, err := a.StatusWithOptions(context.Background(), service.StatusArgs{Inspect: true})
	if err != nil {
		t.Fatal(err)
	}
	status := asMap(t, v)
	whoami, ok := status["whoami"].(map[string]any)
	if !ok {
		t.Fatalf("status omitted inspected identity: %v", status["whoami"])
	}
	if whoami["directory_id"] != "wxid_candidate_abcd" || whoami["internal_id_candidate"] != "wxid_candidate" {
		t.Fatalf("wrong directory identity metadata: %v", whoami)
	}
	if whoami["contact_match_verified"] != false || whoami["display_name"] != nil || whoami["wechat_alias"] != nil {
		t.Fatalf("identity was inferred from another contact's nickname: %v", whoami)
	}
	history, ok := status["history"].(map[string]any)
	if !ok || history["session_count"] != float64(1) || history["message_count"] != float64(1) || history["sparse_local_history"] != true {
		t.Fatalf("unexpected synthetic sparse-history summary: %v", status["history"])
	}
	diagnostics, ok := status["diagnostics"].([]any)
	if !ok || len(diagnostics) != 1 || diagnostics[0].(map[string]any)["code"] != "SPARSE_LOCAL_HISTORY" {
		t.Fatalf("expected one sparse-history diagnostic, got %v", status["diagnostics"])
	}
}

func createSmallSyntheticAccount(t *testing.T, dir, otherContactID, temptingNickname string) {
	t.Helper()
	contactDB, err := store.Open(filepath.Join(dir, "contact.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = contactDB.Exec(`CREATE TABLE contact(username TEXT,nick_name TEXT,remark TEXT,alias TEXT);
INSERT INTO contact VALUES(?,?,?,?);
INSERT INTO contact VALUES(?,?,?,?);`,
		otherContactID, temptingNickname, "", temptingNickname,
		"synthetic@chatroom", "Synthetic group", "", "")
	if err != nil {
		contactDB.Close()
		t.Fatal(err)
	}
	if err = contactDB.Close(); err != nil {
		t.Fatal(err)
	}

	messageDB, err := store.Open(filepath.Join(dir, "message_0.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	table := fmt.Sprintf("Msg_%x", md5.Sum([]byte("synthetic@chatroom")))
	_, err = messageDB.Exec(`CREATE TABLE Name2Id(user_name TEXT,is_session INTEGER)`)
	if err == nil {
		_, err = messageDB.Exec("INSERT INTO Name2Id(rowid,user_name,is_session) VALUES(1,?,0),(2,?,1)", otherContactID, "synthetic@chatroom")
	}
	if err == nil {
		_, err = messageDB.Exec(`CREATE TABLE ` + table + `(local_id INTEGER PRIMARY KEY,server_id INTEGER,local_type INTEGER,sort_seq INTEGER,real_sender_id INTEGER,create_time INTEGER,message_content BLOB,WCDB_CT_message_content INTEGER)`)
	}
	if err == nil {
		_, err = messageDB.Exec("INSERT INTO "+table+" VALUES(1,77,1,1,1,?,?,0)", time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC).Unix(), []byte("Synthetic test message"))
	}
	if err != nil {
		messageDB.Close()
		t.Fatal(err)
	}
	if err = messageDB.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExportAllDemoSnapshotManifestHashesAndCounts(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	for name, tc := range map[string]struct {
		args service.BatchExportArgs
		code string
	}{
		"bad month":         {service.BatchExportArgs{Months: []string{"2026-13"}, Format: "jsonl"}, "INVALID_MONTH"},
		"reversed range":    {service.BatchExportArgs{From: "2026-10-01", To: "2026-09-01"}, "INVALID_DATE_RANGE"},
		"conflicting dates": {service.BatchExportArgs{Months: []string{"2026-09"}, From: "2026-09-01"}, "CONFLICTING_DATES"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := a.ExportAll(ctx, tc.args)
			if err == nil || service.AsFault(err).Code != tc.code {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
		})
	}
	outputDir := a.Config.OutputDir
	if entries, err := os.ReadDir(outputDir); err == nil && len(entries) != 0 {
		t.Fatalf("invalid batch request published output: %v", entries)
	} else if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	v, err := a.ExportAll(ctx, service.BatchExportArgs{Months: []string{"2026-09"}, Format: "jsonl"})
	if err != nil {
		t.Fatal(err)
	}
	out := asMap(t, v)
	if out["session_count"] != float64(3) || out["message_count"] != float64(9) || out["file_count"] != float64(1) {
		t.Fatalf("batch totals differ from synthetic fixture: sessions=%v messages=%v files=%v", out["session_count"], out["message_count"], out["file_count"])
	}
	if out["files_preview_truncated"] != false {
		t.Fatalf("unexpected preview truncation for one synthetic file: %v", out["files_preview_truncated"])
	}
	preview, ok := out["files_preview"].([]any)
	if !ok || len(preview) != 1 || len(preview) > 50 {
		t.Fatalf("unexpected bounded file preview: %v", out["files_preview"])
	}

	dir := out["directory"].(string)
	manifestPath := out["manifest_path"].(string)
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	manifestHash := sha256.Sum256(manifestBytes)
	if out["manifest_sha256"] != hex.EncodeToString(manifestHash[:]) {
		t.Fatal("manifest SHA-256 differs from returned digest")
	}
	var manifest map[string]any
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["session_count"] != float64(3) || manifest["message_count"] != float64(9) || manifest["file_count"] != float64(1) {
		t.Fatalf("manifest totals differ from expected fixture counts: %v", manifest)
	}
	sessions, ok := manifest["sessions"].([]any)
	if !ok || len(sessions) != 3 {
		t.Fatalf("manifest did not enumerate all synthetic sessions: %v", manifest["sessions"])
	}
	fileCount := 0
	for _, rawSession := range sessions {
		session := rawSession.(map[string]any)
		for _, rawFile := range session["files"].([]any) {
			file := rawFile.(map[string]any)
			fileCount++
			path := file["path"].(string)
			rel, err := filepath.Rel(dir, path)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
				t.Fatalf("manifest file escaped batch directory: %q", path)
			}
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(body)
			if file["sha256"] != hex.EncodeToString(digest[:]) {
				t.Fatalf("file hash mismatch for manifest entry %d", fileCount)
			}
			if file["message_count"] != float64(9) {
				t.Fatalf("unexpected per-file message count: %v", file["message_count"])
			}
		}
	}
	if fileCount != 1 {
		t.Fatalf("manifest file count mismatch: %d", fileCount)
	}
}

func TestExportAllCancellationPublishesNoBatch(t *testing.T) {
	a := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := a.ExportAll(ctx, service.BatchExportArgs{Months: []string{"2026-09"}, Format: "txt"})
	if err == nil {
		t.Fatal("expected cancelled batch to fail")
	}
	entries, readErr := os.ReadDir(a.Config.OutputDir)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "export-all-") || strings.HasPrefix(entry.Name(), ".wxmori-batch-staging-") {
			t.Fatalf("cancelled batch left published or staging directory %q", entry.Name())
		}
	}
}
