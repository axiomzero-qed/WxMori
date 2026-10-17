package aesschedule

import (
	"encoding/hex"
	"testing"
)

func TestNISTScheduleAndRecognition(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i)
	}
	e := Expand(key)
	// Known AES-256 round expansion for the incrementing 00..1f example key.
	if got := hex.EncodeToString(e[32:48]); got != "a573c29fa176c498a97fce93a572c09c" {
		t.Fatalf("unexpected schedule %s", got)
	}
	for _, dec := range []bool{false, true} {
		for _, swapped := range []bool{false, true} {
			s := e
			if dec {
				s = DecryptionSchedule(key)
			}
			if swapped {
				for i := 0; i < Size; i += 4 {
					s[i], s[i+3] = s[i+3], s[i]
					s[i+1], s[i+2] = s[i+2], s[i+1]
				}
			}
			data := make([]byte, 1024)
			copy(data[12:], s[:])
			count := 0
			Scan(data, func(got [32]byte) bool {
				if got != key {
					t.Fatal("wrong key")
				}
				count++
				return true
			})
			if count != 1 {
				t.Fatalf("dec=%v swapped=%v count=%d", dec, swapped, count)
			}
			data[100] ^= 1
			count = 0
			Scan(data, func([32]byte) bool { count++; return true })
			if count != 0 {
				t.Fatal("partial schedule was accepted")
			}
		}
	}
}

func TestZeroAndRandomData(t *testing.T) {
	data := make([]byte, 4096)
	for i := 2048; i < len(data); i++ {
		data[i] = byte(i * 31)
	}
	Scan(data, func([32]byte) bool { t.Fatal("not a complete AES schedule"); return true })
}
