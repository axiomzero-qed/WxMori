package cryptodb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/axiomzero-qed/WxMori/internal/fault"
	"golang.org/x/crypto/pbkdf2"
)

const (
	PageSize   = 4096
	Reserve    = 80
	BodySize   = PageSize - Reserve
	Iterations = 256000
)

var SQLiteHeader = []byte("SQLite format 3\x00")

func Derive(master, salt []byte) []byte { return pbkdf2.Key(master, salt, Iterations, 32, sha512.New) }
func MACKey(enc, salt []byte) []byte {
	s := append([]byte(nil), salt...)
	for i := range s {
		s[i] ^= 0x3a
	}
	return pbkdf2.Key(enc, s, 2, 32, sha512.New)
}

func Verify(page, mac []byte, number uint32) bool {
	if len(page) != PageSize || number == 0 {
		return false
	}
	start := 0
	if number == 1 {
		start = 16
	}
	h := hmac.New(sha512.New, mac)
	h.Write(page[start : PageSize-64])
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], number)
	h.Write(n[:])
	return hmac.Equal(h.Sum(nil), page[PageSize-64:])
}

func DecryptPage(page, enc, mac []byte, number uint32) ([]byte, error) {
	if !Verify(page, mac, number) {
		return nil, fault.New("HMAC_FAILED", "数据库页校验失败 / Database page authentication failed", fmt.Sprintf("Page %d: verify the master key and use a stable source snapshot. No partial database is accepted.", number))
	}
	b, err := aes.NewCipher(enc)
	if err != nil {
		return nil, err
	}
	out := make([]byte, PageSize)
	start := 0
	if number == 1 {
		start = 16
		copy(out, SQLiteHeader)
	}
	cipher.NewCBCDecrypter(b, page[BodySize:BodySize+16]).CryptBlocks(out[start:BodySize], page[start:BodySize])
	return out, nil
}

// DecryptFile authenticates every page and publishes only a complete private file.
func DecryptFile(ctx context.Context, src, dst string, master []byte) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if info.Size() < PageSize || info.Size()%PageSize != 0 {
		return fault.New("INVALID_DATABASE_SIZE", "数据库大小无效 / Invalid database size", "Expected complete 4096-byte pages. Check that the source copy is complete.")
	}
	page := make([]byte, PageSize)
	if _, err = io.ReadFull(in, page); err != nil {
		return err
	}
	if bytes.Equal(page[:16], SQLiteHeader) {
		return fault.New("ALREADY_PLAINTEXT", "数据库已解密 / Database is already SQLite", "Use decrypted_dir for existing SQLite databases.")
	}
	enc := Derive(master, page[:16])
	mac := MACKey(enc, page[:16])
	defer clear(enc)
	defer clear(mac)
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if e := out.Close(); err == nil {
			err = e
		}
		if err != nil {
			_ = os.Remove(dst)
		}
	}()
	for n := uint32(1); ; n++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		var plain []byte
		plain, err = DecryptPage(page, enc, mac, n)
		if err != nil {
			return err
		}
		if _, err = out.Write(plain); err != nil {
			return err
		}
		_, err = io.ReadFull(in, page)
		if err == io.EOF {
			err = nil
			break
		}
		if err != nil {
			return err
		}
	}
	return out.Sync()
}
