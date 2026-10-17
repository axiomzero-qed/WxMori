//go:build windows

package keycapture

import (
	"context"
	"errors"
	"golang.org/x/sys/windows"
	"sort"
	"strings"
	"unsafe"
)

type process struct {
	pid     uint32
	handle  windows.Handle
	regions []Region
	size    uint64
}

func platformSupported() bool { return unsafe.Sizeof(uintptr(0)) == 8 }
func listProcesses() ([]*process, int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, 0, err
	}
	defer windows.CloseHandle(snapshot)
	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	err = windows.Process32First(snapshot, &entry)
	var out []*process
	denied := 0
	for err == nil {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "Weixin.exe") {
			h, e := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, entry.ProcessID)
			if e != nil {
				if errors.Is(e, windows.ERROR_ACCESS_DENIED) {
					denied++
				}
			} else {
				p := &process{pid: entry.ProcessID, handle: h}
				p.loadRegions()
				out = append(out, p)
			}
		}
		err = windows.Process32Next(snapshot, &entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].size > out[j].size })
	return out, denied, nil
}
func (p *process) close() { _ = windows.CloseHandle(p.handle) }
func (p *process) Read(address uintptr, buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}
	var n uintptr
	err := windows.ReadProcessMemory(p.handle, address, &buffer[0], uintptr(len(buffer)), &n)
	return int(n), err
}
func (p *process) Contains(address uintptr, size int) bool {
	i := sort.Search(len(p.regions), func(i int) bool { return p.regions[i].Base+p.regions[i].Size > address })
	if i == len(p.regions) {
		return false
	}
	r := p.regions[i]
	return address >= r.Base && uintptr(size) <= r.Base+r.Size-address
}
func (p *process) loadRegions() {
	const (
		memCommit  = 0x1000
		memPrivate = 0x20000
		pageGuard  = 0x100
	)
	for address := uintptr(0); ; {
		var info windows.MemoryBasicInformation
		if err := windows.VirtualQueryEx(p.handle, address, &info, unsafe.Sizeof(info)); err != nil {
			break
		}
		next := info.BaseAddress + info.RegionSize
		if next <= address {
			break
		}
		protect := info.Protect & 0xff
		if info.State == memCommit && info.Type == memPrivate && info.Protect&pageGuard == 0 && (protect == 2 || protect == 4 || protect == 8 || protect == 0x20 || protect == 0x40 || protect == 0x80) {
			p.regions = append(p.regions, Region{info.BaseAddress, info.RegionSize})
			p.size += uint64(info.RegionSize)
		}
		address = next
	}
}
func (p *process) scan(ctx context.Context, probe *Probe, progress func(uint64)) error {
	const chunk = 2 << 20
	buffer := make([]byte, chunk+240)
	defer clear(buffer)
	for _, region := range p.regions {
		for offset := uintptr(0); offset < region.Size; {
			if err := ctx.Err(); err != nil {
				return err
			}
			size := min(uintptr(len(buffer)), region.Size-offset)
			n, _ := p.Read(region.Base+offset, buffer[:size])
			if n > 0 {
				probe.Observe(p, region.Base+offset, buffer[:n])
				progress(uint64(n))
			}
			if probe.Complete() {
				return nil
			}
			offset += min(uintptr(chunk), region.Size-offset)
		}
	}
	return nil
}
