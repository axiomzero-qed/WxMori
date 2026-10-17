//go:build windows && amd64

package keycapture

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/windebug"
	"golang.org/x/sys/windows"
)

type entryCache struct {
	size     int64
	modified int64
	entries  []keyEntry
	err      error
}

var entriesMu sync.Mutex
var entriesCache = map[string]entryCache{}

func cachedEntries(path string) ([]keyEntry, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	entriesMu.Lock()
	defer entriesMu.Unlock()
	old, ok := entriesCache[path]
	if ok && old.size == st.Size() && old.modified == st.ModTime().UnixNano() {
		return old.entries, old.err
	}
	entries, err := findKeyEntries(path)
	if len(entriesCache) >= 8 {
		clear(entriesCache)
	}
	entriesCache[path] = entryCache{st.Size(), st.ModTime().UnixNano(), entries, err}
	return entries, err
}

func loginAddresses(p *process) ([]uintptr, error) {
	s, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPMODULE|windows.TH32CS_SNAPMODULE32, p.pid)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(s)
	var module windows.ModuleEntry32
	module.Size = uint32(unsafe.Sizeof(module))
	for err = windows.Module32First(s, &module); err == nil; err = windows.Module32Next(s, &module) {
		if !strings.EqualFold(windows.UTF16ToString(module.Module[:]), "Weixin.dll") {
			continue
		}
		entries, err := cachedEntries(windows.UTF16ToString(module.ExePath[:]))
		if err != nil {
			return nil, err
		}
		var addresses []uintptr
		for _, entry := range entries {
			if uint64(entry.RVA)+32 > uint64(module.ModBaseSize) {
				return nil, errors.New("key routine outside module")
			}
			address := module.ModBaseAddr + uintptr(entry.RVA)
			var live [32]byte
			if n, e := p.Read(address, live[:]); e != nil || n != 32 || !bytes.Equal(live[:], entry.Prefix[:]) {
				return nil, errors.New("mapped module does not match its file")
			}
			addresses = append(addresses, address)
		}
		return addresses, nil
	}
	return nil, errors.New("native Weixin module not loaded")
}

var captureComplete = errors.New("required keys verified")

func (m *Manager) watchLogin(ctx context.Context, processes []*process, keys map[[16]byte][32]byte) (bool, error) {
	for _, p := range processes {
		addresses, err := loginAddresses(p)
		if err != nil || len(addresses) == 0 {
			continue
		}
		m.mu.Lock()
		m.status.Method = "windows_login_breakpoint"
		m.status.ReadyForLogin = false
		m.status.State = "arming_login_capture"
		m.mu.Unlock()
		probe := NewProbe(nil)
		defer probe.Clear()
		// A bounded cache lets early login events be checked once their database
		// first pages exist. It is never written or returned to the model.
		candidates := map[[32]byte]bool{}
		defer clear(candidates)
		var saved [32]byte
		lastTargets := ""
		stableSince := time.Now()
		lastRefresh := time.Time{}
		var operationErr error
		refresh := func() error {
			if operationErr != nil {
				return operationErr
			}
			if time.Since(lastRefresh) < 500*time.Millisecond {
				return nil
			}
			lastRefresh = time.Now()
			ts, hash, err := targets(m.c.DataDir)
			if f, ok := err.(*fault.Error); ok && f.Code == "CAPTURE_DATABASE_NOT_READY" {
				return nil
			}
			if err != nil {
				return err
			}
			for i := range probe.Targets {
				clear(probe.Targets[i].Page)
			}
			probe.Targets = ts
			if hash != lastTargets {
				lastTargets = hash
				stableSince = time.Now()
				clear(probe.Keys)
			}
			clear(probe.seen)
			for key := range candidates {
				probe.Verify(key, "login_key_setup", nil)
			}
			for salt, key := range probe.Keys {
				keys[salt] = key
			}
			if err := m.publish(ts, keys, &saved); err != nil {
				return err
			}
			m.mu.Lock()
			m.status.Candidates = probe.Candidates
			m.mu.Unlock()
			hasContact, hasMessage := false, false
			for _, t := range ts {
				if filepath.Base(t.Name) == "contact.db" {
					hasContact = true
				} else {
					hasMessage = true
				}
			}
			if hasContact && hasMessage && validKeys(ts, keys) == len(ts) {
				if time.Since(stableSince) >= 5*time.Second {
					return captureComplete
				}
				m.setState("settling", "Keys authenticated; waiting for the database list to settle. / 密钥已校验，等待数据库列表稳定。")
			}
			return nil
		}
		err = windebug.Watch(ctx, p.pid, addresses, func() {
			m.mu.Lock()
			m.status.ReadyForLogin = true
			m.mu.Unlock()
			m.setState("watching_login", "Login watcher is ready. Log out and log in locally yourself; WxMori temporarily watches key setup and detaches on completion or stop. / 登录捕获已就绪，请亲自退出登录并重新登录；完成或停止后会解除临时调试。")
		}, func(key [32]byte) {
			if len(candidates) < 1024 {
				candidates[key] = true
			}
			probe.Verify(key, "login_key_setup", nil)
			for salt, key := range probe.Keys {
				keys[salt] = key
			}
			if e := m.publish(probe.Targets, keys, &saved); e != nil {
				operationErr = e
			}
			m.mu.Lock()
			m.status.Candidates = probe.Candidates
			m.status.BytesRead += 32
			m.mu.Unlock()
		}, refresh)
		if errors.Is(err, captureComplete) {
			m.setState("completed", "All required database keys were authenticated and saved locally, and the login watcher detached. Close WeChat normally before export if encrypted WAL exists. / 所需数据库密钥均已校验并保存，临时调试已解除；若存在加密 WAL，请正常关闭微信后导出。")
			return true, nil
		}
		if errors.Is(err, windebug.ErrProcessExited) {
			return true, errLoginRestart
		}
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		if f, ok := err.(*fault.Error); ok {
			return true, f
		}
		return true, fault.New("LOGIN_WATCH_FAILED", "登录捕获未能继续 / Login watcher could not continue", "Another debugger, a client restart, permissions or an unsupported build may prevent capture. Check that WeChat remains responsive; retry capture before logging in, or use mode=read_only for the non-debugging scanner.")
	}
	return false, nil
}
