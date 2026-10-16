package fault

import "fmt"

// Error carries a stable recovery code without including secrets or message bodies.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Hint       string `json:"hint"`
	Candidates any    `json:"candidates,omitempty"`
}

func (e *Error) Error() string              { return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Hint) }
func New(code, message, hint string) *Error { return &Error{Code: code, Message: message, Hint: hint} }
