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

// Searches answered 405, 操作频繁 (too often): NetEase's search has a
// rate of its own, below which the guard keeps them. Each 405 holds
// searches for searchHold and doubles the gap between them, up to
// maxSlow doublings; slowWindow after the last, the gap is back.
const (
	searchHold = 20 * time.Second
	maxSlow    = 4
	slowWindow = 10 * time.Minute
)

// CoolingError is a request not sent, while NetEase's risk control is
// cooling down, or its search asked for fewer.
type CoolingError struct {
	Until    time.Time
	TooOften bool // a 405's, holding searches only
}

func (e *CoolingError) Error() string {
	if e.TooOften {
		return fmt.Sprintf("NetEase searched too often (405); searching again in %s", e.Until.Sub(now()).Round(time.Second))
	}
	return fmt.Sprintf("NetEase refused requests (-460); not asking again for %s", e.Until.Sub(now()).Round(time.Second))
}

// tooOften reports whether err is NetEase's 405, 操作频繁.
func tooOften(err error) bool { return err != nil && strings.Contains(err.Error(), "code 405") }

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
	if tooOften(err) {
		return true, searchHold
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
	// After a 405: searches wait until SearchHeld, and their gap is
	// doubled Slow times, until slowWindow after LastSlow.
	SearchHeld time.Time `json:"searchHeld"`
	Slow       int       `json:"slow"`
	LastSlow   time.Time `json:"lastSlow"`
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
		if t.Before(s.SearchHeld) {
			guardMu.Unlock()
			return &CoolingError{Until: s.SearchHeld, TooOften: true}
		}
		if t.Sub(s.LastSlow) > slowWindow {
			s.Slow = 0
		}
		last, gap = &s.LastSearch, searchGap<<s.Slow
	}
	if wait := last.Add(gap).Sub(t); wait > 0 {
		sleep(wait)
		t = now()
	}
	*last = t
	saveGuard(s)
	guardMu.Unlock()

	err := fn()
	switch b, _ := Blocked(err); {
	case kind == guardSearch && tooOften(err):
		slowSearches()
	case b:
		trip()
	}
	return err
}

// slowSearches holds searches a while after a 405, and spaces them more.
func slowSearches() {
	guardMu.Lock()
	defer guardMu.Unlock()
	s := loadGuard()
	t := now()
	if t.Before(s.SearchHeld) {
		return
	}
	s.Slow = min(s.Slow+1, maxSlow)
	s.SearchHeld, s.LastSlow = t.Add(searchHold), t
	saveGuard(s)
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
