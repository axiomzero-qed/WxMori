package config

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/keyring"
)

// Account reports local file metadata, never inferred sync or login state.
type Account struct {
	DirectoryID          string `json:"directory_id"`
	InternalID           string `json:"internal_id_candidate,omitempty"`
	DataDir              string `json:"data_dir"`
	LastActiveAt         string `json:"last_file_change_at,omitempty"`
	DatabaseCount        int    `json:"database_count"`
	CoreDatabaseCount    int    `json:"core_database_count"`
	DatabaseBytes        int64  `json:"database_bytes"`
	MessageDatabaseBytes int64  `json:"message_database_bytes"`
	MetadataComplete     bool   `json:"metadata_complete"`
	KeyringAvailable     bool   `json:"protected_keyring_available"`
}

var accountSuffix = regexp.MustCompile(`_[0-9a-fA-F]{4,}$`)
var messageDB = regexp.MustCompile(`^message_[0-9]+\.db$`)

func DescribeAccount(dir, configPath string) Account {
	abs, err := filepath.Abs(dir)
	if err == nil {
		dir = filepath.Clean(abs)
	}
	a := Account{DataDir: dir, MetadataComplete: true}
	if filepath.Base(dir) == "db_storage" {
		a.DirectoryID = filepath.Base(filepath.Dir(dir))
		a.InternalID = accountSuffix.ReplaceAllString(a.DirectoryID, "")
	}
	var latest time.Time
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			a.MetadataComplete = false
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			a.MetadataComplete = false
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		base := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, "-wal"), "-journal"), "-shm")
		if !strings.HasSuffix(base, ".db") {
			return nil
		}
		s, e := d.Info()
		if e != nil {
			a.MetadataComplete = false
			return nil
		}
		if s.ModTime().After(latest) {
			latest = s.ModTime()
		}
		if name == base {
			a.DatabaseCount++
			a.DatabaseBytes += s.Size()
			if base == "contact.db" || messageDB.MatchString(base) {
				a.CoreDatabaseCount++
			}
			if messageDB.MatchString(base) {
				a.MessageDatabaseBytes += s.Size()
			}
		}
		return nil
	})
	if err != nil {
		a.MetadataComplete = false
	}
	if !latest.IsZero() {
		a.LastActiveAt = latest.UTC().Format(time.RFC3339Nano)
	}
	c := Config{DataDir: dir, Path: configPath}
	if s, e := os.Stat(c.CapturedKeyFile()); e == nil && s.Mode().IsRegular() {
		a.KeyringAvailable = true
	}
	return a
}

// Accounts includes discovered accounts plus the selected source's siblings.
// An explicit root supports custom WeChat storage without scanning whole drives.
func Accounts(selected, root, configPath string) []Account {
	paths := Discover()
	roots := []string{}
	if root != "" {
		paths = nil
		roots = append(roots, root)
	}
	if selected != "" {
		paths = append(paths, selected)
		if filepath.Base(filepath.Clean(selected)) == "db_storage" {
			roots = append(roots, filepath.Dir(filepath.Dir(selected)))
		}
	}
	for _, parent := range roots {
		entries, _ := os.ReadDir(parent)
		for _, e := range entries {
			if e.IsDir() && e.Type()&os.ModeSymlink == 0 {
				p := filepath.Join(parent, e.Name(), "db_storage")
				if s, err := os.Stat(p); err == nil && s.IsDir() {
					paths = append(paths, p)
				}
			}
		}
	}
	out := []Account{}
	seen := map[string]bool{}
	for _, p := range paths {
		id := keyring.AccountID(p)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, DescribeAccount(p, configPath))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].LastActiveAt != out[j].LastActiveAt {
			a, _ := time.Parse(time.RFC3339Nano, out[i].LastActiveAt)
			b, _ := time.Parse(time.RFC3339Nano, out[j].LastActiveAt)
			return a.After(b)
		}
		return out[i].DataDir < out[j].DataDir
	})
	return out
}
