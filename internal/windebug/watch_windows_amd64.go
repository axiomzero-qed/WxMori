//go:build windows && amd64

// Package windebug provides a temporary, explicitly detached login watcher.
// It uses processor execution breakpoints; no code patch or DLL is injected.
package windebug

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ErrProcessExited = errors.New("watched process exited")
	kernel           = windows.NewLazySystemDLL("kernel32.dll")
	attach           = kernel.NewProc("DebugActiveProcess")
	detach           = kernel.NewProc("DebugActiveProcessStop")
	killOnExit       = kernel.NewProc("DebugSetProcessKillOnExit")
	waitEvent        = kernel.NewProc("WaitForDebugEventEx")
	continueEvent    = kernel.NewProc("ContinueDebugEvent")
	getContext       = kernel.NewProc("GetThreadContext")
	setContext       = kernel.NewProc("SetThreadContext")
	suspendThread    = kernel.NewProc("SuspendThread")
)

// SDK x64 CONTEXT, including its full floating/vector tail. Windows requires
// 16-byte alignment even though Go's largest native alignment is eight bytes.
// https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-context
type threadContext struct {
	Home                                      [6]uint64
	Flags, MxCsr                              uint32
	Segments                                  [6]uint16
	EFlags                                    uint32
	Dr0, Dr1, Dr2, Dr3, Dr6, Dr7              uint64
	Rax, Rcx, Rdx, Rbx, Rsp, Rbp, Rsi, Rdi    uint64
	R8, R9, R10, R11, R12, R13, R14, R15, Rip uint64
	FloatVector                               [976]byte
}

func alignedContext() (*threadContext, []byte) {
	buf := make([]byte, 1232+15)
	padding := int((16 - uintptr(unsafe.Pointer(&buf[0]))&15) & 15)
	return (*threadContext)(unsafe.Pointer(&buf[padding])), buf
}

const contextFlags = 0x100013 // AMD64 CONTROL | INTEGER | DEBUG_REGISTERS
type savedThread struct {
	handle windows.Handle
	dr     [6]uint64
}
type debugEvent struct {
	Code, PID, TID, Padding uint32
	Data                    [160]byte
}

func call(proc *windows.LazyProc, args ...uintptr) error {
	r, _, err := proc.Call(args...)
	if r == 0 {
		return fmt.Errorf("%s: %w", proc.Name, err)
	}
	return nil
}

func getThread(handle windows.Handle) (*threadContext, []byte, error) {
	c, backing := alignedContext()
	c.Flags = contextFlags
	err := call(getContext, uintptr(handle), uintptr(unsafe.Pointer(c)))
	runtime.KeepAlive(backing)
	return c, backing, err
}
func putThread(handle windows.Handle, c *threadContext, backing []byte) error {
	err := call(setContext, uintptr(handle), uintptr(unsafe.Pointer(c)))
	runtime.KeepAlive(backing)
	return err
}

