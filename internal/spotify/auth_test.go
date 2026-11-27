package spotify

import (
	"context"
	"io"
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

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A refresh refused because the other process refreshed at the same
// moment, rotating the token, takes what that process kept, instead of
// deleting it and signing both out.
func TestRefreshRaceKeepsTheOthersToken(t *testing.T) {
	keychain := []byte(`{"clientId":"c","refresh":"old","access":"x","expiry":"2000-01-01T00:00:00Z"}`)
	r, w, d, client := readSecret, writeSecret, deleteSecret, httpClient
	defer func() { readSecret, writeSecret, deleteSecret, httpClient, cur = r, w, d, client, nil }()
	readSecret = func() ([]byte, error) { return keychain, nil }
	writeSecret = func(b []byte) error { keychain = b; return nil }
	deleted := false
	deleteSecret = func() { deleted = true }
	cur = nil
	httpClient = &http.Client{Transport: roundTrip(func(req *http.Request) (*http.Response, error) {
		// Meanwhile the other process refreshed and kept a new token.
		keychain = []byte(`{"clientId":"c","refresh":"new","access":"fresh","expiry":"2999-01-01T00:00:00Z"}`)
		return &http.Response{StatusCode: 400, Body: io.NopCloser(strings.NewReader(`{"error":"invalid_grant"}`)), Header: http.Header{}}, nil
	})}
	tok, err := accessToken()
	if err != nil || tok != "fresh" || deleted {
		t.Errorf("token %q, err %v, deleted %v", tok, err, deleted)
	}
}
