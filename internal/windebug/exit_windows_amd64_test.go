//go:build windows && amd64

package windebug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWatchReturnsProcessExitedForSyntheticChild(t *testing.T) {
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
	waited := false
	defer func() {
		if !waited {
			_, _ = fmt.Fprintln(in, "quit")
			_ = in.Close()
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("synthetic helper did not report its callback: %v", err)
	}
	address, err := strconv.ParseUint(strings.TrimSpace(line), 16, 64)
	if err != nil {
		t.Fatalf("invalid synthetic callback address %q: %v", line, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ready := false
	var quitErr error
	candidate := false
	watchErr := Watch(ctx, uint32(cmd.Process.Pid), []uintptr{uintptr(address)}, func() {
		ready = true
		quitErr = func() error { _, e := fmt.Fprintln(in, "quit"); return e }()
	}, func([32]byte) { candidate = true }, nil)
	if !errors.Is(watchErr, ErrProcessExited) {
		t.Fatalf("Watch returned %v, want ErrProcessExited", watchErr)
	}
	if !ready || quitErr != nil {
		t.Fatalf("synthetic child shutdown failed: ready=%v write error=%v", ready, quitErr)
	}
	if candidate {
		t.Fatal("unexpected key candidate from helper that never fired its callback")
	}
	if err = in.Close(); err != nil {
		t.Fatal(err)
	}
	if err = cmd.Wait(); err != nil {
		t.Fatalf("synthetic child did not exit successfully: %v", err)
	}
	waited = true
}
