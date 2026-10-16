package cryptodb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestDerivationIndependentVector(t *testing.T) {
	master := make([]byte, 32)
	salt := make([]byte, 16)
	for i := range master {
		master[i] = byte(i)
	}
	for i := range salt {
		salt[i] = byte(i)
	}
	// Generated independently with Python hashlib.pbkdf2_hmac, not this implementation.
	want := "d6d2e116eae459f966d0f24ffeba9c0eca5f0c66c4574816378c04dae2425862"
	if got := hex.EncodeToString(Derive(master, salt)); got != want {
		t.Fatalf("derived %s", got)
	}
	other := append([]byte(nil), salt...)
	other[0] ^= 1
	if bytes.Equal(Derive(master, salt), Derive(master, other)) {
		t.Fatal("salts must produce different keys")
	}
}

func encryptPage(plain, enc, mac, salt []byte, n uint32) []byte {
	page := make([]byte, PageSize)
	start := 0
	if n == 1 {
		start = 16
		copy(page, salt)
	}
	iv := bytes.Repeat([]byte{byte(n)}, 16)
	copy(page[BodySize:], iv)
	b, _ := aes.NewCipher(enc)
	cipher.NewCBCEncrypter(b, iv).CryptBlocks(page[start:BodySize], plain[start:BodySize])
	h := hmac.New(sha512.New, mac)
	h.Write(page[start : PageSize-64])
	var number [4]byte
	binary.LittleEndian.PutUint32(number[:], n)
	h.Write(number[:])
	copy(page[PageSize-64:], h.Sum(nil))
	return page
}

func TestMultiplePageCBCAndAuthentication(t *testing.T) {
	enc := bytes.Repeat([]byte{0x13}, 32)
	salt := bytes.Repeat([]byte{0x79}, 16)
	mac := MACKey(enc, salt)
	for _, n := range []uint32{1, 2, 61} {
		plain := make([]byte, PageSize)
		for i := 0; i < BodySize; i++ {
			plain[i] = byte(i*31 + int(n))
		}
		if n == 1 {
			copy(plain, SQLiteHeader)
		}
		page := encryptPage(plain, enc, mac, salt, n)
		got, err := DecryptPage(page, enc, mac, n)
		if err != nil || !bytes.Equal(got, plain) {
			t.Fatalf("page %d CBC mismatch: %v", n, err)
		}
		if Verify(page, mac, n+1) {
			t.Fatal("page number must be authenticated")
		}
		for _, at := range []int{100, BodySize, PageSize - 1} {
			bad := append([]byte(nil), page...)
			bad[at] ^= 1
			if _, err := DecryptPage(bad, enc, mac, n); err == nil {
				t.Fatalf("tamper %d accepted", at)
			}
		}
	}
}

func TestCorruptLaterPageLeavesNoDatabase(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "encrypted"), filepath.Join(dir, "out")
	master := bytes.Repeat([]byte{0x25}, 32)
	salt := bytes.Repeat([]byte{0xa1}, 16)
	enc := Derive(master, salt)
	mac := MACKey(enc, salt)
	plain := make([]byte, PageSize)
	a := encryptPage(plain, enc, mac, salt, 1)
	b := encryptPage(plain, enc, mac, salt, 2)
	b[33] ^= 1
	if err := os.WriteFile(src, append(a, b...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := DecryptFile(context.Background(), src, dst, master); err == nil {
		t.Fatal("corrupt page accepted")
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatal("partial database left behind")
	}
}
