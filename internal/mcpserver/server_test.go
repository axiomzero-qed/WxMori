package mcpserver_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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
	if len(listed.Tools) != 9 {
		t.Fatalf("expected nine tools, got %d", len(listed.Tools))
	}
	found := false
	names := map[string]bool{}
	for _, tool := range listed.Tools {
		names[tool.Name] = true
		if tool.Name == "list_accounts" || tool.Name == "export_all_chats" {
			continue
		}
		if tool.Name == "wxmori_status" {
			b, _ := json.Marshal(tool.InputSchema)
			var schema map[string]any
			if err := json.Unmarshal(b, &schema); err != nil {
				t.Fatal(err)
			}
			props := schema["properties"].(map[string]any)
			inspect := props["inspect"].(map[string]any)
			if inspect["type"] != "boolean" {
				t.Fatalf("inspect should be a boolean: %v", inspect)
			}
			if def, ok := inspect["default"]; ok && def != false {
				t.Fatalf("inspect schema default must be false: %v", def)
			}
			description, _ := inspect["description"].(string)
			if !strings.Contains(strings.ToLower(description), "default false") && !strings.Contains(description, "默认 false") {
				t.Fatalf("inspect schema should explain default false: %q", description)
			}
			if required, ok := schema["required"].([]any); ok && contains(required, "inspect") {
				t.Fatal("inspect must remain optional")
			}
		}
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
	for _, name := range []string{"wxmori_status", "list_accounts", "capture_key", "list_sessions", "get_messages", "search_messages", "get_contacts", "export_chat", "export_all_chats"} {
		if !names[name] {
			t.Fatalf("tool %q missing", name)
		}
	}
	callStatus := func(args map[string]any) map[string]any {
		t.Helper()
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "wxmori_status", Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		if r.IsError {
			t.Fatalf("status call failed: %+v", r)
		}
		b, err := json.Marshal(r.StructuredContent)
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		if err = json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	plainStatus := callStatus(map[string]any{})
	if plainStatus["snapshot_state"] != "not_loaded" || plainStatus["history"] != nil {
		t.Fatalf("inspect should default false: %v", plainStatus)
	}
	inspectStatus := callStatus(map[string]any{"inspect": true})
	if _, ok := inspectStatus["whoami"].(map[string]any); !ok {
		t.Fatalf("inspect omitted whoami: %v", inspectStatus)
	}
	if _, ok := inspectStatus["history"].(map[string]any); !ok {
		t.Fatalf("inspect omitted history: %v", inspectStatus)
	}
	if _, ok := inspectStatus["diagnostics"].([]any); !ok {
		t.Fatalf("inspect omitted diagnostics: %v", inspectStatus)
	}
	if inspectStatus["snapshot_state"] != "ready" {
		t.Fatalf("inspect was not accepted: %v", inspectStatus)
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
	call(map[string]any{"action": "start", "mode": "unknown"}, "INVALID_ARGUMENTS")
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

func contains(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
