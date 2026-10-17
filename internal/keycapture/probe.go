package keycapture

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"github.com/axiomzero-qed/WxMori/internal/aesschedule"
	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
)

type Target struct {
	Salt [16]byte
	Page []byte
	Name string
}
type Region struct{ Base, Size uintptr }
type Memory interface {
	Read(uintptr, []byte) (int, error)
	Contains(uintptr, int) bool
}

// Probe keeps candidate data in memory only. Nothing is a key until HMAC verifies.
type Probe struct {
	Targets     []Target
	Keys        map[[16]byte][32]byte
	anchors     map[uintptr][16]byte
	seen        map[[32]byte]bool
	checkedRefs map[uintptr]bool
	Candidates  int
	Sources     map[string]int
}

func NewProbe(targets []Target) *Probe {
	return &Probe{Targets: targets, Keys: map[[16]byte][32]byte{}, anchors: map[uintptr][16]byte{}, seen: map[[32]byte]bool{}, checkedRefs: map[uintptr]bool{}, Sources: map[string]int{}}
}
func (p *Probe) Clear() {
	clear(p.Keys)
	clear(p.seen)
	clear(p.anchors)
	clear(p.checkedRefs)
	for i := range p.Targets {
		clear(p.Targets[i].Page)
	}
}
func (p *Probe) BeginPass() { clear(p.checkedRefs) }
func (p *Probe) Verify(key [32]byte, source string, onlySalt *[16]byte) bool {
	if onlySalt == nil && p.seen[key] {
		return false
	}
	if onlySalt == nil && len(p.seen) < 8192 {
		p.seen[key] = true
	}
	p.Candidates++
	matched := false
	for _, target := range p.Targets {
		if onlySalt != nil && target.Salt != *onlySalt {
			continue
		}
		if existing, ok := p.Keys[target.Salt]; ok && existing == key {
			continue
		}
		mac := cryptodb.MACKey(key[:], target.Salt[:])
		valid := cryptodb.Verify(target.Page, mac, 1)
		clear(mac)
		if valid {
			p.Keys[target.Salt] = key
			p.Sources[source]++
			matched = true
		}
	}
	return matched
}
func (p *Probe) Complete() bool {
	if len(p.Targets) == 0 {
		return false
	}
	for _, t := range p.Targets {
		if _, ok := p.Keys[t.Salt]; !ok {
			return false
		}
	}
	return true
}
func (p *Probe) Verified() int {
	n := 0
	for _, t := range p.Targets {
		if _, ok := p.Keys[t.Salt]; ok {
			n++
		}
	}
	return n
}

func (p *Probe) Observe(mem Memory, base uintptr, data []byte) {
	// Find only the selected account's known database salts, not arbitrary secrets.
	for _, t := range p.Targets {
		start := 0
		for start+16 <= len(data) {
			at := bytes.Index(data[start:], t.Salt[:])
			if at < 0 {
				break
			}
			at += start
			if len(p.anchors) < 4096 {
				p.anchors[base+uintptr(at)] = t.Salt
			}
			start = at + 16
		}
	}
	aesschedule.Scan(data, func(key [32]byte) bool { p.Verify(key, "aes256_schedule", nil); return !p.Complete() })
	if p.Complete() {
		return
	}
	// Earlier versions store x'<32-byte key><16-byte salt>' as ASCII.
	for start := 0; start+99 <= len(data); {
		at := bytes.Index(data[start:], []byte("x'"))
		if at < 0 {
			break
		}
		at += start
		if at+99 <= len(data) && data[at+98] == '\'' {
			var decoded [48]byte
			if _, err := hex.Decode(decoded[:], data[at+2:at+98]); err == nil {
				var salt [16]byte
				var key [32]byte
				copy(salt[:], decoded[32:])
				copy(key[:], decoded[:32])
				p.Verify(key, "legacy_raw_key", &salt)
			}
			clear(decoded[:])
		}
		start = at + 2
	}
	if p.Complete() {
		return
	}
	// SQLCipher codec objects reference their salt and per-cipher key objects.
	// Resolve a bounded, two-hop pointer neighborhood; no hardcoded WeChat offsets.
	for i := 0; i+8 <= len(data); i += 8 {
		address := uintptr(binary.LittleEndian.Uint64(data[i:]))
		salt, ok := p.anchors[address]
		if !ok {
			continue
		}
		ref := base + uintptr(i)
		if p.checkedRefs[ref] {
			continue
		}
		if len(p.checkedRefs) >= 2048 {
			return
		}
		p.checkedRefs[ref] = true
		from, to := max(0, i-128), min(len(data), i+256)
		p.follow(mem, data[from:to], salt)
		if p.Complete() {
			return
		}
	}
}

func (p *Probe) follow(mem Memory, window []byte, salt [16]byte) {
	seen := map[uintptr]bool{}
	budget := 160
	var inspect func([]byte, int)
	inspect = func(block []byte, depth int) {
		for i := 0; i+8 <= len(block) && budget > 0; i += 8 {
			at := uintptr(binary.LittleEndian.Uint64(block[i:]))
			if at < 0x10000 || seen[at] || !mem.Contains(at, 32) {
				continue
			}
			seen[at] = true
			budget--
			length := 32
			if depth > 0 && mem.Contains(at, 128) {
				length = 128
			}
			var buffer [128]byte
			n, err := mem.Read(at, buffer[:length])
			if n < 32 {
				continue
			}
			_ = err
			var key [32]byte
			copy(key[:], buffer[:32])
			if p.Verify(key, "salt_referenced_key", &salt) {
				clear(buffer[:])
				continue
			}
			if depth > 0 {
				inspect(buffer[:n], depth-1)
			}
			clear(buffer[:])
		}
	}
	inspect(window, 1)
}
