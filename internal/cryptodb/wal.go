package cryptodb

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"

	"github.com/axiomzero-qed/WxMori/internal/fault"
)

type WALReport struct {
	ValidFrames       uint64 `json:"authenticated_frames"`
	CommittedFrames   uint64 `json:"committed_frames"`
	UncommittedFrames uint64 `json:"uncommitted_frames_excluded"`
	StaleTailBytes    int64  `json:"stale_tail_bytes_excluded"`
}

func walError(code string) error {
	return fault.New(code, "加密日志校验失败 / Encrypted WAL validation failed", "Use a stable database and matching WAL copy. No partial result is accepted; source files are never modified or deleted.")
}

// SQLite uses a rolling checksum over pairs of words. The magic selects word
// byte order; checksum values in the file headers themselves are big-endian.
func walChecksum(data []byte, order binary.ByteOrder, s [2]uint32) [2]uint32 {
	for at := 0; at+8 <= len(data); at += 8 {
		s[0] += order.Uint32(data[at:]) + s[1]
		s[1] += order.Uint32(data[at+4:]) + s[0]
	}
	return s
}

// ReplayWAL validates framing, rolling checksums and encrypted page HMACs before
// applying the last committed prefix to a private plaintext copy. Old-generation
// tails are recognized by salts, never confused with committed current frames.
// The caller must discard the private database on any returned error.
func ReplayWAL(ctx context.Context, walPath, database string, encKey, dbSalt []byte) (report WALReport, err error) {
	if err = ctx.Err(); err != nil {
		return report, err
	}
	if len(encKey) != 32 || len(dbSalt) != 16 {
		return report, walError("INVALID_WAL_KEY")
	}
	f, err := os.Open(walPath)
	if err != nil {
		return report, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return report, err
	}
	if st.Size() == 0 {
		return report, nil
	}
	var header [32]byte
	if _, err = io.ReadFull(f, header[:]); err != nil {
		return report, walError("WAL_TRUNCATED")
	}
	var order binary.ByteOrder
	switch binary.BigEndian.Uint32(header[:]) {
	case 0x377f0682:
		order = binary.LittleEndian
	case 0x377f0683:
		order = binary.BigEndian
	default:
		return report, walError("WAL_INVALID_HEADER")
	}
	if binary.BigEndian.Uint32(header[4:]) != 3007000 || binary.BigEndian.Uint32(header[8:]) != PageSize {
		return report, walError("WAL_UNSUPPORTED_FORMAT")
	}
	checksum := walChecksum(header[:24], order, [2]uint32{})
	if checksum[0] != binary.BigEndian.Uint32(header[24:]) || checksum[1] != binary.BigEndian.Uint32(header[28:]) {
		return report, walError("WAL_HEADER_CHECKSUM_FAILED")
	}
	base, err := os.Stat(database)
	if err != nil {
		return report, err
	}
	if base.Size()%PageSize != 0 {
		return report, walError("INVALID_DATABASE_SIZE")
	}
	mac := MACKey(encKey, dbSalt)
	defer clear(mac)
	frame := make([]byte, 24+PageSize)
	defer clear(frame)
	var finalPages uint32
	for offset := int64(32); ; offset += int64(len(frame)) {
		if err = ctx.Err(); err != nil {
			return report, err
		}
		n, e := io.ReadFull(f, frame)
		if e == io.EOF && n == 0 {
			break
		}
		// Persist-WAL/reset may retain a previous generation or allocated tail.
		// Once its salt differs, SQLite cannot use it as part of this generation.
		if n >= 16 && !bytes.Equal(frame[8:16], header[16:24]) {
			report.StaleTailBytes = st.Size() - offset
			break
		}
		if e != nil {
			return report, walError("WAL_TRUNCATED")
		}
		number := binary.BigEndian.Uint32(frame)
		if number == 0 {
			return report, walError("WAL_INVALID_FRAME")
		}
		checksum = walChecksum(frame[:8], order, checksum)
		checksum = walChecksum(frame[24:], order, checksum)
		if checksum[0] != binary.BigEndian.Uint32(frame[16:]) || checksum[1] != binary.BigEndian.Uint32(frame[20:]) {
			return report, walError("WAL_FRAME_CHECKSUM_FAILED")
		}
		if !Verify(frame[24:], mac, number) {
			return report, walError("WAL_HMAC_FAILED")
		}
		report.ValidFrames++
		if pages := binary.BigEndian.Uint32(frame[4:]); pages != 0 {
			// A valid commit cannot add more pages than the base plus its frames.
			if uint64(pages) > uint64(base.Size()/PageSize)+report.ValidFrames {
				return report, walError("WAL_INVALID_COMMIT")
			}
			finalPages = pages
			report.CommittedFrames = report.ValidFrames
		}
	}
	report.UncommittedFrames = report.ValidFrames - report.CommittedFrames
	if report.CommittedFrames == 0 {
		return report, nil
	}
	if _, err = f.Seek(32, io.SeekStart); err != nil {
		return report, err
	}
	out, err := os.OpenFile(database, os.O_RDWR, 0)
	if err != nil {
		return report, err
	}
	defer out.Close()
	for i := uint64(0); i < report.CommittedFrames; i++ {
		if err = ctx.Err(); err != nil {
			return report, err
		}
		if _, err = io.ReadFull(f, frame); err != nil {
			return report, walError("WAL_CHANGED")
		}
		number := binary.BigEndian.Uint32(frame)
		plain, e := DecryptPage(frame[24:], encKey, mac, number)
		if e != nil {
			return report, e
		}
		if number <= finalPages {
			_, err = out.WriteAt(plain, int64(number-1)*PageSize)
		}
		clear(plain)
		if err != nil {
			return report, err
		}
	}
	if err = out.Truncate(int64(finalPages) * PageSize); err != nil {
		return report, err
	}
	return report, out.Sync()
}
