package keycapture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
)

type Status struct {
	Supported         bool           `json:"supported"`
	Running           bool           `json:"running"`
	ReadyForLogin     bool           `json:"ready_for_login"`
	State             string         `json:"state"`
	Method            string         `json:"method"`
	RequiredDatabases int            `json:"required_databases"`
	VerifiedDatabases int            `json:"verified_databases"`
	ActiveProcesses   int            `json:"active_weixin_processes"`
	AccessDenied      int            `json:"access_denied_processes"`
	Passes            int            `json:"passes"`
	Candidates        int            `json:"candidate_count"`
	KeySetupEvents    int            `json:"key_setup_events"`
	ObservedAccounts  []AccountMatch `json:"observed_account_matches,omitempty"`
	BytesRead         uint64         `json:"bytes_read"`
	StartedAt         string         `json:"started_at,omitempty"`
	FinishedAt        string         `json:"finished_at,omitempty"`
	KeyFile           string         `json:"key_file,omitempty"`
	Hint              string         `json:"hint"`
	Error             *fault.Error   `json:"error,omitempty"`
}
type Manager struct {
	mu        sync.Mutex
	c         config.Config
	status    Status
	cancel    context.CancelFunc
	done      chan struct{}
	enumerate func() ([]*process, int, error)
	mode      string
}

func New(c config.Config) *Manager {
	return &Manager{c: c, enumerate: listProcesses, status: Status{Supported: platformSupported(), State: "idle", Method: "windows_readonly_memory", Hint: "Start capture, then open WeChat and log in yourself. / 开始采集后打开微信并亲自登录。"}}
}
func (m *Manager) Status() Status { m.mu.Lock(); defer m.mu.Unlock(); return m.status }
func (m *Manager) Start(timeout time.Duration) (Status, error) {
	return m.StartWithMode(timeout, "auto")
}
func (m *Manager) StartWithMode(timeout time.Duration, mode string) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !platformSupported() {
		return m.status, fault.New("CAPTURE_UNSUPPORTED", "本平台不支持登录采集 / Login capture is unsupported on this platform", "Use 64-bit Windows with the desktop Weixin.exe client. Existing decrypted databases and manual master keys remain supported.")
	}
	if m.c.DataDir == "" {
		return m.status, fault.New("CAPTURE_SOURCE_REQUIRED", "请先选择加密数据库目录 / Select an encrypted database source", "Run wxmori init --data-dir PATH for the account to capture. decrypted_dir is for existing plaintext only.")
	}
	if err := m.c.ValidateSource(); err != nil {
		return m.status, err
	}
	if m.status.Running {
		return m.status, nil
	}
	if timeout < time.Second || timeout > 10*time.Minute {
		return m.status, fault.New("INVALID_CAPTURE_TIMEOUT", "采集时限无效 / Invalid capture timeout", "Use 1 to 600 seconds.")
	}
	if mode != "auto" && mode != "read_only" {
		return m.status, fault.New("INVALID_CAPTURE_MODE", "采集模式无效 / Invalid capture mode", "Use auto (default) or read_only. Auto may temporarily attach the Windows debugger at a recognized key-setup routine.")
	}
	m.mode = mode
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	m.cancel = cancel
	m.done = make(chan struct{})
	m.status = Status{Supported: true, Running: true, State: "waiting_for_wechat", Method: "windows_readonly_memory", StartedAt: time.Now().UTC().Format(time.RFC3339), Hint: "Open the WeChat client if necessary; wait for ready_for_login=true before asking the user to re-login locally. Keep WxMori running. / 请先打开微信客户端，等待 ready_for_login=true 后再请用户亲自重新登录，保持 WxMori 运行。"}
	go m.run(ctx, m.done)
	return m.status, nil
}
func (m *Manager) Stop() Status {
	m.mu.Lock()
	cancel, done := m.cancel, m.done
	running := m.status.Running
	m.mu.Unlock()
	if running && cancel != nil {
		cancel()
		if done != nil {
			<-done
		}
	}
	return m.Status()
}
func (m *Manager) Close() { m.Stop() }

var requiredName = regexp.MustCompile(`^message_[0-9]+\.db$`)
var errLoginRestart = errors.New("WeChat process restarted")

