package keycapture

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestFindKeyEntriesRecognizesSupportedBoundedRoutine(t *testing.T) {
	path := writeSyntheticPE(t, syntheticPEOptions{functions: 1})
	entries, err := findKeyEntries(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RVA != 0x1000 {
		t.Fatalf("unexpected recognized entries: %+v", entries)
	}
}

func TestFindKeyEntriesRejectsUnsupportedPELayouts(t *testing.T) {
	tests := []struct {
		name    string
		options syntheticPEOptions
	}{
		{name: "missing key size checks", options: syntheticPEOptions{functions: 1, omitBitsCheck: true}},
		{name: "incorrect machine", options: syntheticPEOptions{functions: 1, machine: 0x01c0}},
		{name: "missing exception table", options: syntheticPEOptions{functions: 1, noExceptionTable: true}},
		{name: "ambiguous candidates", options: syntheticPEOptions{functions: 5}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := findKeyEntries(writeSyntheticPE(t, tt.options)); err == nil {
				t.Fatal("unsupported PE layout was accepted")
			}
		})
	}
}

func TestFindKeyEntriesRejectsTruncatedPE(t *testing.T) {
	path := filepath.Join(t.TempDir(), "truncated.exe")
	if err := os.WriteFile(path, []byte("MZ"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := findKeyEntries(path); err == nil {
		t.Fatal("truncated PE was accepted")
	}
}

type syntheticPEOptions struct {
	functions        int
	machine          uint16
	omitBitsCheck    bool
	noExceptionTable bool
}

func writeSyntheticPE(t *testing.T, options syntheticPEOptions) string {
	t.Helper()
	if options.machine == 0 {
		options.machine = 0x8664
	}
	const (
		peOffset     = 0x80
		optionalSize = 240
		sectionTable = peOffset + 4 + 20 + optionalSize
		codeRaw      = 0x200
		codeSize     = 0x800
		functionRVA  = 0x1000
		exceptionRVA = 0x2000
		exceptionRaw = 0xa00
		fileSize     = 0xc00
	)
	data := make([]byte, fileSize)
	copy(data, "MZ")
	binary.LittleEndian.PutUint32(data[0x3c:], peOffset)
	copy(data[peOffset:], "PE\x00\x00")
	coff := data[peOffset+4:]
	binary.LittleEndian.PutUint16(coff[0:], options.machine)
	binary.LittleEndian.PutUint16(coff[2:], 2)
	binary.LittleEndian.PutUint16(coff[16:], optionalSize)
	binary.LittleEndian.PutUint16(coff[18:], 0x0022)
	opt := data[peOffset+24:]
	binary.LittleEndian.PutUint16(opt[0:], 0x20b)
	binary.LittleEndian.PutUint32(opt[4:], codeSize)
	binary.LittleEndian.PutUint32(opt[16:], functionRVA)
	binary.LittleEndian.PutUint32(opt[20:], functionRVA)
	binary.LittleEndian.PutUint64(opt[24:], 0x140000000)
	binary.LittleEndian.PutUint32(opt[32:], 0x1000)
	binary.LittleEndian.PutUint32(opt[36:], 0x200)
	binary.LittleEndian.PutUint32(opt[56:], 0x3000)
	binary.LittleEndian.PutUint32(opt[60:], 0x200)
	binary.LittleEndian.PutUint16(opt[68:], 3)
	binary.LittleEndian.PutUint32(opt[108:], 16)
	if !options.noExceptionTable {
		binary.LittleEndian.PutUint32(opt[112+3*8:], exceptionRVA)
		binary.LittleEndian.PutUint32(opt[112+3*8+4:], uint32(options.functions*12))
	}
	sections := data[sectionTable:]
	copy(sections[0:], ".text\x00\x00\x00")
	binary.LittleEndian.PutUint32(sections[8:], codeSize)
	binary.LittleEndian.PutUint32(sections[12:], functionRVA)
	binary.LittleEndian.PutUint32(sections[16:], codeSize)
	binary.LittleEndian.PutUint32(sections[20:], codeRaw)
	binary.LittleEndian.PutUint32(sections[36:], 0x60000020)
	exception := sections[40:]
	copy(exception[0:], ".pdata\x00\x00")
	binary.LittleEndian.PutUint32(exception[8:], 0x200)
	binary.LittleEndian.PutUint32(exception[12:], exceptionRVA)
	binary.LittleEndian.PutUint32(exception[16:], 0x200)
	binary.LittleEndian.PutUint32(exception[20:], exceptionRaw)
	binary.LittleEndian.PutUint32(exception[36:], 0x40000040)
	for i := 0; i < options.functions; i++ {
		begin := functionRVA + uint32(i*0x100)
		code := data[codeRaw+i*0x100 : codeRaw+(i+1)*0x100]
		putSupportedABI(code, options.omitBitsCheck)
		for n := 0; n < 8; n++ {
			copy(code[64+n*5:], []byte{0x66, 0x0f, 0x3a, 0xdf, byte(n)})
		}
		at := exceptionRaw + i*12
		binary.LittleEndian.PutUint32(data[at:], begin)
		binary.LittleEndian.PutUint32(data[at+4:], begin+0x100)
		binary.LittleEndian.PutUint32(data[at+8:], exceptionRVA+uint32(options.functions*12))
	}
	path := filepath.Join(t.TempDir(), "synthetic.exe")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func putSupportedABI(code []byte, omitBitsCheck bool) {
	ops := [][]byte{
		{0x48, 0x85, 0xc9},
		{0x4d, 0x85, 0xc0},
		{0x0f, 0x10, 0x01},
		{0x81, 0xfa, 0x00, 0x01, 0x00, 0x00},
		{0x81, 0xfa, 0xc0, 0x00, 0x00, 0x00},
		{0x81, 0xfa, 0x80, 0x00, 0x00, 0x00},
	}
	at := 0
	for i, op := range ops {
		if omitBitsCheck && i == len(ops)-1 {
			continue
		}
		copy(code[at:], op)
		at += len(op) + 2
	}
}
