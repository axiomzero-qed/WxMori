//go:build !windows

package keycapture

import "context"

type process struct{ pid uint32 }

func platformSupported() bool                                     { return false }
func listProcesses() ([]*process, int, error)                     { return nil, 0, nil }
func (*process) close()                                           {}
func (*process) scan(context.Context, *Probe, func(uint64)) error { return nil }
