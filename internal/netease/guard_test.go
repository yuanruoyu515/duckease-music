package netease

import (
	"errors"
	"testing"
	"time"
)

// fakeGuard keeps the guard's state in memory, on a clock the test moves.
func fakeGuard(t *testing.T) (clock *time.Time) {
	t.Helper()
	file, n, sl := guardFile, now, sleep
	t.Cleanup(func() { guardFile, now, sleep, memState = file, n, sl, guardState{} })
	at := time.Date(2026, 11, 26, 22, 0, 0, 0, time.UTC)
	guardFile = func() string { return "" }
	memState = guardState{}
	now = func() time.Time { return at }
	sleep = func(d time.Duration) { at = at.Add(d) }
	return &at
}

// Edits are spaced by editGap however fast they are asked for.
func TestGuardSpacesEdits(t *testing.T) {
	clock := fakeGuard(t)
	start := *clock
	var at []time.Duration
	for range 3 {
		guarded(guardEdit, func() error { at = append(at, clock.Sub(start)); return nil })
	}
	if at[1] != editGap || at[2] != 2*editGap {
		t.Errorf("sent at %v", at)
	}
}

// After a -460, nothing is sent until the cooldown ends, and a second
// refusal soon after doubles it.
func TestGuardCoolsDownAfter460(t *testing.T) {
	clock := fakeGuard(t)
	refuse := func() error { return errors.New("netease: /api/playlist/manipulate/tracks: code -460 风险") }
	guarded(guardEdit, refuse)
	sent := false
	err := guarded(guardSearch, func() error { sent = true; return nil })
	if b, wait := Blocked(err); sent || !b || wait != firstCooldown {
		t.Fatalf("sent %v, err %v", sent, err)
	}
	*clock = clock.Add(firstCooldown)
	guarded(guardEdit, refuse)
	_, wait := Blocked(guarded(guardEdit, func() error { return nil }))
	if wait != 2*firstCooldown {
		t.Errorf("second cooldown %v, want %v", wait, 2*firstCooldown)
	}
}
