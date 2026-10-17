package store

import (
	"context"
	"database/sql"

	"github.com/axiomzero-qed/WxMori/internal/config"
)

type Identity struct {
	DirectoryID string `json:"directory_id,omitempty"`
	InternalID  string `json:"internal_id_candidate,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	Alias       string `json:"wechat_alias,omitempty"`
	Verified    bool   `json:"contact_match_verified"`
	Evidence    string `json:"evidence"`
}

func (s *Store) Identity(ctx context.Context, account config.Account) (Identity, error) {
	out := Identity{DirectoryID: account.DirectoryID, InternalID: account.InternalID, Evidence: "directory_only; not a verified self identity / 仅目录线索，尚未核验本人身份"}
	if account.InternalID == "" {
		return out, nil
	}
	var name, nick, alias string
	err := s.DB.QueryRowContext(ctx, "SELECT name,nickname,alias FROM contacts WHERE id=?", account.InternalID).Scan(&name, &nick, &alias)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if nick == "" {
		nick = name
	}
	out.DisplayName, out.Alias, out.Verified, out.Evidence = nick, alias, true, "contact_username_matches_account_directory"
	return out, nil
}

type History struct {
	SessionCount int   `json:"session_count"`
	MessageCount int   `json:"message_count"`
	Earliest     int64 `json:"earliest_timestamp,omitempty"`
	Latest       int64 `json:"latest_timestamp,omitempty"`
	Sparse       bool  `json:"sparse_local_history"`
}

func (s *Store) History(ctx context.Context) (History, error) {
	var out History
	err := s.DB.QueryRowContext(ctx, `SELECT count(*),COALESCE(min(ts),0),COALESCE(max(ts),0) FROM messages`).Scan(&out.MessageCount, &out.Earliest, &out.Latest)
	if err != nil {
		return out, err
	}
	err = s.DB.QueryRowContext(ctx, "SELECT count(*) FROM sessions").Scan(&out.SessionCount)
	out.Sparse = out.SessionCount <= 3 && out.MessageCount <= 10
	return out, err
}
