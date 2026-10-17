package service

import (
	"context"
	"github.com/axiomzero-qed/WxMori/internal/fault"
	"time"
)

type CaptureArgs struct {
	Action         string `json:"action" jsonschema:"start, status or stop / start 开始、status 查看、stop 停止"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"For start: default 180, maximum 600 / 开始采集的时限，默认 180 秒、最多 600 秒"`
	Mode           string `json:"mode,omitempty" jsonschema:"auto (default) temporarily watches key setup on recognized Windows x64 builds; read_only uses polling only / auto 默认即时捕获，read_only 仅读内存轮询"`
}

func (a *App) Capture(ctx context.Context, arg CaptureArgs) (any, error) {
	if a.ConfigErr != nil {
		return nil, a.ConfigErr
	}
	switch arg.Action {
	case "start":
		seconds := arg.TimeoutSeconds
		if seconds == 0 {
			seconds = 180
		}
		if seconds < 1 || seconds > 600 {
			return nil, fault.New("INVALID_CAPTURE_TIMEOUT", "采集时限无效 / Invalid capture timeout", "Use an integer from 1 to 600 seconds.")
		}
		mode := arg.Mode
		if mode == "" {
			mode = "auto"
		}
		return a.capture.StartWithMode(time.Duration(seconds)*time.Second, mode)
	case "status":
		return a.capture.Status(), nil
	case "stop":
		return a.capture.Stop(), nil
	default:
		return nil, fault.New("INVALID_CAPTURE_ACTION", "采集操作无效 / Invalid capture action", "Use action=start, action=status or action=stop. After start, ask the user to log in locally and poll status. Never ask for a password or key.")
	}
}
