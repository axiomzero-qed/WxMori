package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/buildinfo"
	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/demodata"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keycapture"
	"github.com/axiomzero-qed/WxMori/internal/mcpserver"
	"github.com/axiomzero-qed/WxMori/internal/service"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

const Version = buildinfo.Version
const help = `WxMori / 微忆 — local WeChat export for MCP / 本地微信导出
Usage / 用法: wxmori [--config PATH] COMMAND [OPTIONS]
  init       Configure one local source / 配置一个本地数据源
  status     Check configuration / 检查配置
  serve      Start MCP over stdio / 启动 stdio MCP
  sessions   Find chats (--keyword NAME) / 查找会话
  export     Save files (--session ID --months YYYY-MM) / 导出文件
  decrypt    Save a verified SQLite copy (--out NEW_DIR) / 保存校验过的明文副本
  key capture  Capture this account's keys during Windows login / Windows 登录期间采集本账号密钥
  demo       Create fictional test data (--dir NEW_DIR) / 创建合成测试数据
  version    Show version / 查看版本
Keys stay in a local key file or WXMORI_MASTER_KEY. / 密钥放本地文件或环境变量。
`

func Main(args []string, out, errOut io.Writer) int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, args, out, errOut); err != nil {
		_ = json.NewEncoder(errOut).Encode(map[string]any{"error": service.AsFault(err)})
		return 1
	}
	return 0
}
func run(ctx context.Context, args []string, out, errOut io.Writer) error {
	g := flag.NewFlagSet("wxmori", flag.ContinueOnError)
	g.SetOutput(errOut)
	path := g.String("config", "", "Config path / 配置路径")
	g.Usage = func() { fmt.Fprint(out, help) }
	if err := g.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return fault.New("INVALID_ARGUMENTS", "命令行参数无效 / Invalid CLI arguments", "Run wxmori --help.")
	}
	args = g.Args()
	if len(args) == 0 {
		fmt.Fprint(out, help)
		return nil
	}
	command, args := args[0], args[1:]
	if command == "key" {
		if len(args) == 0 || args[0] != "capture" {
			return fault.New("UNKNOWN_COMMAND", "未知密钥命令 / Unknown key command", "Use wxmori [--config PATH] key capture --timeout 180.")
		}
		command = "key-capture"
		args = args[1:]
	}
	if command == "version" || command == "--version" {
		fmt.Fprintln(out, Version)
		return nil
	}
	if command == "help" {
		fmt.Fprint(out, help)
		return nil
	}
	f := flag.NewFlagSet(command, flag.ContinueOnError)
	f.SetOutput(errOut)
	var dir, dataDir, decryptedDir, keyFile, outputDir, timezone, keyword, session, months, from, to, format string
	var captureTimeout int
	var captureMode string
	switch command {
	case "init":
		f.StringVar(&dataDir, "data-dir", "", "Account db_storage / 账号 db_storage")
		f.StringVar(&decryptedDir, "decrypted-dir", "", "Existing SQLite tree / 已解密目录")
		f.StringVar(&keyFile, "key-file", "", "Local key JSON / 本地密钥 JSON")
		f.StringVar(&outputDir, "output-dir", "", "Export directory / 导出目录")
		f.StringVar(&timezone, "timezone", "Asia/Shanghai", "IANA timezone / 时区")
	case "demo":
		f.StringVar(&dir, "dir", "./demo-data", "New demo directory / 新建演示目录")
	case "sessions":
		f.StringVar(&keyword, "keyword", "", "Name keyword / 名称关键词")
	case "export":
		f.StringVar(&session, "session", "", "Session id or unique name / 会话 ID 或唯一名称")
		f.StringVar(&months, "months", "", "Comma-separated YYYY-MM / 逗号分隔的月份")
		f.StringVar(&from, "from", "", "Inclusive date / 起始日期，包含")
		f.StringVar(&to, "to", "", "Exclusive date / 结束日期，不包含")
		f.StringVar(&format, "format", "txt", "txt or jsonl / txt 或 jsonl")
	case "decrypt":
		f.StringVar(&dir, "out", "", "New persistent plaintext directory / 新建明文输出目录")
	case "key-capture":
		f.IntVar(&captureTimeout, "timeout", 180, "Capture seconds, maximum 600 / 采集秒数，最多 600")
		f.StringVar(&captureMode, "mode", "auto", "auto or read_only / 即时捕获或只读轮询")
	case "status", "serve":
	default:
		return fault.New("UNKNOWN_COMMAND", "未知命令 / Unknown command", "Run wxmori --help.")
	}
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return fault.New("INVALID_ARGUMENTS", "命令参数无效 / Invalid command options", "Run wxmori "+command+" --help.")
	}
	if f.NArg() > 0 {
		return fault.New("INVALID_ARGUMENTS", "存在未识别参数 / Unexpected positional arguments", "Use named options and put --config before the command.")
	}
	printJSON := func(v any) error { e := json.NewEncoder(out); e.SetIndent("", "  "); return e.Encode(v) }
	if command == "demo" {
		p, err := demodata.Create(dir)
		if err != nil {
			return err
		}
		return printJSON(map[string]any{"config": p, "next_step": "wxmori --config \"" + p + "\" serve", "data": "Synthetic / 合成数据"})
	}
	if command == "init" {
		if dataDir == "" && decryptedDir == "" {
			candidates := config.Discover()
			if len(candidates) != 1 {
				return printJSON(map[string]any{"configured": false, "accounts": candidates, "next_step": "Choose one: wxmori init --data-dir PATH. Or use --decrypted-dir PATH. / 请选择一个账号。"})
			}
			dataDir = candidates[0]
		}
		if dataDir != "" && decryptedDir != "" {
			return fault.New("INVALID_CONFIG", "只能选择一个数据源 / Choose one source", "Use --data-dir OR --decrypted-dir.")
		}
		abs := func(p string) string {
			if p == "" {
				return ""
			}
			q, _ := filepath.Abs(p)
			return q
		}
		if outputDir == "" {
			home, _ := os.UserHomeDir()
			outputDir = filepath.Join(home, "Documents", "WxMori-exports")
		}
		c := config.Config{DataDir: abs(dataDir), DecryptedDir: abs(decryptedDir), KeyFile: abs(keyFile), OutputDir: abs(outputDir), Timezone: timezone}
		if err := c.ValidateSource(); err != nil {
			return err
		}
		if _, err := c.Location(); err != nil {
			return fault.New("INVALID_TIMEZONE", "时区无效 / Invalid timezone", "Use an IANA timezone such as Asia/Shanghai.")
		}
		p := *path
		if p == "" {
			p = config.DefaultPath()
		}
		if err := config.Save(p, c); err != nil {
			return err
		}
		return printJSON(map[string]any{"configured": true, "config": abs(p), "next_step": "Run wxmori status, then wxmori serve. / 检查状态后启动 MCP。"})
	}
	c, loadErr := config.Load(*path)
	app := service.New(c, loadErr)
	defer app.Close()
	if command == "serve" {
		return mcpserver.Serve(ctx, app)
	}
	if command == "status" {
		v, err := app.Status(ctx)
		if err != nil {
			return err
		}
		return printJSON(v)
	}
	if loadErr != nil {
		return loadErr
	}
	switch command {
	case "key-capture":
		v, err := app.Capture(ctx, service.CaptureArgs{Action: "start", TimeoutSeconds: captureTimeout, Mode: captureMode})
		if err != nil {
			return err
		}
		if err = printJSON(v); err != nil {
			return err
		}
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				v, err = app.Capture(context.Background(), service.CaptureArgs{Action: "stop"})
				if err != nil {
					return err
				}
				return printJSON(v)
			case <-ticker.C:
				v, err = app.Capture(ctx, service.CaptureArgs{Action: "status"})
				if err != nil {
					return err
				}
				if err = printJSON(v); err != nil {
					return err
				}
				status := v.(keycapture.Status)
				if !status.Running {
					if status.State != "completed" {
						return fault.New("CAPTURE_INCOMPLETE", "采集未完成 / Capture incomplete", status.Hint)
					}
					return nil
				}
			}
		}
	case "sessions":
		v, err := app.Sessions(ctx, service.ListArgs{Keyword: keyword})
		if err != nil {
			return err
		}
		return printJSON(v)
	case "export":
		var list []string
		if months != "" {
			list = strings.Split(months, ",")
			for i := range list {
				list[i] = strings.TrimSpace(list[i])
			}
		}
		v, err := app.Export(ctx, service.ExportArgs{SessionID: session, Months: list, From: from, To: to, Format: format})
		if err != nil {
			return err
		}
		return printJSON(v)
	case "decrypt":
		if dir == "" {
			return fault.New("OUTPUT_REQUIRED", "需要输出目录 / Output directory required", "Use --out NEW_DIR. This explicitly saves persistent plaintext.")
		}
		dest, err := filepath.Abs(dir)
		if err != nil {
			return err
		}
		if _, err = os.Stat(dest); !os.IsNotExist(err) {
			return fault.New("OUTPUT_EXISTS", "输出目录已存在 / Output directory already exists", "Choose a new destination; existing data is never overwritten.")
		}
		snap, _, wal, err := store.SnapshotWithReport(ctx, c, false)
		if err != nil {
			return err
		}
		defer os.RemoveAll(snap)
		if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
			return err
		}
		if err = os.Rename(snap, dest); err != nil {
			return fault.New("DESTINATION_MOVE_FAILED", "无法发布明文副本 / Could not publish plaintext copy", "Choose an output on the same drive as the system temporary directory. The incomplete copy was removed.")
		}
		return printJSON(map[string]any{"directory": dest, "integrity_check": "ok", "persistent_plaintext": true, "wal": wal})
	}
	return nil
}
