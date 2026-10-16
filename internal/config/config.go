package config

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/axiomzero-qed/WxMori/internal/fault"
)

type Config struct {
	DataDir      string `json:"data_dir,omitempty"`
	DecryptedDir string `json:"decrypted_dir,omitempty"`
	KeyFile      string `json:"key_file,omitempty"`
	OutputDir    string `json:"output_dir"`
	Timezone     string `json:"timezone"`
	Path         string `json:"-"`
}

func DefaultPath() string {
	if p := os.Getenv("WXMORI_CONFIG"); p != "" {
		return p
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d = "."
	}
	return filepath.Join(d, "wxmori", "config.json")
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}
	c := Config{Path: abs, Timezone: "Asia/Shanghai"}
	f, err := os.Open(abs)
	if os.IsNotExist(err) {
		return c, fault.New("CONFIG_REQUIRED", "尚未配置 WxMori / WxMori is not configured", "Run wxmori init, or wxmori demo --dir ./demo-data, then pass --config PATH.")
	}
	if err != nil {
		return c, err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 1<<20))
	d.DisallowUnknownFields()
	if err = d.Decode(&c); err != nil {
		return c, fault.New("INVALID_CONFIG", "配置文件格式无效 / Invalid configuration", "Use the fields from config.example.json; never put a key in tool arguments.")
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return c, fault.New("INVALID_CONFIG", "配置文件必须只有一个 JSON 对象 / Expected one JSON object", "Remove trailing data.")
	}
	if (c.DataDir == "") == (c.DecryptedDir == "") {
		return c, fault.New("INVALID_CONFIG", "请选择一个数据源 / Choose exactly one source", "Set either data_dir or decrypted_dir, not both.")
	}
	base := filepath.Dir(abs)
	resolve := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(base, p)
	}
	c.DataDir, c.DecryptedDir, c.KeyFile = resolve(c.DataDir), resolve(c.DecryptedDir), resolve(c.KeyFile)
	if c.OutputDir == "" {
		c.OutputDir = "exports"
	}
	c.OutputDir = resolve(c.OutputDir)
	if c.Timezone == "" {
		c.Timezone = "Asia/Shanghai"
	}
	if _, err = c.Location(); err != nil {
		return c, fault.New("INVALID_TIMEZONE", "时区无效 / Invalid timezone", "Use an IANA timezone such as Asia/Shanghai or UTC.")
	}
	return c, nil
}

func (c Config) Source() string {
	if c.DecryptedDir != "" {
		return c.DecryptedDir
	}
	return c.DataDir
}
func (c Config) Location() (*time.Location, error) { return time.LoadLocation(c.Timezone) }

func (c Config) MasterKey() ([]byte, error) {
	s := strings.TrimSpace(os.Getenv("WXMORI_MASTER_KEY"))
	if s == "" && c.KeyFile != "" {
		f, err := os.Open(c.KeyFile)
		if err != nil {
			return nil, fault.New("KEY_FILE_UNREADABLE", "无法读取密钥文件 / Cannot read key file", "Check key_file locally; do not paste keys into chat.")
		}
		defer f.Close()
		var k struct {
			MasterKey string `json:"master_key"`
		}
		if err = json.NewDecoder(io.LimitReader(f, 65536)).Decode(&k); err != nil {
			return nil, fault.New("INVALID_KEY_FILE", "密钥文件格式无效 / Invalid key file", "Expected a local JSON object with a master_key string.")
		}
		s = strings.TrimSpace(k.MasterKey)
	}
	if s == "" {
		return nil, fault.New("KEY_REQUIRED", "需要本地数据库主密钥 / A local database master key is required", "Set WXMORI_MASTER_KEY or key_file locally. Automatic key acquisition is not included in this release.")
	}
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != 32 {
		return nil, fault.New("INVALID_KEY", "主密钥格式无效 / Invalid master key format", "Expected exactly 64 hexadecimal characters. Do not send the value to the model.")
	}
	return b, nil
}

func Save(path string, c Config) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(abs), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(append(b, '\n'))
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func Discover() []string {
	h, _ := os.UserHomeDir()
	roots := []string{filepath.Join(h, "Documents", "xwechat_files"), filepath.Join(h, "OneDrive", "Documents", "xwechat_files")}
	var found []string
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if e.IsDir() {
				p := filepath.Join(root, e.Name(), "db_storage")
				if s, err := os.Stat(p); err == nil && s.IsDir() {
					found = append(found, p)
				}
			}
		}
	}
	return found
}

func (c Config) ValidateSource() error {
	s, err := os.Stat(c.Source())
	if err != nil || !s.IsDir() {
		return fault.New("SOURCE_NOT_FOUND", "未找到数据目录 / Source directory not found", "Set data_dir to one account's db_storage, or decrypted_dir to a SQLite export tree.")
	}
	return nil
}

func WrapIO(err error) error {
	return fault.New("IO_ERROR", "文件操作失败 / File operation failed", fmt.Sprintf("Check local paths and permissions (%T).", err))
}