func targets(root string) ([]Target, string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fault.New("SYMLINK_SOURCE", "数据目录含符号链接 / Source contains symlinks", "Use a regular account db_storage directory.")
		}
		if !d.IsDir() && (d.Name() == "contact.db" || requiredName.MatchString(d.Name())) {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	sort.Strings(paths)
	if len(paths) > 128 {
		return nil, "", fault.New("TOO_MANY_DATABASES", "数据库数量异常 / Unexpected database count", "Select exactly one account's db_storage directory.")
	}
	h := sha256.New()
	out := []Target{}
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			return nil, "", fault.New("CAPTURE_SOURCE_UNREADABLE", "无法读取所选数据库 / Could not read a selected database", "Check access to the selected account's db_storage directory.")
		}
		page := make([]byte, cryptodb.PageSize)
		_, e = io.ReadFull(f, page)
		f.Close()
		if e != nil {
			clear(page)
			return nil, "", fault.New("CAPTURE_DATABASE_NOT_READY", "数据库首页尚未写完 / Database first page is not ready", "Keep capture running while WeChat finishes creating this account's databases.")
		}
		if bytes.Equal(page[:16], cryptodb.SQLiteHeader) {
			return nil, "", fault.New("CAPTURE_PLAINTEXT_SOURCE", "选择的是明文数据库 / Source is already plaintext", "Use decrypted_dir to read it; capture requires original encrypted db_storage files.")
		}
		var salt [16]byte
		copy(salt[:], page[:16])
		name, _ := filepath.Rel(root, path)
		out = append(out, Target{Salt: salt, Page: page, Name: name})
		fmt.Fprintf(h, "%s:%x\n", name, salt)
	}
	return out, hex.EncodeToString(h.Sum(nil)), nil
}
func validKeys(targets []Target, keys map[[16]byte][32]byte) int {
	n := 0
	for _, t := range targets {
		key, ok := keys[t.Salt]
		if !ok {
			continue
		}
		mac := cryptodb.MACKey(key[:], t.Salt[:])
		valid := cryptodb.Verify(t.Page, mac, 1)
		clear(mac)
		if valid {
			n++
		} else {
			delete(keys, t.Salt)
		}
	}
	return n
}

func (m *Manager) publish(ts []Target, keys map[[16]byte][32]byte, savedHash *[32]byte) error {
	verified := validKeys(ts, keys)
	m.mu.Lock()
	m.status.RequiredDatabases = len(ts)
	m.status.VerifiedDatabases = verified
	m.mu.Unlock()
	if verified == 0 {
		return nil
	}
	h := sha256.New()
	current := make(map[[16]byte][32]byte, verified)
	defer clear(current)
	for _, t := range ts {
		if key, ok := keys[t.Salt]; ok {
			h.Write(t.Salt[:])
			h.Write(key[:])
			current[t.Salt] = key
		}
	}
	var hash [32]byte
	copy(hash[:], h.Sum(nil))
	if hash == *savedHash {
		return nil
	}
	if err := keyring.SaveCaptured(m.c.CapturedKeyFile(), keyring.AccountID(m.c.DataDir), current); err != nil {
		return fault.New("KEY_SAVE_FAILED", "无法保存已校验密钥 / Could not save verified keys", "Check permissions for the configuration directory and Windows DPAPI availability.")
	}
	*savedHash = hash
	m.mu.Lock()
	m.status.KeyFile = m.c.CapturedKeyFile()
	m.mu.Unlock()
	return nil
}

