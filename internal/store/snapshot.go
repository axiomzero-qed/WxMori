package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
	_ "modernc.org/sqlite"
)

var messageFile = regexp.MustCompile(`^message_[0-9]+\.db$`)

func Files(root string, all bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fault.New("SYMLINK_SOURCE", "数据源包含符号链接 / Source contains a symlink", "Use a source directory containing regular files.")
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".db") && (all || messageFile.MatchString(d.Name()) || d.Name() == "contact.db") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	if err == nil && len(files) == 0 {
		err = fault.New("NO_DATABASES", "没有找到数据库 / No databases found", "Expected contact/contact.db and message/message_N.db under the configured source.")
	}
	return files, err
}

func Fingerprint(root string, all bool) (string, []string, error) {
	files, err := Files(root, all)
	if err != nil {
		return "", nil, err
	}
	h := sha256.New()
	for _, p := range files {
		for _, suffix := range []string{"", "-wal", "-journal"} {
			s, e := os.Stat(p + suffix)
			if os.IsNotExist(e) {
				fmt.Fprintf(h, "%s:absent\n", p+suffix)
				continue
			}
			if e != nil {
				return "", nil, e
			}
			fmt.Fprintf(h, "%s:%d:%d\n", p+suffix, s.Size(), s.ModTime().UnixNano())
		}
	}
	return hex.EncodeToString(h.Sum(nil)), files, nil
}

func Open(path string, ro bool) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(abs)
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := u.Query()
	if ro {
		q.Set("mode", "ro")
	}
	q.Add("_pragma", "busy_timeout(5000)")
	u.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func Integrity(ctx context.Context, path string) error {
	db, err := Open(path, true)
	if err != nil {
		return err
	}
	defer db.Close()
	r, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return err
	}
	defer r.Close()
	count := 0
	for r.Next() {
		var result string
		if err = r.Scan(&result); err != nil {
			return err
		}
		count++
		if result != "ok" {
			return fault.New("INTEGRITY_FAILED", "SQLite 完整性检查失败 / SQLite integrity check failed", "Use a consistent database copy and matching key; no partial result will be returned.")
		}
	}
	if err = r.Err(); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("unexpected integrity result")
	}
	return nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if e := out.Close(); err == nil {
			err = e
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

// Snapshot copies first. It never opens the original databases through SQLite.
func Snapshot(ctx context.Context, c config.Config, all bool) (dir, fingerprint string, err error) {
	return snapshot(ctx, c, all, &SnapshotReport{})
}

type SnapshotReport struct {
	EncryptedWALFiles int    `json:"encrypted_wal_files"`
	CommittedFrames   uint64 `json:"committed_frames"`
	UncommittedFrames uint64 `json:"uncommitted_frames_excluded"`
	StaleTails        int    `json:"stale_tails_excluded"`
}

func SnapshotWithReport(ctx context.Context, c config.Config, all bool) (string, string, SnapshotReport, error) {
	var report SnapshotReport
	dir, fp, err := snapshot(ctx, c, all, &report)
	return dir, fp, report, err
}

func snapshot(ctx context.Context, c config.Config, all bool, report *SnapshotReport) (dir, fingerprint string, err error) {
	before, files, err := Fingerprint(c.Source(), all)
	if err != nil {
		return "", "", err
	}
	dir, err = os.MkdirTemp("", "wxmori-")
	if err != nil {
		return "", "", err
	}
	success := false
	cleanupDir := dir
	defer func() {
		if !success {
			_ = os.RemoveAll(cleanupDir)
		}
	}()
	var material *keyring.Material
	if c.DataDir != "" {
		material, err = c.Keys()
		if err != nil {
			return "", "", err
		}
		defer material.Clear()
	}
	for _, src := range files {
		if err = ctx.Err(); err != nil {
			return "", "", err
		}
		rel, _ := filepath.Rel(c.Source(), src)
		if st, e := os.Stat(src + "-journal"); e == nil && st.Size() > 0 {
			return "", "", fault.New("ROLLBACK_JOURNAL_PRESENT", "检测到回滚日志 / Rollback journal detected", "Wait for the writer to finish or close WeChat normally, then retry. Source journals are never deleted or ignored.")
		} else if e != nil && !os.IsNotExist(e) {
			return "", "", e
		}
		dst := filepath.Join(dir, rel)
		if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return "", "", err
		}
		if c.DataDir != "" {
			hasWAL := false
			if s, e := os.Stat(src + "-wal"); e == nil && s.Size() > 0 {
				hasWAL = true
			} else if e != nil && !os.IsNotExist(e) {
				return "", "", e
			}
			// Keep the raw copy private and separate from the published plaintext.
			raw := dst + ".encrypted"
			if err = copyFile(src, raw); err != nil {
				return "", "", err
			}
			rawWAL := raw + "-wal"
			if hasWAL {
				if err = copyFile(src+"-wal", rawWAL); err != nil {
					return "", "", err
				}
			}
			var page [cryptodb.PageSize]byte
			in, e := os.Open(raw)
			if e != nil {
				return "", "", e
			}
			_, e = io.ReadFull(in, page[:])
			in.Close()
			if e != nil {
				return "", "", e
			}
			enc, e := material.Resolve(page[:])
			if e != nil {
				return "", "", e
			}
			err = cryptodb.DecryptFileWithKey(ctx, raw, dst, enc)
			if err == nil && hasWAL {
				var wal cryptodb.WALReport
				wal, err = cryptodb.ReplayWAL(ctx, rawWAL, dst, enc, page[:16])
				if err == nil {
					report.EncryptedWALFiles++
					report.CommittedFrames += wal.CommittedFrames
					report.UncommittedFrames += wal.UncommittedFrames
					if wal.StaleTailBytes > 0 {
						report.StaleTails++
					}
				}
			}
			clear(enc)
			_ = os.Remove(raw)
			if hasWAL {
				_ = os.Remove(rawWAL)
			}
			if err != nil {
				return "", "", err
			}
		} else {
			if err = copyFile(src, dst); err != nil {
				return "", "", err
			}
			if s, e := os.Stat(src + "-wal"); e == nil && s.Size() > 0 {
				if err = copyFile(src+"-wal", dst+"-wal"); err != nil {
					return "", "", err
				}
				var db *sql.DB
				db, err = Open(dst, false)
				if err != nil {
					return "", "", err
				}
				var busy, log, done int
				err = db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &log, &done)
				db.Close()
				if err != nil {
					return "", "", err
				}
				if busy != 0 {
					return "", "", fault.New("SNAPSHOT_BUSY", "快照仍被占用 / Snapshot is busy", "Retry after database activity stops.")
				}
			} else if e != nil && !os.IsNotExist(e) {
				return "", "", e
			}
		}
		if err = Integrity(ctx, dst); err != nil {
			return "", "", err
		}
	}
	after, _, err := Fingerprint(c.Source(), all)
	if err != nil {
		return "", "", err
	}
	if before != after {
		return "", "", fault.New("SOURCE_CHANGED", "复制期间数据发生变化 / Source changed during snapshot", "Pause WeChat activity or close it normally, then retry. The incomplete snapshot was discarded.")
	}
	success = true
	return dir, before, nil
}
