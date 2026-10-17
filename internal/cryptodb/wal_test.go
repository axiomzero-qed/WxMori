package cryptodb_test

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/axiomzero-qed/WxMori/internal/cryptodb"
)

var (
	testWALKey = bytes.Repeat([]byte{0x37}, 32)
	testDBSalt = bytes.Repeat([]byte{0x91}, 16)
)

func TestReplayWALChecksumByteOrdersAndPageOneHeader(t *testing.T) {
	for _, tc := range []struct {
		name  string
		magic uint32
		order binary.ByteOrder
	}{
		{name: "little checksum", magic: 0x377f0682, order: binary.LittleEndian},
		{name: "big checksum", magic: 0x377f0683, order: binary.BigEndian},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plain := plainWALPage(1, 0x45)
			wal := makeSyntheticWAL(tc.magic, [][]byte{encryptedWALPage(plain, 1)}, []uint32{1}, []uint32{1}, nil)
			base, walPath := writeReplayInputs(t, [][]byte{plainWALPage(1, 0x12)}, wal)
			report, err := cryptodb.ReplayWAL(context.Background(), walPath, base, testWALKey, testDBSalt)
			if err != nil {
				t.Fatal(err)
			}
			if report.ValidFrames != 1 || report.CommittedFrames != 1 || report.UncommittedFrames != 0 || report.StaleTailBytes != 0 {
				t.Fatalf("unexpected replay report: %+v", report)
			}
			got := readPage(t, base, 1)
			if !bytes.Equal(got, plain) {
				for i := range got {
					if got[i] != plain[i] {
						t.Fatalf("page mismatch at byte %d: got %02x expected %02x", i, got[i], plain[i])
					}
				}
			}
		})
	}
}

func TestReplayWALKeepsLastCommitAndDropsUncommittedFrame(t *testing.T) {
	page1 := plainWALPage(1, 0x21)
	page2Commit := plainWALPage(2, 0x32)
	page2Tail := plainWALPage(2, 0x43)
	wal := makeSyntheticWAL(0x377f0682,
		[][]byte{encryptedWALPage(page1, 1), encryptedWALPage(page2Commit, 2), encryptedWALPage(page2Tail, 2)},
		[]uint32{1, 2, 2}, []uint32{1, 2, 0}, nil)
	base, walPath := writeReplayInputs(t, [][]byte{plainWALPage(1, 0x10), plainWALPage(2, 0x11)}, wal)
	report, err := cryptodb.ReplayWAL(context.Background(), walPath, base, testWALKey, testDBSalt)
	if err != nil {
		t.Fatal(err)
	}
	if report.ValidFrames != 3 || report.CommittedFrames != 2 || report.UncommittedFrames != 1 {
		t.Fatalf("unexpected replay report: %+v", report)
	}
	if got := readPage(t, base, 1); !bytes.Equal(got, page1) {
		t.Fatal("first committed page was not replayed")
	}
	if got := readPage(t, base, 2); !bytes.Equal(got, page2Commit) {
		t.Fatal("uncommitted tail page was replayed")
	}
}

func TestReplayWALStopsAtStaleGeneration(t *testing.T) {
	committed := plainWALPage(1, 0x54)
	oldSalt := bytes.Repeat([]byte{0x22}, 8)
	wal := makeSyntheticWAL(0x377f0683,
		[][]byte{encryptedWALPage(committed, 1), encryptedWALPage(plainWALPage(1, 0x65), 1)},
		[]uint32{1, 1}, []uint32{1, 1}, map[int][]byte{1: oldSalt})
	base, walPath := writeReplayInputs(t, [][]byte{plainWALPage(1, 0x11)}, wal)
	report, err := cryptodb.ReplayWAL(context.Background(), walPath, base, testWALKey, testDBSalt)
	if err != nil {
		t.Fatal(err)
	}
	if report.ValidFrames != 1 || report.CommittedFrames != 1 || report.StaleTailBytes != int64(24+cryptodb.PageSize) {
		t.Fatalf("unexpected stale generation report: %+v", report)
	}
	if got := readPage(t, base, 1); !bytes.Equal(got, committed) {
		t.Fatal("stale-generation tail was replayed")
	}
}

func TestReplayWALRejectsCorruptionAndValidGenerationTruncation(t *testing.T) {
	basePage := plainWALPage(1, 0x16)
	makeBase := func(t *testing.T, wal []byte) (string, string) { return writeReplayInputs(t, [][]byte{basePage}, wal) }
	tests := []struct {
		name string
		wal  func() []byte
	}{
		{name: "bad header checksum", wal: func() []byte { w := oneCommitWAL(); w[24] ^= 1; return w }},
		{name: "bad frame checksum", wal: func() []byte { w := oneCommitWAL(); w[32+16] ^= 1; return w }},
		{name: "bad SQLCipher page authentication despite valid WAL checksum", wal: func() []byte {
			w := oneCommitWAL()
			w[32+24+400] ^= 1
			rechecksumWALFrame(w, binary.LittleEndian, 0, nil)
			return w
		}},
		{name: "partial valid frame", wal: func() []byte { w := oneCommitWAL(); return append(w, 0x01, 0x02, 0x03) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base, walPath := makeBase(t, tc.wal())
			if _, err := cryptodb.ReplayWAL(context.Background(), walPath, base, testWALKey, testDBSalt); err == nil {
				t.Fatal("invalid WAL was accepted")
			}
		})
	}
}

func oneCommitWAL() []byte {
	return makeSyntheticWAL(0x377f0682, [][]byte{encryptedWALPage(plainWALPage(1, 0x27), 1)}, []uint32{1}, []uint32{1}, nil)
}

