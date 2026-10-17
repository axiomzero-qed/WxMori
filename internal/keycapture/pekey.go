package keycapture

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sort"
)

type keyEntry struct {
	RVA    uint32
	Prefix [32]byte
}
type functionRange struct{ begin, end uint32 }

// Find a narrowly recognized Microsoft-x64 AES key-expansion routine using its
// executable instructions and PE runtime-function boundaries, never a version
// offset. Unsupported or ambiguous layouts are rejected before any attachment.
func findKeyEntries(path string) ([]keyEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p, err := pe.NewFile(f)
	if err != nil {
		return nil, err
	}
	opt, ok := p.OptionalHeader.(*pe.OptionalHeader64)
	if !ok || p.Machine != pe.IMAGE_FILE_MACHINE_AMD64 || opt.NumberOfRvaAndSizes < 4 {
		return nil, errors.New("unsupported native image")
	}
	readRVA := func(rva, size uint32) ([]byte, error) {
		if size > 384<<20 {
			return nil, errors.New("oversized PE range")
		}
		for _, s := range p.Sections {
			if uint64(rva) >= uint64(s.VirtualAddress) && uint64(rva)+uint64(size) <= uint64(s.VirtualAddress)+uint64(s.Size) {
				b := make([]byte, int(size))
				_, err := f.ReadAt(b, int64(s.Offset)+int64(rva-s.VirtualAddress))
				return b, err
			}
		}
		return nil, io.ErrUnexpectedEOF
	}
	d := opt.DataDirectory[3]
	if d.Size == 0 || d.Size > 32<<20 || d.Size%12 != 0 {
		return nil, errors.New("unsupported function table")
	}
	table, err := readRVA(d.VirtualAddress, d.Size)
	if err != nil {
		return nil, err
	}
	ranges := make([]functionRange, 0, len(table)/12)
	for at := 0; at+12 <= len(table); at += 12 {
		r := functionRange{binary.LittleEndian.Uint32(table[at:]), binary.LittleEndian.Uint32(table[at+4:])}
		if r.end <= r.begin || (len(ranges) > 0 && r.begin < ranges[len(ranges)-1].begin) {
			return nil, errors.New("invalid function table")
		}
		ranges = append(ranges, r)
	}
	counts := map[int]int{}
	for _, s := range p.Sections {
		if s.Characteristics&pe.IMAGE_SCN_MEM_EXECUTE == 0 {
			continue
		}
		code, err := readRVA(s.VirtualAddress, s.Size)
		if err != nil {
			return nil, err
		}
		for start := 0; start < len(code); {
			at := bytes.Index(code[start:], []byte{0x66, 0x0f, 0x3a, 0xdf})
			if at < 0 {
				break
			}
			at += start
			rva := s.VirtualAddress + uint32(at)
			i := sort.Search(len(ranges), func(i int) bool { return ranges[i].begin > rva }) - 1
			if i >= 0 && rva < ranges[i].end {
				counts[i]++
			}
			start = at + 4
		}
	}
	var entries []keyEntry
	for i, n := range counts {
		r := ranges[i]
		if n < 8 || r.end-r.begin < 128 || r.end-r.begin > 4096 {
			continue
		}
		code, err := readRVA(r.begin, r.end-r.begin)
		if err != nil {
			return nil, err
		}
		if !keySetupABI(code) {
			continue
		}
		entry := keyEntry{RVA: r.begin}
		copy(entry.Prefix[:], code)
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].RVA < entries[j].RVA })
	if len(entries) == 0 || len(entries) > 4 {
		return nil, errors.New("no unambiguous supported AES key-setup routine")
	}
	return entries, nil
}

func keySetupABI(code []byte) bool {
	if len(code) < 128 {
		return false
	}
	head := code[:128]
	// RCX=input, EDX=128/192/256 bits, R8=output; the supported routine checks
	// both pointers and reads its first input block before expanding round keys.
	for _, op := range [][]byte{{0x48, 0x85, 0xc9}, {0x4d, 0x85, 0xc0}, {0x0f, 0x10, 0x01}, {0x81, 0xfa, 0x00, 0x01, 0x00, 0x00}, {0x81, 0xfa, 0xc0, 0x00, 0x00, 0x00}, {0x81, 0xfa, 0x80, 0x00, 0x00, 0x00}} {
		if !bytes.Contains(head, op) {
			return false
		}
	}
	return true
}
