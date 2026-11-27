package spotify

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Spotify sending the listener back with someone else's state, as a stale
// tab would, doesn't end the login; their refusal does, with its error.
func TestLoginChecksState(t *testing.T) {
	l, err := StartLogin(strings.Repeat("a", 32))
	if err != nil {
		t.Skip("port taken:", err)
	}
	done := make(chan error, 1)
	go func() { done <- l.Wait(context.Background()) }()
	time.Sleep(50 * time.Millisecond)
	get := func(q url.Values) {
		res, err := http.Get(RedirectURI + "?" + q.Encode())
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	get(url.Values{"state": {"stale"}, "code": {"x"}})
	select {
	case err := <-done:
		t.Fatalf("a stale state ended the login: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	get(url.Values{"state": {l.state}, "error": {"access_denied"}})
	if err := <-done; err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("Wait = %v", err)
	}
}
