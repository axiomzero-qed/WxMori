package store

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/fault"
)

type Store struct {
	DB          *sql.DB
	Dir         string
	Fingerprint string
	CreatedAt   string
	Warnings    map[string]int
}
type Contact struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}
type Session struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	Type          string `json:"type"`
	MessageCount  int    `json:"message_count"`
	LastMessageAt string `json:"last_message_at,omitempty"`
}
type Message struct {
	SessionID string `json:"session_id"`
	Timestamp int64  `json:"timestamp"`
	Time      string `json:"time"`
	SenderID  string `json:"sender_id,omitempty"`
	Sender    string `json:"sender,omitempty"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	ServerID  string `json:"server_id,omitempty"`
}

func (s *Store) Close() {
	if s.DB != nil {
		_ = s.DB.Close()
	}
	_ = os.RemoveAll(s.Dir)
}
func quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func columns(ctx context.Context, db *sql.DB, table string) (map[string]bool, error) {
	r, err := db.QueryContext(ctx, "PRAGMA table_info("+quote(table)+")")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	m := map[string]bool{}
	for r.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt any
		if err = r.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		m[name] = true
	}
	return m, r.Err()
}
func tableNames(ctx context.Context, db *sql.DB) ([]string, error) {
	r, err := db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var names []string
	for r.Next() {
		var n string
		if err = r.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, r.Err()
}

func Build(ctx context.Context, c config.Config) (s *Store, err error) {
	dir, fp, report, err := SnapshotWithReport(ctx, c, false)
	if err != nil {
		return nil, err
	}
	s = &Store{Dir: dir, Fingerprint: fp, CreatedAt: time.Now().UTC().Format(time.RFC3339), Warnings: map[string]int{}}
	if report.EncryptedWALFiles > 0 {
		s.Warnings["encrypted_wal_files_validated"] = report.EncryptedWALFiles
	}
	if report.CommittedFrames > 0 {
		s.Warnings["wal_committed_frames_applied"] = int(report.CommittedFrames)
	}
	if report.UncommittedFrames > 0 {
		s.Warnings["wal_uncommitted_frames_excluded"] = int(report.UncommittedFrames)
	}
	if report.StaleTails > 0 {
		s.Warnings["wal_previous_generation_tails_excluded"] = report.StaleTails
	}
	created := s
	success := false
	defer func() {
		if !success {
			created.Close()
		}
	}()
	s.DB, err = Open(filepath.Join(dir, "normalized.sqlite"), false)
	if err != nil {
		return nil, err
	}
	_, err = s.DB.ExecContext(ctx, `PRAGMA journal_mode=MEMORY; PRAGMA synchronous=OFF;
CREATE TABLE contacts(id TEXT PRIMARY KEY,name TEXT NOT NULL);
CREATE TABLE sessions(id TEXT PRIMARY KEY,name TEXT NOT NULL,type TEXT NOT NULL);
CREATE TABLE messages(session TEXT NOT NULL,uid TEXT NOT NULL,ts INTEGER NOT NULL,seq INTEGER NOT NULL,sender_id TEXT NOT NULL,sender TEXT NOT NULL,type TEXT NOT NULL,text TEXT NOT NULL,server_id TEXT NOT NULL,PRIMARY KEY(session,uid));
CREATE INDEX message_time ON messages(session,ts,seq,uid);`)
	if err != nil {
		return nil, err
	}
	files, err := Files(dir, false)
	if err != nil {
		return nil, err
	}
	contacts := map[string]string{}
	ids := map[string]string{}
	for _, p := range files {
		if filepath.Base(p) == "contact.db" {
			if err = s.importContacts(ctx, p, contacts); err != nil {
				return nil, err
			}
		}
	}
	for id := range contacts {
		ids[fmt.Sprintf("Msg_%x", md5.Sum([]byte(id)))] = id
	}
	for _, p := range files {
		if messageFile.MatchString(filepath.Base(p)) {
			if err = knownIDs(ctx, p, ids); err != nil {
				return nil, err
			}
		}
	}
	for _, p := range files {
		if messageFile.MatchString(filepath.Base(p)) {
			if err = s.importMessages(ctx, p, ids, contacts); err != nil {
				return nil, err
			}
		}
	}
	success = true
	return s, nil
}

func (s *Store) importContacts(ctx context.Context, path string, names map[string]string) error {
	db, err := Open(path, true)
	if err != nil {
		return err
	}
	defer db.Close()
	cols, err := columns(ctx, db, "contact")
	if err != nil {
		return err
	}
	if !cols["username"] {
		return fault.New("UNSUPPORTED_SCHEMA", "联系人结构不支持 / Unsupported contacts schema", "Expected contact.username. Report schema column names only, without chat content or keys.")
	}
	expr := func(col string) string {
		if cols[col] {
			return "COALESCE(" + quote(col) + ",'')"
		}
		return "''"
	}
	r, err := db.QueryContext(ctx, "SELECT username,"+expr("nick_name")+","+expr("remark")+","+expr("alias")+" FROM contact")
	if err != nil {
		return err
	}
	defer r.Close()
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for r.Next() {
		var id, nick, remark, alias string
		if err = r.Scan(&id, &nick, &remark, &alias); err != nil {
			return err
		}
		name := id
		for _, v := range []string{nick, remark, alias} {
			if strings.TrimSpace(v) != "" {
				name = v
				break
			}
		}
		names[id] = name
		if _, err = tx.ExecContext(ctx, "INSERT OR REPLACE INTO contacts VALUES(?,?)", id, name); err != nil {
			return err
		}
	}
	if err = r.Err(); err != nil {
		return err
	}
	return tx.Commit()
}

func knownIDs(ctx context.Context, path string, ids map[string]string) error {
	db, err := Open(path, true)
	if err != nil {
		return err
	}
	defer db.Close()
	cols, err := columns(ctx, db, "Name2Id")
	if err != nil {
		return err
	}
	if !cols["user_name"] {
		return nil
	}
	r, err := db.QueryContext(ctx, "SELECT user_name FROM Name2Id")
	if err != nil {
		return err
	}
	defer r.Close()
	for r.Next() {
		var id string
		if err = r.Scan(&id); err != nil {
			return err
		}
		ids[fmt.Sprintf("Msg_%x", md5.Sum([]byte(id)))] = id
	}
	return r.Err()
}

var messageTable = regexp.MustCompile(`^Msg_[0-9a-fA-F]{32}$`)

func (s *Store) importMessages(ctx context.Context, path string, ids, contacts map[string]string) error {
	db, err := Open(path, true)
	if err != nil {
		return err
	}
	defer db.Close()
	tables, err := tableNames(ctx, db)
	if err != nil {
		return err
	}
	senders := map[int64]string{}
	ncols, err := columns(ctx, db, "Name2Id")
	if err != nil {
		return err
	}
	if ncols["user_name"] {
		r, e := db.QueryContext(ctx, "SELECT rowid,user_name FROM Name2Id")
		if e != nil {
			return e
		}
		for r.Next() {
			var n int64
			var id string
			if e = r.Scan(&n, &id); e != nil {
				r.Close()
				return e
			}
			senders[n] = id
		}
		e = r.Err()
		r.Close()
		if e != nil {
			return e
		}
	} else {
		s.Warnings["sender_map_missing"]++
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, "INSERT OR IGNORE INTO messages VALUES(?,?,?,?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, table := range tables {
		if !messageTable.MatchString(table) {
			continue
		}
		id := ids[table]
		if id == "" {
			id = table
			s.Warnings["unresolved_session_id"]++
		}
		name := contacts[id]
		if name == "" {
			name = id
		}
		typ := "private"
		if strings.HasSuffix(id, "@chatroom") {
			typ = "group"
		}
		if _, err = tx.ExecContext(ctx, "INSERT OR IGNORE INTO sessions VALUES(?,?,?)", id, name, typ); err != nil {
			return err
		}
		cols, e := columns(ctx, db, table)
		if e != nil {
			return e
		}
		for _, n := range []string{"create_time", "message_content", "local_type", "real_sender_id"} {
			if !cols[n] {
				return fault.New("UNSUPPORTED_SCHEMA", "消息结构不支持 / Unsupported message schema", "Missing required column "+n+". Report column names only.")
			}
		}
		expr := func(n, fallback string) string {
			if cols[n] {
				return "COALESCE(" + quote(n) + "," + fallback + ")"
			}
			return fallback
		}
		query := "SELECT " + strings.Join([]string{expr("server_id", "0"), expr("local_id", "rowid"), expr("create_time", "0"), expr("sort_seq", "0"), expr("local_type", "0"), expr("real_sender_id", "0"), expr("message_content", "''"), expr("WCDB_CT_message_content", "0")}, ",") + " FROM " + quote(table) + " ORDER BY rowid"
		r, e := db.QueryContext(ctx, query)
		if e != nil {
			return e
		}
		occurrences := map[string]int{}
		for r.Next() {
			var server, local, ts, seq, lt, rsid, marker int64
			var raw []byte
			if e = r.Scan(&server, &local, &ts, &seq, &lt, &rsid, &raw, &marker); e != nil {
				r.Close()
				return e
			}
			if ts > 10000000000 {
				ts /= 1000
			}
			if ts <= 0 || ts > 253402300799 {
				r.Close()
				return fault.New("INVALID_TIMESTAMP", "消息时间无效 / Invalid message timestamp", "The database schema may have changed; no message was silently skipped.")
			}
			body, e := DecodeBlob(raw, marker)
			if e != nil {
				r.Close()
				return fault.New("DECODE_FAILED", "消息解码失败 / Message decoding failed", "Unsupported, damaged, or oversized message (maximum decoded size 4 MiB). No partial export was returned.")
			}
			senderID := senders[rsid]
			var mismatch bool
			body, senderID, mismatch = StripPrefix(body, senderID)
			if mismatch {
				s.Warnings["sender_prefix_mismatch"]++
			}
			if senderID == "" {
				senderID = xmlFields(body)["fromusername"]
			}
			sender := contacts[senderID]
			if sender == "" {
				sender = senderID
			}
			if sender == "" {
				if rsid == 0 {
					sender = "我 / Me"
				} else {
					sender = fmt.Sprintf("用户 / User %d", rsid)
					s.Warnings["unresolved_sender"]++
				}
			}
			kind, text := Readable(body, lt)
			if kind == "system" {
				senderID = ""
				sender = ""
			}
			serverID := ""
			uid := ""
			if server > 0 {
				serverID = fmt.Sprint(server)
				uid = "s:" + serverID
			} else {
				hash := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%d\x00%s", ts, senderID, lt, text)))
				h := hex.EncodeToString(hash[:])
				occurrences[h]++
				uid = fmt.Sprintf("f:%s:%d", h, occurrences[h])
			}
			res, e := stmt.ExecContext(ctx, id, uid, ts, seq, senderID, sender, kind, text, serverID)
			if e != nil {
				r.Close()
				return e
			}
			if n, _ := res.RowsAffected(); n == 0 {
				s.Warnings["duplicates_removed"]++
			}
		}
		e = r.Err()
		r.Close()
		if e != nil {
			return e
		}
	}
	return tx.Commit()
}

func (s *Store) Contacts(ctx context.Context, query string) ([]Contact, error) {
	r, err := s.DB.QueryContext(ctx, "SELECT id,name FROM contacts WHERE instr(lower(id),lower(?))>0 OR instr(lower(name),lower(?))>0 ORDER BY name,id", query, query)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []Contact{}
	for r.Next() {
		var c Contact
		if err = r.Scan(&c.ID, &c.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, r.Err()
}

func (s *Store) Sessions(ctx context.Context, keyword string, loc *time.Location) ([]Session, error) {
	r, err := s.DB.QueryContext(ctx, `SELECT s.id,s.name,s.type,count(m.uid),COALESCE(max(m.ts),0) FROM sessions s LEFT JOIN messages m ON m.session=s.id WHERE instr(lower(s.id),lower(?))>0 OR instr(lower(s.name),lower(?))>0 GROUP BY s.id ORDER BY s.name,s.id`, keyword, keyword)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	out := []Session{}
	for r.Next() {
		var item Session
		var ts int64
		if err = r.Scan(&item.ID, &item.DisplayName, &item.Type, &item.MessageCount, &ts); err != nil {
			return nil, err
		}
		if ts > 0 {
			item.LastMessageAt = time.Unix(ts, 0).In(loc).Format(time.RFC3339)
		}
		out = append(out, item)
	}
	return out, r.Err()
}

func (s *Store) Resolve(ctx context.Context, input string, loc *time.Location) (Session, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return Session{}, fault.New("SESSION_REQUIRED", "请选择会话 / A session is required", "Use list_sessions and pass its id as session_id.")
	}
	items, err := s.Sessions(ctx, input, loc)
	if err != nil {
		return Session{}, err
	}
	for _, item := range items {
		if item.ID == input {
			return item, nil
		}
	}
	exact := []Session{}
	for _, item := range items {
		if item.DisplayName == input {
			exact = append(exact, item)
		}
	}
	if len(exact) > 0 {
		items = exact
	}
	if len(items) == 1 {
		return items[0], nil
	}
	if len(items) == 0 {
		return Session{}, fault.New("SESSION_NOT_FOUND", "没有找到会话 / Session not found", "Call list_sessions with part of the name; then use its exact id.")
	}
	if len(items) > 20 {
		items = items[:20]
	}
	return Session{}, &fault.Error{Code: "AMBIGUOUS_SESSION", Message: "有多个匹配会话 / Multiple sessions match", Hint: "Choose one candidate id and call the tool again.", Candidates: items}
}

type Filter struct {
	SessionID string
	From, To  int64
	Query     string
}

func where(f Filter) (string, []any) {
	w := []string{"1=1"}
	args := []any{}
	if f.SessionID != "" {
		w = append(w, "session=?")
		args = append(args, f.SessionID)
	}
	if f.From > 0 {
		w = append(w, "ts>=?")
		args = append(args, f.From)
	}
	if f.To > 0 {
		w = append(w, "ts<?")
		args = append(args, f.To)
	}
	for _, term := range strings.Fields(f.Query) {
		w = append(w, "instr(lower(text),lower(?))>0")
		args = append(args, term)
	}
	return strings.Join(w, " AND "), args
}

func (s *Store) Count(ctx context.Context, f Filter) (int, error) {
	w, a := where(f)
	var n int
	err := s.DB.QueryRowContext(ctx, "SELECT count(*) FROM messages WHERE "+w, a...).Scan(&n)
	return n, err
}
func (s *Store) Visit(ctx context.Context, f Filter, limit, offset int, loc *time.Location, visit func(Message) error) error {
	w, a := where(f)
	q := "SELECT session,ts,sender_id,sender,type,text,server_id FROM messages WHERE " + w + " ORDER BY ts,seq,session,uid"
	if limit >= 0 {
		q += " LIMIT ? OFFSET ?"
		a = append(a, limit, offset)
	}
	r, err := s.DB.QueryContext(ctx, q, a...)
	if err != nil {
		return err
	}
	defer r.Close()
	for r.Next() {
		var m Message
		if err = r.Scan(&m.SessionID, &m.Timestamp, &m.SenderID, &m.Sender, &m.Type, &m.Text, &m.ServerID); err != nil {
			return err
		}
		m.Time = time.Unix(m.Timestamp, 0).In(loc).Format(time.RFC3339)
		if err = visit(m); err != nil {
			return err
		}
	}
	return r.Err()
}

func SortedWarningKeys(w map[string]int) []string {
	keys := make([]string, 0, len(w))
	for k := range w {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
