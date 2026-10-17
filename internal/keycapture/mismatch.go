package keycapture

import (
	"crypto/sha256"
	"path/filepath"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/config"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keyring"
)

type AccountMatch struct {
	DirectoryID      string `json:"directory_id"`
	DataDir          string `json:"data_dir"`
	MatchedDatabases int    `json:"matched_databases"`
}
type alternative struct {
	account config.Account
	probe   *Probe
}
type mismatchProbe struct {
	alternatives []alternative
	since        time.Time
}

func (p *mismatchProbe) Clear() {
	for _, a := range p.alternatives {
		a.probe.Clear()
	}
	p.alternatives = nil
}

// Inspect only first pages of at most eight local alternatives; never decrypt
// or persist their keys. AES candidates alone are not evidence of a mismatch.
func (p *mismatchProbe) reload(c config.Config) {
	previous := p.alternatives
	p.alternatives = nil
	defer func() {
		for _, a := range previous {
			a.probe.Clear()
		}
	}()
	for _, account := range config.Accounts(c.DataDir, "", c.Path) {
		if keyring.AccountID(account.DataDir) == keyring.AccountID(c.DataDir) {
			continue
		}
		if len(p.alternatives) >= 8 {
			break
		}
		ts, _, err := targets(account.DataDir)
		if err != nil {
			continue
		}
		selected := []Target{}
		contact, message := false, false
		for _, t := range ts {
			name := filepath.Base(t.Name)
			if name == "contact.db" && !contact {
				selected = append(selected, t)
				contact = true
			} else if requiredName.MatchString(name) && !message {
				selected = append(selected, t)
				message = true
			} else {
				clear(t.Page)
			}
		}
		if len(selected) > 0 {
			probe := NewProbe(selected)
			for _, old := range previous {
				if old.account.DataDir == account.DataDir {
					for _, key := range old.probe.Keys {
						probe.Verify(key, "other_account_page_hmac", nil)
					}
				}
			}
			p.alternatives = append(p.alternatives, alternative{account, probe})
		}
	}
}
func (p *mismatchProbe) observe(key [32]byte) {
	for _, a := range p.alternatives {
		a.probe.Verify(key, "other_account_page_hmac", nil)
	}
}
func (p *mismatchProbe) matches() []AccountMatch {
	out := []AccountMatch{}
	for _, a := range p.alternatives {
		contact, message := false, false
		for _, t := range a.probe.Targets {
			if _, ok := a.probe.Keys[t.Salt]; !ok {
				continue
			}
			if filepath.Base(t.Name) == "contact.db" {
				contact = true
			} else if requiredName.MatchString(filepath.Base(t.Name)) {
				message = true
			}
		}
		// Require independent contact and message evidence, not one AES miss.
		if contact && message && len(a.probe.Keys) >= 2 {
			out = append(out, AccountMatch{a.account.DirectoryID, a.account.DataDir, len(a.probe.Keys)})
		}
	}
	return out
}
func (p *mismatchProbe) check(now time.Time, selected int) error {
	matches := p.matches()
	if selected > 0 || len(matches) == 0 {
		p.since = time.Time{}
		return nil
	}
	if p.since.IsZero() {
		p.since = now
		return nil
	}
	if now.Sub(p.since) < 10*time.Second {
		return nil
	}
	return &fault.Error{Code: "KEY_ACCOUNT_MISMATCH", Message: "捕获的数据库密钥匹配其他账号 / Captured database keys match another account", Hint: "Contact and message page HMACs match another local account while none match the configured source. Capture has stopped and detached. Check the observed account paths and selected data_dir; use accounts and init --data-dir PATH --force to choose explicitly, or close other WeChat instances and retry. No account was switched and no other-account keys were saved. / 已停止并解除临时调试；请核对账号目录，必要时关闭其他微信实例后重试。未自动切换账号或保存其他账号密钥。", Candidates: matches}
}

// A rolling cache retains recent events even during long waits before login.
type candidateCache struct {
	values      [2048][32]byte
	present     map[[32]byte]bool
	next, count int
}

func (c *candidateCache) add(key [32]byte) bool {
	if c.present == nil {
		c.present = map[[32]byte]bool{}
	}
	if c.present[key] {
		return false
	}
	if c.count == len(c.values) {
		delete(c.present, c.values[c.next])
	} else {
		c.count++
	}
	c.values[c.next] = key
	c.present[key] = true
	c.next = (c.next + 1) % len(c.values)
	return true
}
func (c *candidateCache) clear() { clear(c.values[:]); clear(c.present); c.count = 0; c.next = 0 }

func pageHash(ts []Target) (out [32]byte) {
	h := sha256.New()
	for _, t := range ts {
		h.Write(t.Page)
	}
	copy(out[:], h.Sum(nil))
	return out
}