func (m *Manager) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	keys := map[[16]byte][32]byte{}
	defer clear(keys)
	probes := map[uint32]*Probe{}
	defer func() {
		for _, p := range probes {
			p.Clear()
		}
	}()
	defer func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.status.Running = false
		m.status.ReadyForLogin = false
		m.status.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		if m.status.State == "completed" || m.status.State == "failed" || m.status.State == "account_mismatch" {
			return
		}
		if ctx.Err() == context.DeadlineExceeded {
			if m.status.VerifiedDatabases > 0 {
				m.status.State = "partial"
			} else {
				m.status.State = "timeout"
			}
			m.status.Hint = "No complete key set was captured. Start capture before re-login; source/version/timing or read permissions may be unsupported. / 未取得完整密钥，请在重新登录前开始采集，并检查版本、时机和读取权限。"
		} else {
			m.status.State = "stopped"
			m.status.Hint = "Capture stopped; only verified keys, if any, were saved locally. / 采集已停止，仅已校验的密钥可能保存在本机。"
		}
	}()
	lastHash := ""
	stableSince := time.Now()
	var savedHash [32]byte
	for {
		if ctx.Err() != nil {
			return
		}
		ts, hash, err := targets(m.c.DataDir)
		if err != nil {
			if f, ok := err.(*fault.Error); ok && f.Code == "CAPTURE_DATABASE_NOT_READY" {
				m.setState("waiting_for_databases", f.Hint)
				select {
				case <-ctx.Done():
					return
				case <-time.After(500 * time.Millisecond):
					continue
				}
			}
			m.fail(err)
			return
		}
		if hash != lastHash {
			for _, p := range probes {
				p.Clear()
			}
			clear(probes)
			lastHash = hash
			stableSince = time.Now()
		}
		if err = m.publish(ts, keys, &savedHash); err != nil {
			m.fail(err)
			return
		}
		verified := validKeys(ts, keys)
		hasContact, hasMessage := false, false
		for _, t := range ts {
			if filepath.Base(t.Name) == "contact.db" {
				hasContact = true
			} else {
				hasMessage = true
			}
		}
		if len(ts) > 0 && verified == len(ts) && hasContact && hasMessage {
			if time.Since(stableSince) >= 5*time.Second {
				m.mu.Lock()
				m.status.State = "completed"
				m.status.Hint = "All currently required database keys passed HMAC and were saved with Windows user protection. Close WeChat normally before reading if encrypted WAL exists. / 当前所需数据库密钥均已校验并由 Windows 用户保护保存；若有加密 WAL，请正常关闭微信后读取。"
				m.mu.Unlock()
				return
			}
			m.setState("settling", "Keys verified; waiting for the account's database list to settle. / 密钥已校验，等待账号数据库列表稳定。")
		} else if len(ts) == 0 {
			m.setState("waiting_for_databases", "Log in to the account whose db_storage is configured. / 请登录配置中 db_storage 所属账号。")
		} else {
			processes, denied, e := m.enumerate()
			if e != nil {
				m.fail(fault.New("PROCESS_ENUMERATION_FAILED", "无法枚举微信进程 / Could not enumerate WeChat processes", "Retry from a normal local Windows session."))
				return
			}
			m.mu.Lock()
			m.status.ActiveProcesses = len(processes)
			m.status.AccessDenied = denied
			m.status.Passes++
			m.mu.Unlock()
			if len(processes) == 0 {
				if denied > 0 {
					m.setState("access_denied", "Windows denied read access. Run WxMori at the same integrity level as WeChat; do not disable antivirus. / Windows 拒绝读取，请以与微信匹配的权限运行，不要关闭杀毒软件。 ")
				} else {
					m.setState("waiting_for_wechat", "Open the desktop WeChat client and log in yourself. / 请打开桌面微信并亲自登录。")
				}
			}
			if m.mode != "read_only" {
				handled, watchErr := m.watchLogin(ctx, processes, keys)
				if handled {
					for _, p := range processes {
						p.close()
					}
					if errors.Is(watchErr, errLoginRestart) {
						m.mu.Lock()
						m.status.ReadyForLogin = false
						m.mu.Unlock()
						m.setState("waiting_for_wechat", "WeChat restarted; waiting to re-arm capture on its new process. / 微信进程已重启，正在等待为新进程准备捕获。")
						select {
						case <-ctx.Done():
							return
						case <-time.After(500 * time.Millisecond):
							continue
						}
					}
					if watchErr != nil && ctx.Err() == nil {
						m.fail(watchErr)
					}
					return
				}
			}
			m.mu.Lock()
			m.status.ReadyForLogin = len(processes) > 0
			m.status.Method = "windows_readonly_memory"
			m.mu.Unlock()
			for i, p := range processes {
				if ctx.Err() != nil {
					for _, rest := range processes[i:] {
						rest.close()
					}
					return
				}
				probe := probes[p.pid]
				if probe == nil {
					probe = NewProbe(ts)
					probes[p.pid] = probe
				}
				probe.Targets = ts
				probe.BeginPass()
				for salt, key := range keys {
					probe.Keys[salt] = key
				}
				m.setState("scanning", "Reading only Weixin.exe memory; waiting for database-authenticated keys. / 仅读取 Weixin.exe 内存，等待通过数据库校验的密钥。")
				reportedCandidates := probe.Candidates
				err = p.scan(ctx, probe, func(n uint64) {
					m.mu.Lock()
					m.status.BytesRead += n
					m.status.VerifiedDatabases = probe.Verified()
					m.status.Candidates += probe.Candidates - reportedCandidates
					reportedCandidates = probe.Candidates
					m.mu.Unlock()
				})
				p.close()
				for salt, key := range probe.Keys {
					keys[salt] = key
				}
				// Save successful reads even when the scan was cancelled or timed out.
				if saveErr := m.publish(ts, keys, &savedHash); saveErr != nil {
					for _, rest := range processes[i+1:] {
						rest.close()
					}
					m.fail(saveErr)
					return
				}
				if err != nil && ctx.Err() != nil {
					for _, rest := range processes[i+1:] {
						rest.close()
					}
					return
				}
				if validKeys(ts, keys) == len(ts) {
					for _, rest := range processes[i+1:] {
						rest.close()
					}
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
func (m *Manager) setState(state, hint string) {
	m.mu.Lock()
	m.status.State = state
	m.status.Hint = hint
	m.mu.Unlock()
}
func (m *Manager) fail(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status.State = "failed"
	if f, ok := err.(*fault.Error); ok {
		m.status.Error = f
		if f.Code == "KEY_ACCOUNT_MISMATCH" {
			m.status.State = "account_mismatch"
		}
	} else {
		m.status.Error = fault.New("CAPTURE_FAILED", "采集失败 / Capture failed", "Check the selected account path and local permissions.")
	}
	m.status.Hint = m.status.Error.Hint
	m.status.ReadyForLogin = false
}
