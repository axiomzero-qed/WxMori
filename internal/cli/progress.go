package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keycapture"
	"github.com/axiomzero-qed/WxMori/internal/service"
)

type captureClient interface {
	Capture(context.Context, service.CaptureArgs) (any, error)
}

// runCapture starts key capture, displays progress, and polls at most once per second.
// progress is "human" (the default) or "json"; human progress goes to errOut.
func runCapture(ctx context.Context, app captureClient, out, errOut io.Writer, timeout int, mode, progress string) error {
	return runCaptureWith(ctx, app, out, errOut, timeout, mode, progress, time.Now, waitCapture)
}

func runCaptureWith(ctx context.Context, app captureClient, out, errOut io.Writer, timeout int, mode, progress string, now func() time.Time, wait func(context.Context, time.Duration) error) error {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	if progress == "" {
		progress = "human"
	}
	if progress != "human" && progress != "json" {
		return fault.New("INVALID_PROGRESS", "采集进度模式无效 / Invalid capture progress mode", "Use progress=human or progress=json.")
	}
	started := now()
	v, err := app.Capture(ctx, service.CaptureArgs{Action: "start", TimeoutSeconds: timeout, Mode: mode})
	if err != nil {
		return err
	}
	status, err := captureStatus(v)
	if err != nil {
		return err
	}
	printer := newCaptureProgress(out, errOut, progress, now, now())
	printer.print(status, true)
	if printer.writeErr != nil {
		return printer.writeErr
	}
	remindedTimeout := false
	remindTimeout := func(s keycapture.Status) {
		if !remindedTimeout && s.Running && timeoutReminderDue(timeout, now().Sub(started)) {
			printer.writeErr = writeCaptureLine(errOut, "采集剩余时间约 30 秒或更少；如需延长，请使用 --timeout（最多 600 秒）。 / Capture has about 30 seconds or less remaining; use --timeout to extend (up to 600 seconds).", printer.writeErr)
			remindedTimeout = true
		}
	}
	remindTimeout(status)
	if statusErr := captureFinished(status); statusErr != nil || !status.Running {
		printer.finish(status)
		if statusErr != nil {
			return statusErr
		}
		return printer.writeErr
	}
	if printer.writeErr != nil {
		return printer.writeErr
	}

	remindedLogin := false
	for {
		if err := wait(ctx, time.Second); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			stopped, stopErr := app.Capture(context.Background(), service.CaptureArgs{Action: "stop"})
			if stopErr != nil {
				return stopErr
			}
			stopStatus, parseErr := captureStatus(stopped)
			if parseErr != nil {
				return parseErr
			}
			printer.print(stopStatus, false)
			printer.finish(stopStatus)
			return printer.writeErr
		}
		v, err = app.Capture(ctx, service.CaptureArgs{Action: "status"})
		if err != nil {
			return err
		}
		status, err = captureStatus(v)
		if err != nil {
			return err
		}
		printer.print(status, false)
		remindTimeout(status)
		if !remindedLogin && status.State == "watching_login" && status.VerifiedDatabases == 0 && now().Sub(started) >= 30*time.Second {
			printer.writeErr = writeCaptureLine(errOut, "尚未找到匹配的数据库密钥；请等 ready_for_login=true 后再本机重新登录，并确认所选数据目录属于目标账号。 / No matching database key has been found yet; re-login locally only after ready_for_login=true, and check that the selected data directory belongs to the intended account.", printer.writeErr)
			remindedLogin = true
		}
		if statusErr := captureFinished(status); statusErr != nil || !status.Running {
			printer.finish(status)
			if statusErr != nil {
				return statusErr
			}
			return printer.writeErr
		}
		if printer.writeErr != nil {
			return printer.writeErr
		}
	}
}

