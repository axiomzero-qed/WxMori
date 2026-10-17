package service

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

type ExportFile struct {
	Path         string `json:"path"`
	Month        string `json:"month"`
	MessageCount int    `json:"message_count"`
	SHA256       string `json:"sha256"`
}

func (a *App) Export(ctx context.Context, arg ExportArgs) (any, error) {
	if err := validateExportShape(&arg); err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.ensure(ctx); err != nil {
		return nil, err
	}
	return a.exportOneLocked(ctx, arg, a.Config.OutputDir)
}

func validateExportShape(arg *ExportArgs) error {
	if arg.Format == "" {
		arg.Format = "txt"
	}
	if arg.Format != "txt" && arg.Format != "jsonl" {
		return fault.New("INVALID_FORMAT", "导出格式无效 / Invalid export format", "Use txt or jsonl.")
	}
	if len(arg.Months) > 120 {
		return fault.New("TOO_MANY_MONTHS", "月份过多 / Too many months", "Request at most 120 months per call.")
	}
	if len(arg.Months) > 0 && (arg.From != "" || arg.To != "") {
		return fault.New("CONFLICTING_DATES", "日期参数冲突 / Conflicting date options", "Use months OR from/to, not both.")
	}
	return nil
}

func (a *App) exportOneLocked(ctx context.Context, arg ExportArgs, outputDir string) (any, error) {
	if err := validateExportShape(&arg); err != nil {
		return nil, err
	}
	loc, _ := a.Config.Location()
	session, err := a.data.Resolve(ctx, arg.SessionID, loc)
	if err != nil {
		return nil, err
	}
	f, err := dateFilter(arg.From, arg.To, loc)
	if err != nil {
		return nil, err
	}
	f.SessionID = session.ID
	months := map[string]bool{}
	for _, m := range arg.Months {
		t, e := time.ParseInLocation("2006-01", m, loc)
		if e != nil || t.Format("2006-01") != m {
			return nil, fault.New("INVALID_MONTH", "月份格式无效 / Invalid month", "Use YYYY-MM, for example 2026-09.")
		}
		months[m] = true
	}
	if err = os.MkdirAll(outputDir, 0700); err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(outputDir, ".wxmori-staging-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(stage)
	result := []ExportFile{}
	var current string
	var file *os.File
	var writer *bufio.Writer
	count := 0
	finish := func() error {
		if file == nil {
			return nil
		}
		e := writer.Flush()
		e2 := file.Close()
		file = nil
		if e != nil {
			return e
		}
		if e2 != nil {
			return e2
		}
		path := filepath.Join(stage, current+"."+arg.Format)
		in, e := os.Open(path)
		if e != nil {
			return e
		}
		h := sha256.New()
		_, e = io.Copy(h, in)
		in.Close()
		if e != nil {
			return e
		}
		result = append(result, ExportFile{Month: current, MessageCount: count, SHA256: hex.EncodeToString(h.Sum(nil))})
		return nil
	}
	defer func() {
		if file != nil {
			file.Close()
		}
	}()
	err = a.data.Visit(ctx, f, -1, 0, loc, func(m store.Message) error {
		month := time.Unix(m.Timestamp, 0).In(loc).Format("2006-01")
		if len(months) > 0 && !months[month] {
			return nil
		}
		if current != month {
			if e := finish(); e != nil {
				return e
			}
			current = month
			count = 0
			var e error
			file, e = os.OpenFile(filepath.Join(stage, month+"."+arg.Format), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if e != nil {
				return e
			}
			writer = bufio.NewWriter(file)
			if arg.Format == "txt" {
				if _, e = fmt.Fprintf(writer, "%s (%s)\n%s | %s\n聊天记录 / Chat history\n============================================================\n\n", strings.ReplaceAll(session.DisplayName, "\n", " "), session.ID, month, a.Config.Timezone); e != nil {
					return e
				}
			}
		}
		if arg.Format == "jsonl" {
			if e := json.NewEncoder(writer).Encode(m); e != nil {
				return e
			}
		} else {
			sender := m.Sender
			if m.Type == "system" {
				sender = "[系统 / System]"
			}
			if _, e := fmt.Fprintf(writer, "[%s] %s: %s\n", time.Unix(m.Timestamp, 0).In(loc).Format("2006-01-02 15:04:05"), strings.ReplaceAll(sender, "\n", " "), m.Text); e != nil {
				return e
			}
		}
		count++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = finish(); err != nil {
		return nil, err
	}
	// Empty requested months are explicit metadata, not fabricated chat files.
	empty := []string{}
	for month := range months {
		found := false
		for _, f := range result {
			if f.Month == month {
				found = true
				break
			}
		}
		if !found {
			empty = append(empty, month)
		}
	}
	sort.Strings(empty)
	out := envelope(a.data, a.Config.Timezone)
	out["session"] = session
	out["exported_at"] = time.Now().UTC().Format(time.RFC3339)
	out["format"] = arg.Format
	out["empty_months"] = empty
	if len(result) > 0 {
		base := "export-" + time.Now().UTC().Format("20060102T150405Z") + "-" + strings.TrimPrefix(filepath.Base(stage), ".wxmori-staging-")
		dest := filepath.Join(outputDir, base)
		if err = os.Rename(stage, dest); err != nil {
			return nil, err
		}
		for i := range result {
			result[i].Path = filepath.Join(dest, result[i].Month+"."+arg.Format)
		}
	}
	total := 0
	for _, file := range result {
		total += file.MessageCount
	}
	out["files"] = result
	out["message_count"] = total
	return out, nil
}
