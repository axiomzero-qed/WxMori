package mcpserver_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/demodata"
	"github.com/axiomzero-qed/WxMori/internal/mcpserver"
	"github.com/axiomzero-qed/WxMori/internal/service"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Exercise the registered protocol handlers without opening any real process.
// The independent stdio smoke test remains a separate required check.
func TestCaptureToolProtocolWithSyntheticSource(t *testing.T) {
	p, err := demodata.Create(filepath.Join(t.TempDir(), "合成 Demo"))
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(p)
	if err != nil {
		t.Fatal(err)
	}
	app := service.New(c, nil)
	defer app.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	st, ct := mcp.NewInMemoryTransports()
	ss, err := mcpserver.New(app).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "wxmori-synthetic-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	listed, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 7 {
		t.Fatalf("expected seven tools, got %d", len(listed.Tools))
	}
	found := false
	for _, tool := range listed.Tools {
		if tool.Name != "capture_key" {
			continue
		}
		found = true
		b, _ := json.Marshal(tool.InputSchema)
		var schema map[string]any
		if err := json.Unmarshal(b, &schema); err != nil {
			t.Fatal(err)
		}
		props := schema["properties"].(map[string]any)
		action := props["action"].(map[string]any)
		if !reflect.DeepEqual(action["enum"], []any{"start", "status", "stop"}) || schema["additionalProperties"] != false {
			t.Fatal("capture schema must advertise its three flat actions")
		}
	}
	if !found {
		t.Fatal("capture tool missing")
	}
	call := func(args map[string]any, code string) map[string]any {
		t.Helper()
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "capture_key", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if r.IsError != (code != "") || len(r.Content) != 1 {
			t.Fatalf("unexpected error/content: %+v", r)
		}
		var body map[string]any
		if err := json.Unmarshal([]byte(r.Content[0].(*mcp.TextContent).Text), &body); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(r.StructuredContent)
		var structured map[string]any
		if err := json.Unmarshal(b, &structured); err != nil || !reflect.DeepEqual(body, structured) {
			t.Fatal("text and structured output differ")
		}
		if code != "" && body["error"].(map[string]any)["code"] != code {
			t.Fatalf("expected %s, got %v", code, body)
		}
		return body
	}
	if call(map[string]any{"action": "status"}, "")["state"] != "idle" {
		t.Fatal("new capture should be idle")
	}
	call(map[string]any{}, "INVALID_ARGUMENTS")
	call(map[string]any{"action": "other"}, "INVALID_ARGUMENTS")
	call(map[string]any{"action": "start", "timeout_seconds": "180"}, "INVALID_ARGUMENTS")
	call(map[string]any{"action": "start", "timeout_seconds": 601}, "INVALID_CAPTURE_TIMEOUT")
	call(map[string]any{"action": "start", "password": "never-send-secrets"}, "INVALID_ARGUMENTS")
	code := "CAPTURE_UNSUPPORTED"
	if runtime.GOOS == "windows" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") {
		code = "CAPTURE_SOURCE_REQUIRED"
	}
	call(map[string]any{"action": "start"}, code)
	if call(map[string]any{"action": "stop"}, "")["running"] != false {
		t.Fatal("idle stop started a job")
	}
}