func plainWALPage(pageNo uint32, fill byte) []byte {
	page := bytes.Repeat([]byte{fill}, cryptodb.PageSize)
	clear(page[cryptodb.BodySize:])
	if pageNo == 1 {
		copy(page, []byte("SQLite format 3\x00"))
		binary.BigEndian.PutUint16(page[16:18], cryptodb.PageSize)
		page[20] = cryptodb.Reserve
	}
	return page
}

// This fixture implementation deliberately builds the SQLCipher page bytes and
// WAL checksums in the test, independently of ReplayWAL and its parsers.
func encryptedWALPage(plain []byte, pageNo uint32) []byte {
	page := make([]byte, cryptodb.PageSize)
	start := 0
	if pageNo == 1 {
		start = 16
		copy(page[:16], testDBSalt)
	}
	iv := bytes.Repeat([]byte{byte(pageNo + 5)}, aes.BlockSize)
	copy(page[cryptodb.BodySize:cryptodb.BodySize+aes.BlockSize], iv)
	block, _ := aes.NewCipher(testWALKey)
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(page[start:cryptodb.BodySize], plain[start:cryptodb.BodySize])
	macKey := independentMACKey(testWALKey, testDBSalt)
	h := hmac.New(sha512.New, macKey)
	h.Write(page[start : cryptodb.PageSize-64])
	var number [4]byte
	binary.LittleEndian.PutUint32(number[:], pageNo)
	h.Write(number[:])
	copy(page[cryptodb.PageSize-64:], h.Sum(nil))
	return page
}

func independentMACKey(encKey, salt []byte) []byte {
	modified := append([]byte(nil), salt...)
	for i := range modified {
		modified[i] ^= 0x3a
	}
	firstInput := append(append([]byte(nil), modified...), 0, 0, 0, 1)
	h := hmac.New(sha512.New, encKey)
	h.Write(firstInput)
	u1 := h.Sum(nil)
	h = hmac.New(sha512.New, encKey)
	h.Write(u1)
	u2 := h.Sum(nil)
	out := make([]byte, 32)
	for i := range out {
		out[i] = u1[i] ^ u2[i]
	}
	return out
}

func makeSyntheticWAL(magic uint32, pages [][]byte, pageNumbers, commitSizes []uint32, frameSalts map[int][]byte) []byte {
	wal := make([]byte, 32+len(pages)*(24+cryptodb.PageSize))
	binary.BigEndian.PutUint32(wal[0:], magic)
	binary.BigEndian.PutUint32(wal[4:], 3007000)
	binary.BigEndian.PutUint32(wal[8:], cryptodb.PageSize)
	binary.BigEndian.PutUint32(wal[12:], 7)
	copy(wal[16:24], []byte("WALsalt!"))
	var checksumOrder binary.ByteOrder = binary.LittleEndian
	if magic == 0x377f0683 {
		checksumOrder = binary.BigEndian
	}
	c1, c2 := walChecksum(checksumOrder, 0, 0, wal[:24])
	binary.BigEndian.PutUint32(wal[24:], c1)
	binary.BigEndian.PutUint32(wal[28:], c2)
	for i, page := range pages {
		at := 32 + i*(24+cryptodb.PageSize)
		binary.BigEndian.PutUint32(wal[at:], pageNumbers[i])
		binary.BigEndian.PutUint32(wal[at+4:], commitSizes[i])
		copy(wal[at+8:at+16], wal[16:24])
		if salt, ok := frameSalts[i]; ok {
			copy(wal[at+8:at+16], salt)
		}
		copy(wal[at+24:], page)
		c1, c2 = walChecksum(checksumOrder, c1, c2, wal[at:at+8], page)
		binary.BigEndian.PutUint32(wal[at+16:], c1)
		binary.BigEndian.PutUint32(wal[at+20:], c2)
	}
	return wal
}

func walChecksum(order binary.ByteOrder, s1, s2 uint32, parts ...[]byte) (uint32, uint32) {
	var pair [8]byte
	for _, part := range parts {
		for i := 0; i+8 <= len(part); i += 8 {
			a := order.Uint32(part[i:])
			b := order.Uint32(part[i+4:])
			s1 += a + s2
			s2 += b + s1
		}
	}
	order.PutUint32(pair[:4], s1)
	order.PutUint32(pair[4:], s2)
	return order.Uint32(pair[:4]), order.Uint32(pair[4:])
}

func rechecksumWALFrame(wal []byte, order binary.ByteOrder, frameIndex int, previous []uint32) {
	at := 32 + frameIndex*(24+cryptodb.PageSize)
	s1, s2 := binary.BigEndian.Uint32(wal[24:]), binary.BigEndian.Uint32(wal[28:])
	if len(previous) == 2 {
		s1, s2 = previous[0], previous[1]
	}
	s1, s2 = walChecksum(order, s1, s2, wal[at:at+8], wal[at+24:at+24+cryptodb.PageSize])
	binary.BigEndian.PutUint32(wal[at+16:], s1)
	binary.BigEndian.PutUint32(wal[at+20:], s2)
}

func writeReplayInputs(t *testing.T, pages [][]byte, wal []byte) (string, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath, walPath := filepath.Join(dir, "plain.sqlite"), filepath.Join(dir, "synthetic-wal")
	var base []byte
	for _, page := range pages {
		base = append(base, page...)
	}
	if err := os.WriteFile(dbPath, base, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(walPath, wal, 0600); err != nil {
		t.Fatal(err)
	}
	return dbPath, walPath
}

func readPage(t *testing.T, path string, pageNo uint32) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start := int(pageNo-1) * cryptodb.PageSize
	return append([]byte(nil), b[start:start+cryptodb.PageSize]...)
}
