//go:build windows && amd64

package windebug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestDebugSyntheticHelper(t *testing.T) {
	if os.Getenv("WXMORI_DEBUG_SYNTHETIC_HELPER") != "1" {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var key [32]byte
	for i := range key {
		key[i] = byte(71 + i)
	}
	cb := syscall.NewCallback(func(_, _, _ uintptr) uintptr { return 0 })
	fmt.Printf("%x\n", cb)
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		switch input.Text() {
		case "fire":
			syscall.SyscallN(cb, uintptr(unsafe.Pointer(&key[0])), 256, 0)
			runtime.KeepAlive(&key)
			fmt.Println("fired")
		case "quit":
			return
		}
	}
}

func TestWatchCapturesOnlySyntheticChildAndDetaches(t *testing.T) {
	// Reuse one debugger OS thread and keep unrelated scheduler workers out of
	// this process-wide handle measurement. Watch still pins itself in normal use.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	procs := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(procs)
	if unsafe.Sizeof(threadContext{}) != 1232 || unsafe.Offsetof(threadContext{}.Rip) != 248 || unsafe.Offsetof(threadContext{}.Rcx) != 128 || unsafe.Sizeof(debugEvent{}) != 176 {
		t.Fatal("Windows SDK layout mismatch")
	}
	for i := 0; i < 16; i++ {
		c, b := alignedContext()
		if uintptr(unsafe.Pointer(c))&15 != 0 {
			t.Fatal("context is not aligned")
		}
		runtime.KeepAlive(b)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestDebugSyntheticHelper$")
	cmd.Env = append(os.Environ(), "WXMORI_DEBUG_SYNTHETIC_HELPER=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { fmt.Fprintln(in, "quit"); in.Close(); cmd.Wait() }()
	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	address, err := strconv.ParseUint(strings.TrimSpace(line), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	handleCount := func() uint32 {
		var n uint32
		// Convert the pointer directly in LazyProc.Call so the compiler keeps
		// the destination alive and immovable for the Windows call.
		r, _, err := kernel.NewProc("GetProcessHandleCount").Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n)))
		if r == 0 {
			t.Fatal(err)
		}
		return n
	}
	before := handleCount()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var found [32]byte
	ready := false
	err = Watch(ctx, uint32(cmd.Process.Pid), []uintptr{uintptr(address)}, func() { ready = true; fmt.Fprintln(in, "fire") }, func(key [32]byte) { found = key; cancel() }, nil)
	if !errors.Is(err, context.Canceled) || !ready {
		t.Fatalf("watch failed to detach: %v", err)
	}
	t.Logf("first attachment includes Go scheduler warm-up: handles %d -> %d", before, handleCount())
	for i, v := range found {
		if v != byte(71+i) {
			t.Fatal("native argument capture failed")
		}
	}
	line, err = r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "fired" {
		t.Fatalf("child did not resume: %q %v", line, err)
	}
	// Execute the same address after detach. A leftover breakpoint would crash
	// this child, proving cleanup rather than just suppressing debugger output.
	fmt.Fprintln(in, "fire")
	line, err = r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "fired" {
		t.Fatalf("child failed after detach: %q %v", line, err)
	}
	baseline := handleCount()
	for attempt := 0; attempt < 12; attempt++ {
		threadsBefore, _ := runtime.ThreadCreateProfile(nil)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		err := Watch(ctx, uint32(cmd.Process.Pid), []uintptr{uintptr(address)}, func() { fmt.Fprintln(in, "fire") }, func([32]byte) { cancel() }, nil)
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("repeat attach/detach failed: %v", err)
		}
		line, err := r.ReadString('\n')
		if err != nil || strings.TrimSpace(line) != "fired" {
			t.Fatal("child failed after repeated detach")
		}
		after := handleCount()
		threadsAfter, _ := runtime.ThreadCreateProfile(nil)
		if after > baseline {
			t.Fatalf("repeat debugger retained handles: baseline=%d after=%d runtime_threads=%d->%d", baseline, after, threadsBefore, threadsAfter)
		}
	}
}
