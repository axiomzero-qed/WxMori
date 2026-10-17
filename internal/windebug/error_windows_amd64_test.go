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

func TestWatchErrorRestoresBreakpointsBeforeReturning(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestDebugSyntheticHelper$")
	cmd.Env = append(os.Environ(), "WXMORI_DEBUG_SYNTHETIC_HELPER=1")
	output, err := cmd.StdoutPipe()
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
	r := bufio.NewReader(output)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	address, err := strconv.ParseUint(strings.TrimSpace(line), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	want := errors.New("synthetic account mismatch")
	armed := false
	err = Watch(ctx, uint32(cmd.Process.Pid), []uintptr{uintptr(address)}, func() { armed = true }, func([32]byte) { t.Error("unexpected candidate") }, func() error {
		if armed {
			return want
		}
		return nil
	})
	if !errors.Is(err, want) {
		t.Fatalf("watch lost operation error: %v", err)
	}
	if _, err = fmt.Fprintln(in, "fire"); err != nil {
		t.Fatal(err)
	}
	line, err = r.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "fired" {
		t.Fatal("debug register restoration failed after an operation error")
	}
}
