package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

type BatchExportArgs struct {
	Months []string `json:"months,omitempty" jsonschema:"Optional YYYY-MM months; omit for all dates / 可选月份 YYYY-MM，省略表示全部日期"`
	From   string   `json:"from,omitempty" jsonschema:"Inclusive date, not combined with months / 包含的起始日期，不与 months 同用"`
	To     string   `json:"to,omitempty" jsonschema:"Exclusive date, not combined with months / 不包含的结束日期，不与 months 同用"`
	Format string   `json:"format,omitempty" jsonschema:"txt (default) or jsonl / 默认 txt 或 jsonl"`
}

type sessionExport struct {
	Session      store.Session `json:"session"`
	MessageCount int           `json:"message_count"`
	Files        []ExportFile  `json:"files"`
	EmptyMonths  []string      `json:"empty_months"`
}

// ExportAll publishes a single batch directory only after every session succeeds.
func (a *App) ExportAll(ctx context.Context, arg BatchExportArgs) (any, error) {
	if arg.Format == "" {
		arg.Format = "txt"
	}
	if arg.Format != "txt" && arg.Format != "jsonl" {
		return nil, fault.New("INVALID_FORMAT", "导出格式无效 / Invalid export format", "Use txt or jsonl.")
	}
	if len(arg.Months) > 120 {
		return nil, fault.New("TOO_MANY_MONTHS", "月份过多 / Too many months", "Request at most 120 months per call.")
	}
	if len(arg.Months) > 0 && (arg.From != "" || arg.To != "") {
		return nil, fault.New("CONFLICTING_DATES", "日期参数冲突 / Conflicting date options", "Use months OR from/to.")
	}
	loc, err := a.Config.Location()
	if err != nil {
		return nil, err
	}
	if _, err = dateFilter(arg.From, arg.To, loc); err != nil {
		return nil, err
	}
	for _, month := range arg.Months {
		t, e := time.ParseInLocation("2006-01", month, loc)
		if e != nil || t.Format("2006-01") != month {
			return nil, fault.New("INVALID_MONTH", "月份格式无效 / Invalid month", "Use YYYY-MM, for example 2026-09.")
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err = a.ensure(ctx); err != nil {
		return nil, err
	}
	sessions, err := a.data.Sessions(ctx, "", loc)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(a.Config.OutputDir, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(a.Config.OutputDir, ".wxmori-batch-staging-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	dest := filepath.Join(a.Config.OutputDir, "export-all-"+time.Now().UTC().Format("20060102T150405Z")+"-"+strings.TrimPrefix(filepath.Base(stage), ".wxmori-batch-staging-"))
	results := []sessionExport{}
	preview := []ExportFile{}
	total, files := 0, 0
	for _, session := range sessions {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		v, e := a.exportOneLocked(ctx, ExportArgs{SessionID: session.ID, Months: arg.Months, From: arg.From, To: arg.To, Format: arg.Format}, stage)
		if e != nil {
			return nil, e
		}
		out := v.(map[string]any)
		item := sessionExport{Session: session, MessageCount: out["message_count"].(int), Files: out["files"].([]ExportFile), EmptyMonths: out["empty_months"].([]string)}
		if len(item.Files) > 0 {
			folder := filepath.Join(stage, batchSessionDirectory(session))
			if e := os.Rename(filepath.Dir(item.Files[0].Path), folder); e != nil {
				return nil, e
			}
			for i := range item.Files {
				item.Files[i].Path = filepath.Join(folder, filepath.Base(item.Files[i].Path))
			}
		}
		for i := range item.Files {
			rel, e := filepath.Rel(stage, item.Files[i].Path)
			if e != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
				return nil, fault.New("INVALID_EXPORT_PATH", "导出路径无效 / Invalid export path", "No batch was published.")
			}
			item.Files[i].Path = filepath.Join(dest, rel)
			if len(preview) < 50 {
				preview = append(preview, item.Files[i])
			}
		}
		total += item.MessageCount
		files += len(item.Files)
		results = append(results, item)
	}
	manifest := envelope(a.data, a.Config.Timezone)
	manifest["sessions"], manifest["session_count"], manifest["message_count"], manifest["file_count"], manifest["format"], manifest["exported_at"] = results, len(results), total, files, arg.Format, time.Now().UTC().Format(time.RFC3339)
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	if err = os.WriteFile(filepath.Join(stage, "manifest.json"), b, 0600); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = os.Rename(stage, dest); err != nil {
		return nil, err
	}
	hash := sha256.Sum256(b)
	out := envelope(a.data, a.Config.Timezone)
	out["directory"], out["manifest_path"], out["manifest_sha256"] = dest, filepath.Join(dest, "manifest.json"), hex.EncodeToString(hash[:])
	out["session_count"], out["message_count"], out["file_count"], out["format"] = len(results), total, files, arg.Format
	out["files_preview"], out["files_preview_truncated"] = preview, files > len(preview)
	return out, nil
}

func batchSessionDirectory(session store.Session) string {
	h := sha256.Sum256([]byte(session.ID))
	name := []rune{}
	for _, r := range session.DisplayName {
		if len(name) >= 40 {
			break
		}
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			r = '_'
		}
		name = append(name, r)
	}
	return "chat-" + hex.EncodeToString(h[:6]) + "-" + strings.Trim(string(name), " .")
}
