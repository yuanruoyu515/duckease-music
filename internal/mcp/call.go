package mcp

import (
	"sync"
	"sync/atomic"
	"time"
)

// How long a call may take. A client gives up waiting on a call after a
// minute or a few (Codex, five), and a call still running then holds up
// every call after it, as they run one at a time. So a call that looks
// for many songs stops itself after callBudget, keeping what it found
// for the next to go on from; and one that runs on past callLimit, as
// one stuck on the keychain's dialog, is answered with an error, and the
// calls after it are told it is still running rather than waiting on it.
var (
	callBudget = 45 * time.Second
	callLimit  = 100 * time.Second
	// busyWait is how long a call waits for one still running past
	// callLimit before it is told to come back later.
	busyWait = 5 * time.Second
)

// SessionReady, if set, closes once the NetEase session is read off the
// keychain, which asks the listener first after each new build. A call
// waits sessionWait for it, then says what to click.
var (
	SessionReady <-chan struct{}
	sessionWait  = 20 * time.Second
)

// sessionNote is what a call says while the keychain waits on the
// listener.
const sessionNote = "error: DuckEase Music is waiting for macOS to let it read its login from the keychain. Ask the user to look for a keychain dialog about DuckEase Music on their Mac and click Always Allow (始终允许), then call again."

// sessionRead reports whether the session was read, waiting for it a
// while.
func sessionRead() bool {
	if SessionReady == nil {
		return true
	}
	select {
	case <-SessionReady:
		return true
	case <-time.After(sessionWait):
		return false
	}
}

// callCtx is the call running: when it should stop by, whether the
// client cancelled it, and how to tell the client how far it got.
type callCtx struct {
	deadline  time.Time
	cancelled atomic.Bool
	progress  func(done, total int, message string) // nil without a token
}

var (
	activeMu sync.Mutex
	active   *callCtx
)

// current returns the call running, or one with the budget from now, as
// in tests that run tools without the server.
func current() *callCtx {
	activeMu.Lock()
	defer activeMu.Unlock()
	if active == nil {
		return &callCtx{deadline: time.Now().Add(callBudget)}
	}
	return active
}

func setActive(c *callCtx) {
	activeMu.Lock()
	active = c
	activeMu.Unlock()
}

// over reports whether the call should stop now: past its budget, or
// cancelled.
func (c *callCtx) over() bool {
	return c.cancelled.Load() || time.Now().After(c.deadline)
}

// left is how long the call has before its budget runs out.
func (c *callCtx) left() time.Duration { return time.Until(c.deadline) }

// tell sends the client a progress notification, if it asked for them.
func (c *callCtx) tell(done, total int, message string) {
	if c.progress != nil {
		c.progress(done, total, message)
	}
}
