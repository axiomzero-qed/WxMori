package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/axiomzero-qed/WxMori/internal/fault"
	"github.com/axiomzero-qed/WxMori/internal/keycapture"
	"github.com/axiomzero-qed/WxMori/internal/service"
)

type captureFake struct {
	statuses    []keycapture.Status
	start       keycapture.Status
	stop        keycapture.Status
	statusCalls int
	actions     []string
}

func (f *captureFake) Capture(_ context.Context, args service.CaptureArgs) (any, error) {
	f.actions = append(f.actions, args.Action)
	switch args.Action {
	case "start":
		return f.start, nil
	case "status":
		at := f.statusCalls
		f.statusCalls++
		if at >= len(f.statuses) {
			at = len(f.statuses) - 1
		}
		return f.statuses[at], nil
	case "stop":
		return f.stop, nil
	default:
		return nil, errors.New("unexpected capture action")
	}
}

type fakeCaptureClock struct{ at time.Time }

func (c *fakeCaptureClock) now() time.Time { return c.at }
func (c *fakeCaptureClock) wait(_ context.Context, d time.Duration) error {
	c.at = c.at.Add(d)
	return nil
}

func TestRunCaptureHumanProgressAndCautiousLoginReminder(t *testing.T) {
	hint := "A synthetic long status hint that should only print on the initial state."
	start := keycapture.Status{Running: true, State: "watching_login", RequiredDatabases: 2, Passes: 1, Hint: hint}
	progress := make([]keycapture.Status, 30)
	for i := range progress {
		progress[i] = start
		progress[i].Passes = i + 2
	}
	complete := keycapture.Status{State: "completed", RequiredDatabases: 2, VerifiedDatabases: 2, Hint: "Synthetic capture completed."}
	progress = append(progress, complete)
	app := &captureFake{start: start, statuses: progress}
	clock := &fakeCaptureClock{at: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)}
	var stdout, stderr bytes.Buffer
	err := runCaptureWith(context.Background(), app, &stdout, &stderr, 30, "auto", "human", clock.now, clock.wait)
	if err != nil {
		t.Fatal(err)
	}
	text := stderr.String()
	if strings.Count(text, hint) != 1 {
		t.Fatalf("long hint printed %d times", strings.Count(text, hint))
	}
	if strings.Count(text, "尚未找到匹配的数据库密钥") != 1 || strings.Contains(text, "仍在等待登录事件") {
		t.Fatalf("cautious login reminder missing or repeated: %q", text)
	}
	if strings.Count(text, "采集剩余时间约 30 秒或更少") != 1 || !strings.Contains(text, "--timeout") {
		t.Fatalf("timeout reminder missing or repeated: %q", text)
	}
	if !strings.Contains(text, "completed") || !strings.Contains(text, "Verified") && !strings.Contains(text, "数据库 / DB") {
		t.Fatalf("human progress or completion missing: %q", text)
	}
	var final keycapture.Status
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &final); err != nil || final.State != "completed" {
		t.Fatalf("human mode did not emit a machine-readable final status: %q (%v)", stdout.String(), err)
	}
	if app.statusCalls != 31 {
		t.Fatalf("polled status %d times, want all 31 updates", app.statusCalls)
	}
}

func TestRunCaptureJSONEmissionCadenceAndStateChanges(t *testing.T) {
	start := keycapture.Status{Running: true, State: "waiting_for_wechat"}
	updates := []keycapture.Status{
		{Running: true, State: "waiting_for_wechat", Passes: 1},
		{Running: true, State: "scanning", ReadyForLogin: true, Passes: 2},
		{Running: true, State: "scanning", ReadyForLogin: true, Passes: 3},
		{Running: true, State: "scanning", ReadyForLogin: true, Passes: 4},
		{Running: true, State: "scanning", ReadyForLogin: true, Passes: 5},
		{Running: true, State: "scanning", ReadyForLogin: true, Passes: 6},
		{Running: true, State: "scanning", ReadyForLogin: true, Passes: 7},
		{State: "completed", ReadyForLogin: false, Passes: 7},
	}
	app := &captureFake{start: start, statuses: updates}
	clock := &fakeCaptureClock{at: time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)}
	var stdout, stderr bytes.Buffer
	err := runCaptureWith(context.Background(), app, &stdout, &stderr, 180, "auto", "json", clock.now, clock.wait)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d JSON status updates, want start, readiness/state, 5-second counter, and completion: %s", len(lines), stdout.String())
	}
	var states []keycapture.Status
	for _, line := range lines {
		var status keycapture.Status
		if err := json.Unmarshal([]byte(line), &status); err != nil {
			t.Fatal(err)
		}
		states = append(states, status)
	}
	if states[0].State != "waiting_for_wechat" || states[1].State != "scanning" || !states[1].ReadyForLogin || states[2].Passes != 7 || states[3].State != "completed" {
		t.Fatalf("unexpected JSON emission sequence: %+v", states)
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON progress without short timeout wrote diagnostics: %q", stderr.String())
	}
}

