package keycapture

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/aesschedule"
	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
)

func fixtureTarget() (Target, [32]byte) {
	var key [32]byte
	for i := range key {
		key[i] = byte(101 + i)
	}
	var salt [16]byte
	for i := range salt {
		salt[i] = byte(i + 19)
	}
	return encryptedTarget(key, salt), key
}

func encryptedTarget(key [32]byte, salt [16]byte) Target {
	page := make([]byte, cryptodb.PageSize)
	copy(page, salt[:])
	iv := bytes.Repeat([]byte{0x3d}, 16)
	copy(page[cryptodb.BodySize:], iv)
	plain := bytes.Repeat([]byte{0x7c}, cryptodb.BodySize-16)
	block, _ := aes.NewCipher(key[:])
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(page[16:cryptodb.BodySize], plain)
	mac := cryptodb.MACKey(key[:], salt[:])
	defer clear(mac)
	h := hmac.New(sha512.New, mac)
	h.Write(page[16 : cryptodb.PageSize-64])
	h.Write([]byte{1, 0, 0, 0})
	copy(page[cryptodb.PageSize-64:], h.Sum(nil))
	return Target{Salt: salt, Page: page, Name: "message/message_0.db"}
}

type fakeMemory struct {
	base uintptr
	data []byte
}

func (f fakeMemory) Contains(at uintptr, n int) bool {
	return at >= f.base && at <= f.base+uintptr(len(f.data)) && uintptr(n) <= f.base+uintptr(len(f.data))-at
}
func (f fakeMemory) Read(at uintptr, out []byte) (int, error) {
	if !f.Contains(at, len(out)) {
		return 0, nil
	}
	return copy(out, f.data[at-f.base:]), nil
}

func TestScheduleNeedsDatabaseAuthentication(t *testing.T) {
	target, key := fixtureTarget()
	p := NewProbe([]Target{target})
	defer p.Clear()
	buffer := make([]byte, 1024)
	other := key
	other[0] ^= 1
	s := aesschedule.Expand(other)
	copy(buffer[32:], s[:])
	mem := fakeMemory{0x10000, buffer}
	p.Observe(mem, mem.base, buffer)
	if p.Complete() {
		t.Fatal("AES structure alone is not a valid database key")
	}
	s = aesschedule.Expand(key)
	copy(buffer[32:], s[:])
	p.Observe(mem, mem.base, buffer)
	if !p.Complete() || p.Keys[target.Salt] != key {
		t.Fatal("authenticated schedule not recovered")
	}
}

func TestSaltReferenceTwoHop(t *testing.T) {
	target, key := fixtureTarget()
	p := NewProbe([]Target{target})
	defer p.Clear()
	mem := fakeMemory{base: 0x10000, data: make([]byte, 4096)}
	copy(mem.data[128:], target.Salt[:])
	copy(mem.data[2048:], key[:])
	binary.LittleEndian.PutUint64(mem.data[512:], uint64(mem.base+128))
	binary.LittleEndian.PutUint64(mem.data[520:], uint64(mem.base+1024))
	binary.LittleEndian.PutUint64(mem.data[1032:], uint64(mem.base+2048))
	p.Observe(mem, mem.base, mem.data)
	if !p.Complete() {
		t.Fatal("two-hop key not authenticated")
	}
}

func TestWrongSaltAndTruncatedPageRejected(t *testing.T) {
	target, key := fixtureTarget()
	p := NewProbe([]Target{target})
	defer p.Clear()
	wrong := target.Salt
	wrong[0] ^= 1
	if p.Verify(key, "test", &wrong) {
		t.Fatal("wrong account/salt accepted")
	}
	target.Page = target.Page[:123]
	p2 := NewProbe([]Target{target})
	if p2.Verify(key, "test", nil) {
		t.Fatal("truncated page accepted")
	}
}
