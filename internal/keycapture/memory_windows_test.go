//go:build windows

package keycapture

import (
	"bufio"
	"context"
	"fmt"
	"github.com/axiomzero-qed/WxMori/internal/aesschedule"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"unsafe"
)

func TestSyntheticMemoryHelper(t *testing.T) {
	if os.Getenv("WXMORI_SYNTHETIC_MEMORY_HELPER") != "1" {
		return
	}
	_, key := fixtureTarget()
	schedule := aesschedule.DecryptionSchedule(key)
	fmt.Printf("%x\n", uintptr(unsafe.Pointer(&schedule[0])))
	bufio.NewReader(os.Stdin).ReadString('\n')
	runtime.KeepAlive(schedule)
}

func TestReadOnlyChildMemoryCapture(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(exe, "-test.run=^TestSyntheticMemoryHelper$")
	cmd.Env = append(os.Environ(), "WXMORI_SYNTHETIC_MEMORY_HELPER=1")
	out, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	in, e := cmd.StdinPipe()
	if e != nil {
		t.Fatal(e)
	}
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() { fmt.Fprintln(in, "done"); in.Close(); cmd.Wait() }()
	line, e := bufio.NewReader(out).ReadString('\n')
	if e != nil {
		t.Fatal(e)
	}
	address, e := strconv.ParseUint(line[:len(line)-1], 16, 64)
	if e != nil {
		t.Fatal(e)
	}
	h, e := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(cmd.Process.Pid))
	if e != nil {
		t.Fatal(e)
	}
	p := &process{pid: uint32(cmd.Process.Pid), handle: h}
	defer p.close()
	p.loadRegions()
	if !p.Contains(uintptr(address), 240) {
		t.Fatal("synthetic schedule region was not readable")
	}
	var buffer [240]byte
	if n, e := p.Read(uintptr(address), buffer[:]); e != nil || n != len(buffer) {
		t.Fatalf("read failed %d %v", n, e)
	}
	target, key := fixtureTarget()
	probe := NewProbe([]Target{target})
	defer probe.Clear()
	probe.Observe(p, uintptr(address), buffer[:])
	if !probe.Complete() || probe.Keys[target.Salt] != key {
		t.Fatal("child schedule did not authenticate")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = p.scan(ctx, probe, func(uint64) {}); e != context.Canceled {
		t.Fatalf("scan ignored cancellation: %v", e)
	}
}
