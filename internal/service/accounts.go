package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
	"github.com/axiomzero-qed/WxMori/internal/store"
)

type AccountArgs struct {
	Limit  int `json:"limit,omitempty" jsonschema:"Default 50, maximum 200 / 默认 50，最多 200"`
	Offset int `json:"offset,omitempty"`
}

func (a *App) Accounts(ctx context.Context, arg AccountArgs) (any, error) {
	limit, err := page(arg.Limit, arg.Offset, 50, 200)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	items := config.Accounts(a.Config.DataDir, "", a.Config.Path)
	i, j := bounds(len(items), limit, arg.Offset)
	out := map[string]any{"accounts": items[i:j], "identity_policy": "Directory ids may differ from custom WeChat ids. File changes do not prove login or sync. / 目录标识可能不同于自定义微信号；文件变化不能证明登录或同步状态。", "next_step": "Choose an explicit data_dir with CLI init; no account is switched automatically. / 用 CLI init 明确选择 data_dir，不会自动切换账号。"}
	pagination(out, len(items), j-i, arg.Offset)
	return out, nil
}

func (a *App) KeyList(ctx context.Context) (any, error) {
	base := a.Config.Path
	if base == "" {
		base = config.DefaultPath()
	}
	dir := filepath.Join(filepath.Dir(base), ".wxmori-keys")
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string]any{"keyrings": []any{}}, nil
	}
	if err != nil {
		return nil, err
	}
	labels := map[string]config.Account{}
	for _, item := range config.Accounts(a.Config.DataDir, "", base) {
		labels[keyring.AccountID(item.DataDir)] = item
	}
	items := []map[string]any{}
	for _, entry := range entries {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, e := keyring.InspectCaptured(path)
		item := map[string]any{"file": path, "protected": true}
		if e != nil {
			item["error"] = AsFault(e)
		} else {
			item["captured_at"], item["database_key_count"], item["account_binding"] = info.CapturedAt, info.DatabaseKeyCount, info.AccountHash
			if account, ok := labels[info.AccountHash]; ok {
				item["account"] = account
			}
		}
		items = append(items, item)
	}
	return map[string]any{"keyrings": items, "key_values_included": false}, nil
}

func directoryIdentity(account config.Account) store.Identity {
	return store.Identity{DirectoryID: account.DirectoryID, InternalID: account.InternalID, Evidence: "directory_only; use status inspect or whoami for contact verification / 仅目录线索，使用 status --inspect 或 whoami 核验联系人"}
}

func dataDiagnostics(s *store.Store) map[string]any {
	identity, history := s.IdentitySummary, s.HistorySummary
	items := []map[string]string{}
	if history.Sparse {
		items = append(items, map[string]string{"code": "SPARSE_LOCAL_HISTORY", "message": "本机历史较少，可能尚未迁移或同步 / Little local history is present; it may not have been migrated or synced", "hint": "Only local records can be exported. Check the account and WeChat's own history migration options. Sparse counts alone do not prove missing history. / 只能导出本机记录；请核对账号及微信自身的聊天迁移选项。数量少不能证明有历史缺失。"})
	}
	return map[string]any{"whoami": identity, "history": history, "diagnostics": items}
}
