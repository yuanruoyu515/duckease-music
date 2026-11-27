package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// The guard paces the requests NetEase's risk control watches, edits and
// searches, and stops them all for a while once it answers -460: asking
// on while refused only lengthens it. The app and its MCP server (`mcp`)
// are two processes on one account, so the guard keeps its state in a
// file both read.

// Gaps between guarded requests of a kind, whoever sends them.
var (
	editGap   = 400 * time.Millisecond
	searchGap = 400 * time.Millisecond
)

// ReadGap is the least time between any two requests: 0 in the app, whose
// pages load what they need, and set by the MCP server, whose agent may
// read one playlist after another.
var ReadGap time.Duration

// Cooldowns after -460: the first, doubling with each refusal within
// strikeWindow of the last, up to maxCooldown.
const (
	firstCooldown = time.Minute
	maxCooldown   = 15 * time.Minute
	strikeWindow  = 10 * time.Minute
)

// CoolingError is a request not sent, while NetEase's risk control is
// cooling down.
type CoolingError struct{ Until time.Time }

func (e *CoolingError) Error() string {
	return fmt.Sprintf("NetEase refused requests (-460); not asking again for %s", e.Until.Sub(now()).Round(time.Second))
}

// Blocked reports whether err is NetEase's risk control: its -460, or a
// request the guard held back for it. With it, wait says how long until
// requests go out again, 0 when unknown.
func Blocked(err error) (blocked bool, wait time.Duration) {
	var c *CoolingError
	if errors.As(err, &c) {
		return true, max(c.Until.Sub(now()), 0)
	}
	var r *RefusedError
	if errors.As(err, &r) && r.Blocked() {
		return true, 0
	}
	return err != nil && strings.Contains(err.Error(), "code -460"), 0
}

// guardState is what the processes share.
type guardState struct {
	BlockedUntil time.Time `json:"blockedUntil"`
	Strikes      int       `json:"strikes"`
	LastStrike   time.Time `json:"lastStrike"`
	LastEdit     time.Time `json:"lastEdit"`
	LastSearch   time.Time `json:"lastSearch"`
}

var (
	guardMu  sync.Mutex
	lastRead time.Time
	// guardFile is where the state is kept; "" keeps it in memory, as in
	// tests.
	guardFile = func() string {
		dir, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			return ""
		}
		return filepath.Join(dir, "netease-guard.json")
	}
	memState guardState
	now      = time.Now
	sleep    = time.Sleep
)

func loadGuard() guardState {
	p := guardFile()
	if p == "" {
		return memState
	}
	var s guardState
	if b, err := os.ReadFile(p); err == nil {
		json.Unmarshal(b, &s)
	}
	return s
}

func saveGuard(s guardState) {
	p := guardFile()
	if p == "" {
		memState = s
		return
	}
	b, _ := json.Marshal(s)
	os.MkdirAll(filepath.Dir(p), 0o755)
	tmp := fmt.Sprintf("%s.%d.tmp", p, os.Getpid())
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, p)
	}
}

type guardKind int

const (
	guardEdit guardKind = iota
	guardSearch
)

// guarded sends a request of kind through fn: not while cooling down,
// and no sooner than its gap after the last of its kind.
func guarded(kind guardKind, fn func() error) error {
	guardMu.Lock()
	s := loadGuard()
	t := now()
	if t.Before(s.BlockedUntil) {
		guardMu.Unlock()
		return &CoolingError{Until: s.BlockedUntil}
	}
	last, gap := &s.LastEdit, editGap
	if kind == guardSearch {
		last, gap = &s.LastSearch, searchGap
	}
	if wait := last.Add(gap).Sub(t); wait > 0 {
		sleep(wait)
		t = now()
	}
	*last = t
	saveGuard(s)
	guardMu.Unlock()

	err := fn()
	if b, _ := Blocked(err); b {
		trip()
	}
	return err
}

// trip starts a cooldown, after NetEase answered -460, to any request.
func trip() {
	guardMu.Lock()
	defer guardMu.Unlock()
	s := loadGuard()
	t := now()
	if t.Before(s.BlockedUntil) {
		return // the same refusal, seen by another request
	}
	if t.Sub(s.LastStrike) > strikeWindow {
		s.Strikes = 0
	}
	s.Strikes++
	d := firstCooldown << min(s.Strikes-1, 8)
	s.BlockedUntil, s.LastStrike = t.Add(min(d, maxCooldown)), t
	saveGuard(s)
}

// paceRead waits out ReadGap since the last request of this process.
func paceRead() {
	if ReadGap <= 0 {
		return
	}
	guardMu.Lock()
	defer guardMu.Unlock()
	if wait := lastRead.Add(ReadGap).Sub(now()); wait > 0 {
		sleep(wait)
	}
	lastRead = now()
}
