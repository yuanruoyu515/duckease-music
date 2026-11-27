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

// A search answered 405 holds searches, not edits, for searchHold, and
// doubles their gap after; it was taken for a failure, and matching a
// long Spotify playlist stopped there.
func TestGuardSlowsSearchesAfter405(t *testing.T) {
	clock := fakeGuard(t)
	guarded(guardSearch, func() error {
		return errors.New("netease: /api/cloudsearch/pc: code 405 操作频繁，请稍候再试")
	})
	if _, wait := Blocked(guarded(guardSearch, func() error { return nil })); wait != searchHold {
		t.Fatalf("search held %v, want %v", wait, searchHold)
	}
	if err := guarded(guardEdit, func() error { return nil }); err != nil {
		t.Errorf("an edit was held: %v", err)
	}
	*clock = clock.Add(searchHold)
	guarded(guardSearch, func() error { return nil })
	start := *clock
	guarded(guardSearch, func() error { return nil })
	if gap := clock.Sub(start); gap != 2*searchGap {
		t.Errorf("gap after a 405 %v, want %v", gap, 2*searchGap)
	}
}