// Watch runs on one pinned OS thread. ready fires only after the debugger's
// initial breakpoint has been handled. candidate runs after resuming the target.
// tick may return an error to stop; cleanup restores owned registers and detaches.
func Watch(ctx context.Context, pid uint32, addresses []uintptr, ready func(), candidate func([32]byte), tick func() error) (retErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(addresses) == 0 || len(addresses) > 4 {
		return errors.New("invalid breakpoint count")
	}
	for _, address := range addresses {
		if address < 0x10000 {
			return errors.New("invalid breakpoint address")
		}
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := call(attach, uintptr(pid)); err != nil {
		return err
	}
	// A debugger must never terminate the user's application on normal exit.
	if err := call(killOnExit, 0); err != nil {
		_ = call(detach, uintptr(pid))
		return err
	}
	threads := map[uint32]savedThread{}
	managedThreads := map[uint32]windows.Handle{}
	var managedProcess windows.Handle
	var process windows.Handle
	var pending *debugEvent
	var pendingStatus uintptr = 0x10002
	ack := func(event *debugEvent, status uintptr) error {
		err := call(continueEvent, uintptr(event.PID), uintptr(event.TID), status)
		if err == nil {
			// Windows closes these event handles on continued exit events,
			// but does not close still-live handles when we explicitly detach.
			if event.Code == 4 {
				delete(managedThreads, event.TID)
			}
			if event.Code == 5 {
				clear(managedThreads)
				managedProcess = 0
			}
		}
		return err
	}
	defer func() {
		if pending != nil {
			_ = ack(pending, pendingStatus)
		}
		var cleanupErr error
		for _, saved := range threads {
			r, _, e := suspendThread.Call(uintptr(saved.handle))
			if r == 0xffffffff {
				continue
			} // an exiting thread needs no restoration
			c, backing, err := getThread(saved.handle)
			if err == nil {
				c.Dr0, c.Dr1, c.Dr2, c.Dr3, c.Dr6, c.Dr7 = saved.dr[0], saved.dr[1], saved.dr[2], saved.dr[3], saved.dr[4], saved.dr[5]
				err = putThread(saved.handle, c, backing)
			}
			_, resumeErr := windows.ResumeThread(saved.handle)
			if err != nil {
				cleanupErr = err
			}
			if resumeErr != nil {
				cleanupErr = resumeErr
			}
			_ = e
		}
		if err := call(detach, uintptr(pid)); err != nil {
			// An exited process is already detached.
			var exit uint32
			if process == 0 || windows.GetExitCodeProcess(process, &exit) != nil || exit == 259 {
				cleanupErr = err
			}
		}
		for _, saved := range threads {
			windows.CloseHandle(saved.handle)
		}
		for _, h := range managedThreads {
			windows.CloseHandle(h)
		}
		if managedProcess != 0 {
			windows.CloseHandle(managedProcess)
		}
		if process != 0 {
			windows.CloseHandle(process)
		}
		if cleanupErr != nil {
			retErr = fmt.Errorf("debugger cleanup: %w", cleanupErr)
		}
	}()
	var err error
	process, err = windows.OpenProcess(windows.PROCESS_VM_READ|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return err
	}
	arm := func(tid uint32) error {
		h, err := windows.OpenThread(windows.THREAD_GET_CONTEXT|windows.THREAD_SET_CONTEXT|windows.THREAD_SUSPEND_RESUME, false, tid)
		if err != nil {
			return err
		}
		c, backing, err := getThread(h)
		if err != nil {
			windows.CloseHandle(h)
			return err
		}
		if c.Dr7&0xff != 0 {
			windows.CloseHandle(h)
			return errors.New("existing hardware breakpoints; leave the other debugger alone")
		}
		saved := savedThread{h, [6]uint64{c.Dr0, c.Dr1, c.Dr2, c.Dr3, c.Dr6, c.Dr7}}
		registers := []*uint64{&c.Dr0, &c.Dr1, &c.Dr2, &c.Dr3}
		for i, address := range addresses {
			*registers[i] = uint64(address)
			c.Dr7 &^= uint64(15) << (16 + 4*i)
			c.Dr7 |= uint64(1) << (2 * i)
		}
		if err := putThread(h, c, backing); err != nil {
			windows.CloseHandle(h)
			return err
		}
		threads[tid] = saved
		return nil
	}
	initialBreakpoint := false
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if tick != nil {
			if err := tick(); err != nil {
				return err
			}
		}
		var event debugEvent
		r, _, e := waitEvent.Call(uintptr(unsafe.Pointer(&event)), 100)
		if r == 0 {
			if errors.Is(e, windows.ERROR_SEM_TIMEOUT) || errors.Is(e, windows.WAIT_TIMEOUT) {
				continue
			}
			return fmt.Errorf("wait debug event: %w", e)
		}
		pending, pendingStatus = &event, 0x10002
		var key [32]byte
		hasKey, armed := false, false
		switch event.Code {
		case 2, 3:
			if event.Code == 3 {
				managedProcess = windows.Handle(binary.LittleEndian.Uint64(event.Data[8:]))
				managedThreads[event.TID] = windows.Handle(binary.LittleEndian.Uint64(event.Data[16:]))
				h := windows.Handle(binary.LittleEndian.Uint64(event.Data[:]))
				if h != 0 {
					windows.CloseHandle(h)
				}
			} else {
				managedThreads[event.TID] = windows.Handle(binary.LittleEndian.Uint64(event.Data[:]))
			}
			if err := arm(event.TID); err != nil {
				return err
			}
		case 4:
			if saved, ok := threads[event.TID]; ok {
				windows.CloseHandle(saved.handle)
				delete(threads, event.TID)
			}
		case 5:
			if err := ack(&event, pendingStatus); err != nil {
				return err
			}
			pending = nil
			return ErrProcessExited
		case 6:
			h := windows.Handle(binary.LittleEndian.Uint64(event.Data[:]))
			if h != 0 {
				windows.CloseHandle(h)
			}
		case 1:
			code := binary.LittleEndian.Uint32(event.Data[:])
			pendingStatus = 0x80010001 // preserve exceptions that are not ours
			if code == 0x80000003 && !initialBreakpoint {
				initialBreakpoint, armed, pendingStatus = true, true, 0x10002
			} else if code == 0x80000004 {
				saved, ok := threads[event.TID]
				if !ok {
					return errors.New("breakpoint on untracked thread")
				}
				c, backing, err := getThread(saved.handle)
				if err != nil {
					return err
				}
				ours := false
				for i, address := range addresses {
					if c.Dr6&(1<<i) != 0 && c.Rip == uint64(address) {
						ours = true
					}
				}
				if ours {
					if c.Rdx == 256 && c.Rcx >= 0x10000 {
						var n uintptr
						if err := windows.ReadProcessMemory(process, uintptr(c.Rcx), &key[0], 32, &n); err == nil && n == 32 {
							hasKey = true
						}
					}
					c.Dr6 &^= 15
					c.EFlags |= 1 << 16 // resume past this execution breakpoint once
					if err := putThread(saved.handle, c, backing); err != nil {
						clear(key[:])
						return err
					}
					pendingStatus = 0x10002
				}
			}
		}
		if err := ack(&event, pendingStatus); err != nil {
			clear(key[:])
			return err
		}
		pending = nil
		if armed && ready != nil {
			ready()
		}
		if hasKey {
			candidate(key)
			clear(key[:])
		}
	}
}