func TestRunCaptureCancellationStopsAndPrintsStoppedStatus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app := &captureFake{
		start: keycapture.Status{Running: true, State: "scanning"},
		stop:  keycapture.Status{State: "stopped", Hint: "Synthetic stopped."},
	}
	clock := &fakeCaptureClock{at: time.Now()}
	var stdout, stderr bytes.Buffer
	wait := func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	err := runCaptureWith(ctx, app, &stdout, &stderr, 180, "auto", "human", clock.now, wait)
	if err != nil {
		t.Fatal(err)
	}
	if len(app.actions) != 2 || app.actions[0] != "start" || app.actions[1] != "stop" {
		t.Fatalf("unexpected actions after cancel: %v", app.actions)
	}
	if !strings.Contains(stderr.String(), "stopped") || !strings.Contains(stderr.String(), "Synthetic stopped.") {
		t.Fatalf("stopped status was not printed: %q", stderr.String())
	}
}

func TestRunCapturePreservesFailureAndMarksIncomplete(t *testing.T) {
	failure := fault.New("CAPTURE_SOURCE_UNREADABLE", "synthetic failure", "synthetic hint")
	failed := keycapture.Status{State: "failed", Error: failure, Hint: failure.Hint}
	app := &captureFake{start: keycapture.Status{Running: true, State: "scanning"}, statuses: []keycapture.Status{failed}}
	clock := &fakeCaptureClock{at: time.Now()}
	_, stderr := new(bytes.Buffer), new(bytes.Buffer)
	got := runCaptureWith(context.Background(), app, &bytes.Buffer{}, stderr, 180, "auto", "human", clock.now, clock.wait)
	if got != failure {
		t.Fatalf("failure was replaced: got %v, want original error %v", got, failure)
	}
	if !strings.Contains(stderr.String(), failure.Code) {
		t.Fatalf("failure output omitted actual error code: %q", stderr.String())
	}

	mismatch := fault.New("KEY_ACCOUNT_MISMATCH", "synthetic account mismatch", "check the observed account")
	app = &captureFake{
		start: keycapture.Status{Running: true, State: "scanning"},
		statuses: []keycapture.Status{{
			State: "account_mismatch", Error: mismatch, Hint: mismatch.Hint, KeySetupEvents: 3,
			ObservedAccounts: []keycapture.AccountMatch{{DirectoryID: "account-hash", DataDir: `C:\synthetic\other-account`, MatchedDatabases: 2}},
		}},
	}
	var mismatchOut, mismatchErr bytes.Buffer
	got = runCaptureWith(context.Background(), app, &mismatchOut, &mismatchErr, 180, "auto", "human", clock.now, clock.wait)
	if got != mismatch {
		t.Fatalf("account mismatch error was replaced: got %v, want %v", got, mismatch)
	}
	for _, want := range []string{mismatch.Code, "account-hash", `C:\synthetic\other-account`, "密钥设置事件 / key setup events: 3"} {
		if !strings.Contains(mismatchErr.String(), want) {
			t.Fatalf("account diagnostics omitted %q: %q", want, mismatchErr.String())
		}
	}

	app = &captureFake{start: keycapture.Status{State: "timeout", Hint: "not completed"}}
	got = runCaptureWith(context.Background(), app, &bytes.Buffer{}, &bytes.Buffer{}, 180, "auto", "human", clock.now, clock.wait)
	var coded *fault.Error
	if !errors.As(got, &coded) || coded.Code != "CAPTURE_INCOMPLETE" {
		t.Fatalf("incomplete capture error = %v", got)
	}
}

func TestKeySetupEventsAreProgressCounters(t *testing.T) {
	start := keycapture.Status{Running: true, State: "scanning", KeySetupEvents: 1}
	finished := keycapture.Status{State: "timeout", KeySetupEvents: 2}
	app := &captureFake{start: start, statuses: []keycapture.Status{finished}}
	clock := &fakeCaptureClock{at: time.Now()}
	var stderr bytes.Buffer
	err := runCaptureWith(context.Background(), app, &bytes.Buffer{}, &stderr, 180, "auto", "human", clock.now, clock.wait)
	var coded *fault.Error
	if !errors.As(err, &coded) || coded.Code != "CAPTURE_INCOMPLETE" {
		t.Fatalf("unexpected terminal error: %v", err)
	}
	if !strings.Contains(stderr.String(), "key setup events: 1") || !strings.Contains(stderr.String(), "key setup events: 2") {
		t.Fatalf("key setup event counter did not update: %q", stderr.String())
	}
}

func TestTimeoutReminderThresholdAndInvalidProgressError(t *testing.T) {
	if timeoutReminderDue(300, 269*time.Second) || !timeoutReminderDue(300, 270*time.Second) {
		t.Fatal("300-second capture reminder must begin with 30 seconds remaining")
	}
	if !timeoutReminderDue(30, 0) || timeoutReminderDue(0, time.Hour) {
		t.Fatal("short and default timeout thresholds are incorrect")
	}
	app := &captureFake{}
	err := runCaptureWith(context.Background(), app, &bytes.Buffer{}, &bytes.Buffer{}, 180, "auto", "xml", time.Now, waitCapture)
	var coded *fault.Error
	if !errors.As(err, &coded) || coded.Code != "INVALID_PROGRESS" {
		t.Fatalf("invalid progress mode error = %v, want INVALID_PROGRESS", err)
	}
}
