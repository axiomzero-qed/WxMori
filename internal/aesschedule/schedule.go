// Package aesschedule recognizes complete AES-256 round-key schedules.
// Recognition produces candidates only; database authentication is mandatory.
package aesschedule

import "encoding/binary"

const Size = 240

var sbox = func() [256]byte {
	var out [256]byte
	for n := range out {
		x, y, power := byte(n), byte(1), 254
		for power > 0 {
			if power&1 != 0 {
				y = mul(y, x)
			}
			x = mul(x, x)
			power >>= 1
		}
		if n == 0 {
			y = 0
		}
		rot := func(n uint) byte { return y<<n | y>>(8-n) }
		out[n] = y ^ rot(1) ^ rot(2) ^ rot(3) ^ rot(4) ^ 0x63
	}
	return out
}()

func mul(a, b byte) byte {
	var out byte
	for b != 0 {
		if b&1 != 0 {
			out ^= a
		}
		if a&0x80 != 0 {
			a = (a << 1) ^ 0x1b
		} else {
			a <<= 1
		}
		b >>= 1
	}
	return out
}
func sub(x uint32) uint32 {
	return uint32(sbox[byte(x>>24)])<<24 | uint32(sbox[byte(x>>16)])<<16 | uint32(sbox[byte(x>>8)])<<8 | uint32(sbox[byte(x)])
}

// Expand follows FIPS 197 KEYEXPANSION for AES-256 (14 rounds).
func Expand(key [32]byte) [Size]byte {
	var w [60]uint32
	for i := 0; i < 8; i++ {
		w[i] = binary.BigEndian.Uint32(key[i*4:])
	}
	rcon := byte(1)
	for i := 8; i < 60; i++ {
		v := w[i-1]
		if i%8 == 0 {
			v = sub(v<<8|v>>24) ^ uint32(rcon)<<24
			rcon = mul(rcon, 2)
		} else if i%8 == 4 {
			v = sub(v)
		}
		w[i] = w[i-8] ^ v
	}
	var out [Size]byte
	for i, v := range w {
		binary.BigEndian.PutUint32(out[i*4:], v)
	}
	return out
}

func column(block []byte, inverse bool) {
	for i := 0; i < 16; i += 4 {
		a, b, c, d := block[i], block[i+1], block[i+2], block[i+3]
		if inverse {
			block[i] = mul(a, 14) ^ mul(b, 11) ^ mul(c, 13) ^ mul(d, 9)
			block[i+1] = mul(a, 9) ^ mul(b, 14) ^ mul(c, 11) ^ mul(d, 13)
			block[i+2] = mul(a, 13) ^ mul(b, 9) ^ mul(c, 14) ^ mul(d, 11)
			block[i+3] = mul(a, 11) ^ mul(b, 13) ^ mul(c, 9) ^ mul(d, 14)
		} else {
			block[i] = mul(a, 2) ^ mul(b, 3) ^ c ^ d
			block[i+1] = a ^ mul(b, 2) ^ mul(c, 3) ^ d
			block[i+2] = a ^ b ^ mul(c, 2) ^ mul(d, 3)
			block[i+3] = mul(a, 3) ^ b ^ c ^ mul(d, 2)
		}
	}
}

func DecryptionSchedule(key [32]byte) [Size]byte {
	e := Expand(key)
	var d [Size]byte
	for round := 0; round < 15; round++ {
		copy(d[round*16:], e[(14-round)*16:(15-round)*16])
		if round > 0 && round < 14 {
			column(d[round*16:(round+1)*16], true)
		}
	}
	return d
}

func match(data []byte, expected [Size]byte, swapped bool) bool {
	for i, v := range expected {
		at := i
		if swapped {
			at = i&^3 | 3 - i&3
		}
		if data[at] != v {
			return false
		}
	}
	return true
}
func keyBytes(data []byte, swapped bool) (key [32]byte) {
	for i := range key {
		at := i
		if swapped {
			at = i&^3 | 3 - i&3
		}
		key[i] = data[at]
	}
	return key
}

// Scan accepts canonical AES-NI byte order and reversed bytes within uint32 words.
// Data starts at a four-byte aligned process address. emit never receives dumps.
func Scan(data []byte, emit func([32]byte) bool) {
	for i := 0; i+Size <= len(data); i += 4 {
		b := data[i:]
		w8 := binary.LittleEndian.Uint32(b[32:])
		w9 := binary.LittleEndian.Uint32(b[36:])
		w10 := binary.LittleEndian.Uint32(b[40:])
		w11 := binary.LittleEndian.Uint32(b[44:])
		if w8|w9|w10|w11 == 0 {
			continue
		}
		enc := w9^w8 == binary.LittleEndian.Uint32(b[4:]) && w10^w9 == binary.LittleEndian.Uint32(b[8:]) && w11^w10 == binary.LittleEndian.Uint32(b[12:])
		dec := w9^w8 == binary.LittleEndian.Uint32(b[68:]) && w10^w9 == binary.LittleEndian.Uint32(b[72:]) && w11^w10 == binary.LittleEndian.Uint32(b[76:])
		if !enc && !dec {
			continue
		}
		for _, swapped := range []bool{false, true} {
			if enc {
				key := keyBytes(b, swapped)
				if match(b, Expand(key), swapped) {
					if !emit(key) {
						return
					}
				}
			}
			if dec {
				var key [32]byte
				last := keyBytes(b[208:], swapped)
				copy(key[:16], last[16:32])
				copy(key[16:], last[:16])
				column(key[16:], false)
				if match(b, DecryptionSchedule(key), swapped) {
					if !emit(key) {
						return
					}
				}
			}
		}
	}
}