func waitCapture(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func timeoutReminderDue(timeout int, elapsed time.Duration) bool {
	return timeout > 0 && elapsed >= time.Duration(maxInt(timeout-30, 0))*time.Second
}

func captureStatus(v any) (keycapture.Status, error) {
	switch value := v.(type) {
	case keycapture.Status:
		return value, nil
	case *keycapture.Status:
		if value != nil {
			return *value, nil
		}
	}
	return keycapture.Status{}, fmt.Errorf("capture returned unexpected status %T", v)
}

func captureFinished(status keycapture.Status) error {
	if !status.Running && status.Error != nil {
		return status.Error
	}
	if status.Running || status.State == "completed" {
		return nil
	}
	return fault.New("CAPTURE_INCOMPLETE", "采集未完成 / Capture incomplete", status.Hint)
}

type captureProgress struct {
	out, errOut io.Writer
	mode        string
	now         func() time.Time
	last        keycapture.Status
	hasLast     bool
	lastPrinted time.Time
	humanLine   bool
	lastJSON    keycapture.Status
	hasJSON     bool
	writeErr    error
}

func newCaptureProgress(out, errOut io.Writer, mode string, now func() time.Time, at time.Time) *captureProgress {
	return &captureProgress{out: out, errOut: errOut, mode: mode, now: now, lastPrinted: at}
}

func (p *captureProgress) print(status keycapture.Status, initial bool) {
	changed := !p.hasLast || captureStateChanged(p.last, status)
	countersChanged := !p.hasLast || captureCountersChanged(p.last, status)
	if p.mode == "json" {
		now := p.now()
		if changed || initial || now.Sub(p.lastPrinted) >= 5*time.Second {
			p.emitJSON(status)
			p.lastPrinted = now
		}
	} else if changed {
		if p.humanLine {
			p.writeErr = writeCaptureLine(p.errOut, "", p.writeErr)
		}
		p.writeErr = writeCaptureLine(p.errOut, captureStateLine(status), p.writeErr)
		if status.Hint != "" && (!p.hasLast || status.Hint != p.last.Hint) {
			p.writeErr = writeCaptureLine(p.errOut, status.Hint, p.writeErr)
		}
		p.humanLine = false
	} else if countersChanged {
		p.writeErr = writeCapture(p.errOut, "\r"+captureCounterLine(status), p.writeErr)
		p.humanLine = true
	}
	p.last, p.hasLast = status, true
}

func (p *captureProgress) finish(status keycapture.Status) {
	if p.mode == "human" {
		p.emitJSON(status)
		return
	}
	if !p.hasJSON || !reflect.DeepEqual(p.lastJSON, status) {
		p.emitJSON(status)
	}
}

func (p *captureProgress) emitJSON(status keycapture.Status) {
	if p.writeErr == nil {
		p.writeErr = json.NewEncoder(p.out).Encode(status)
	}
	p.lastJSON, p.hasJSON = status, true
}

func writeCaptureLine(w io.Writer, text string, first error) error {
	if first != nil {
		return first
	}
	_, err := fmt.Fprintln(w, text)
	return err
}

func writeCapture(w io.Writer, text string, first error) error {
	if first != nil {
		return first
	}
	_, err := io.WriteString(w, text)
	return err
}

func captureStateChanged(a, b keycapture.Status) bool {
	codeA, codeB := "", ""
	if a.Error != nil {
		codeA = a.Error.Code
	}
	if b.Error != nil {
		codeB = b.Error.Code
	}
	return a.State != b.State || a.ReadyForLogin != b.ReadyForLogin || a.Running != b.Running || codeA != codeB || a.Hint != b.Hint || !reflect.DeepEqual(a.ObservedAccounts, b.ObservedAccounts)
}

func captureCountersChanged(a, b keycapture.Status) bool {
	return a.RequiredDatabases != b.RequiredDatabases || a.VerifiedDatabases != b.VerifiedDatabases || a.ActiveProcesses != b.ActiveProcesses || a.AccessDenied != b.AccessDenied || a.Passes != b.Passes || a.Candidates != b.Candidates || a.KeySetupEvents != b.KeySetupEvents || a.BytesRead != b.BytesRead
}

func captureStateLine(s keycapture.Status) string {
	state := strings.TrimSpace(s.State)
	if state == "" {
		state = "unknown"
	}
	line := "密钥采集 / Key capture: " + state + "\n" + captureCounterLine(s)
	if s.Error != nil {
		line += "\n" + s.Error.Code + ": " + s.Error.Message
	}
	for _, account := range s.ObservedAccounts {
		line += fmt.Sprintf("\n其他账号匹配 / Other account match: %s (%d databases) at %s", account.DirectoryID, account.MatchedDatabases, account.DataDir)
	}
	return line
}

func captureCounterLine(s keycapture.Status) string {
	ready := "否 / no"
	if s.ReadyForLogin {
		ready = "是 / yes"
	}
	return fmt.Sprintf("登录准备 / Ready for login: %s | 数据库 / DB: %d/%d | 微信进程 / Weixin: %d (拒绝访问 / denied %d) | 扫描 / passes: %d | 候选 / candidates: %d | 密钥设置事件 / key setup events: %d | 已读 / bytes: %d", ready, s.VerifiedDatabases, s.RequiredDatabases, s.ActiveProcesses, s.AccessDenied, s.Passes, s.Candidates, s.KeySetupEvents, s.BytesRead)
}
