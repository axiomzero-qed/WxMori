package service_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/demodata"
	"github.com/axiomzero-qed/WxMori/internal/service"
)

func setup(t *testing.T) *service.App {
	t.Helper()
	p, err := demodata.Create(filepath.Join(t.TempDir(), "演示 Demo"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	a := service.New(c, nil)
	t.Cleanup(a.Close)
	return a
}
func asMap(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err = json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestShardSenderDedupAndMonthExport(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	v, err := a.Messages(ctx, service.MessageArgs{SessionID: demodata.SessionID, From: "2026-09-01", To: "2026-10-01"})
	if err != nil {
		t.Fatal(err)
	}
	out := asMap(t, v)
	if out["total"] != float64(9) {
		t.Fatalf("expected 9 September messages: %v", out["total"])
	}
	seenAlice, seenBob, repeats := false, false, 0
	for _, item := range out["messages"].([]any) {
		m := item.(map[string]any)
		text := m["text"].(string)
		if strings.Contains(text, "Hello, WxMori!") {
			if m["sender_id"] != "wxid_demo_alice" {
				t.Fatal("wrong shard sender")
			}
			seenAlice = true
		}
		if strings.Contains(text, "Testing from Bob") {
			if m["sender_id"] != "wxid_demo_bob" {
				t.Fatal("cross-shard rowid collision")
			}
			seenBob = true
		}
		if strings.Contains(text, "Repeated message") {
			repeats++
		}
		if strings.Contains(text, "<appmsg") || strings.HasPrefix(text, "wxid_demo_") {
			t.Fatal("undecoded content")
		}
		if strings.Contains(text, "Demo address") && !strings.Contains(text, "收件人: 测试用户\n地址:") {
			t.Fatal("multiline body damaged")
		}
	}
	if !seenAlice || !seenBob || repeats != 2 {
		t.Fatalf("message loss alice=%v bob=%v repeat=%d", seenAlice, seenBob, repeats)
	}
	v, err = a.Export(ctx, service.ExportArgs{SessionID: "微忆 Demo 群", Months: []string{"2026-09", "2026-08"}})
	if err != nil {
		t.Fatal(err)
	}
	export := asMap(t, v)
	if export["message_count"] != float64(9) {
		t.Fatal("export count mismatch")
	}
	files := export["files"].([]any)
	if len(files) != 1 {
		t.Fatal("unexpected files")
	}
	f := files[0].(map[string]any)
	body, err := os.ReadFile(f["path"].(string))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(body)
	if f["sha256"] != hex.EncodeToString(h[:]) {
		t.Fatal("hash mismatch")
	}
	if strings.Contains(string(body), "October boundary") {
		t.Fatal("month end is not exclusive")
	}
	if export["empty_months"].([]any)[0] != "2026-08" {
		t.Fatal("empty month not reported")
	}
}

func TestRecoveryAndDateBoundaries(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		call func() (any, error)
		code string
	}{
		{"ambiguous", func() (any, error) { return a.Export(ctx, service.ExportArgs{SessionID: "Twin Demo"}) }, "AMBIGUOUS_SESSION"},
		{"month", func() (any, error) {
			return a.Export(ctx, service.ExportArgs{SessionID: demodata.SessionID, Months: []string{"2026-13"}})
		}, "INVALID_MONTH"},
		{"range", func() (any, error) {
			return a.Messages(ctx, service.MessageArgs{SessionID: demodata.SessionID, From: "2026-10-01", To: "2026-09-01"})
		}, "INVALID_DATE_RANGE"},
		{"limit", func() (any, error) {
			return a.Messages(ctx, service.MessageArgs{SessionID: demodata.SessionID, Limit: 501})
		}, "INVALID_PAGINATION"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.call()
			if err == nil || service.AsFault(err).Code != tc.code {
				t.Fatalf("expected %s got %v", tc.code, err)
			}
		})
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	start, _ := service.ParseTime("2026-10-01", loc)
	utc, _ := service.ParseTime("2026-09-30T16:00:00Z", loc)
	if start != utc {
		t.Fatal("timezone applied twice")
	}
	v, err := a.Search(ctx, service.SearchArgs{Query: "testing bob", Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if asMap(t, v)["total"] != float64(1) {
		t.Fatal("AND search failed")
	}
	v, err = a.Messages(ctx, service.MessageArgs{SessionID: demodata.SessionID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	out := asMap(t, v)
	if out["next_offset"] != float64(2) || out["has_more"] != true {
		t.Fatal("pagination failed")
	}
}
