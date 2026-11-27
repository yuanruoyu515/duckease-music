// Package spotify reads and edits the listener's Spotify playlists
// through the Web API, as a Development Mode app the listener registers
// themselves: Spotify gives such an app only to its owner and up to five
// users they name, and asks the owner to have Premium.
//
// Spotify's audio stays in Spotify: the Web API gives none. Songs of a
// Spotify playlist play from NetEase, matched by title, artists and length.
package spotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// RedirectURI is where Spotify sends the listener back after they allow
// the app; it must be among the redirect URIs of their app on Spotify's
// dashboard, exactly. Spotify takes plain http only for a loopback IP.
const RedirectURI = "http://127.0.0.1:43821/callback"

// Scopes are what the app asks to do: read the listener's playlists, also
// private and collaborative ones, and edit them.
const Scopes = "playlist-read-private playlist-read-collaborative playlist-modify-private playlist-modify-public user-read-private"

const (
	authURL  = "https://accounts.spotify.com/authorize"
	tokenURL = "https://accounts.spotify.com/api/token"
	secret   = "spotify-session"
)

// ErrNotConnected is what calls fail with before the listener connects.
var ErrNotConnected = errors.New("spotify: not connected")

// session is what is kept of the connection, in the keychain: the app's
// client id, which the listener copies from their dashboard, and tokens.
type session struct {
	ClientID string    `json:"clientId"`
	Refresh  string    `json:"refresh"`
	Access   string    `json:"access"`
	Expiry   time.Time `json:"expiry"`
	User     string    `json:"user"` // the listener's Spotify id
	Name     string    `json:"name"` // and display name
}

var (
	mu       sync.Mutex
	cur      *session  // nil until loaded
	loadedAt time.Time // when cur was read from the keychain
	// refreshing is held while a token is refreshed over the network,
	// which mu, held by quick reads such as the settings' on the main
	// thread, isn't.
	refreshing sync.Mutex
)

// staleAfter is how long what was read of the keychain holds: the other
// process, the app or the MCP server, may connect, disconnect or
// refresh meanwhile.
const staleAfter = 30 * time.Second

// Storage of the session: the keychain, or a private file in development,
// as netease keeps its own.
var (
	readSecret = func() ([]byte, error) {
		if !mygo.IsDev() {
			return mygo.Secrets.Get(secret)
		}
		p, err := sessionFile()
		if err != nil {
			return nil, err
		}
		return os.ReadFile(p)
	}
	writeSecret = func(b []byte) error {
		if !mygo.IsDev() {
			return mygo.Secrets.Set(secret, b)
		}
		p, err := sessionFile()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		return os.WriteFile(p, b, 0o600)
	}
	deleteSecret = func() {
		if !mygo.IsDev() {
			mygo.Secrets.Delete(secret)
			return
		}
		if p, err := sessionFile(); err == nil {
			os.Remove(p)
		}
	}
)

func sessionFile() (string, error) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, secret), nil
}

// load returns the session kept, or an empty one. mu is held.
func load() *session {
	if cur != nil && time.Since(loadedAt) < staleAfter {
		return cur
	}
	cur, loadedAt = &session{}, time.Now()
	if b, err := readSecret(); err == nil {
		json.Unmarshal(b, cur)
	}
	return cur
}

// save keeps s; the access token goes too, so that the MCP server, another
// process, needn't refresh it at once. mu is held.
func save(s *session) error {
	cur, loadedAt = s, time.Now()
	b, _ := json.Marshal(s)
	return writeSecret(b)
}

// Account says who is connected: their display name, or "" when no one.
func Account() (id, name string) {
	mu.Lock()
	defer mu.Unlock()
	s := load()
	if s.Refresh == "" {
		return "", ""
	}
	return s.User, s.Name
}

// ClientID returns the client id last connected with.
func ClientID() string {
	mu.Lock()
	defer mu.Unlock()
	return load().ClientID
}

// Disconnect forgets the tokens, keeping the client id.
func Disconnect() {
	mu.Lock()
	defer mu.Unlock()
	id := load().ClientID
	deleteSecret()
	cur = &session{ClientID: id}
	if id != "" {
		save(cur)
	}
}

func random(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// Login is a connection under way: the listener opens URL in the
// browser, allows the app, and Spotify sends them back to RedirectURI,
// which Wait listens on.
type Login struct {
	URL      string
	clientID string
	verifier string
	state    string
	ln       net.Listener
	cancel   chan struct{}
	once     sync.Once
}

// StartLogin starts connecting with the client id of the listener's app,
// with PKCE, which needs no client secret.
func StartLogin(clientID string) (*Login, error) {
	clientID = strings.TrimSpace(clientID)
	if len(clientID) != 32 {
		return nil, errors.New("spotify: a client id is 32 letters and digits, from the app's page on developer.spotify.com")
	}
	u, _ := url.Parse(RedirectURI)
	ln, err := net.Listen("tcp", u.Host)
	if err != nil {
		return nil, fmt.Errorf("spotify: listen on %s: %w", u.Host, err)
	}
	l := &Login{clientID: clientID, verifier: random(48), state: random(16), ln: ln, cancel: make(chan struct{})}
	sum := sha256.Sum256([]byte(l.verifier))
	q := url.Values{
		"client_id":             {clientID},
		"response_type":         {"code"},
		"redirect_uri":          {RedirectURI},
		"scope":                 {Scopes},
		"state":                 {l.state},
		"code_challenge_method": {"S256"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
	}
	l.URL = authURL + "?" + q.Encode()
	return l, nil
}

const donePage = `<!doctype html><meta charset="utf-8"><title>DuckEase Music</title>
<body style="font:16px -apple-system,sans-serif;text-align:center;padding-top:20vh">%s</body>`

// Wait waits for Spotify to send the listener back, until ctx ends, then
// trades the code for tokens and keeps them.
func (l *Login) Wait(ctx context.Context) error {
	defer l.ln.Close()
	type result struct {
		code string
		err  error
	}
	got := make(chan result, 1)
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch {
		case q.Get("state") != l.state:
			fmt.Fprintf(w, donePage, "这个链接已经失效，请回到 DuckEase Music 重新连接。")
			return
		case q.Get("error") != "":
			fmt.Fprintf(w, donePage, "没有连接 Spotify。可以关掉这个页面了。")
			select {
			case got <- result{err: fmt.Errorf("spotify: %s", q.Get("error"))}:
			default:
			}
			return
		}
		fmt.Fprintf(w, donePage, "已连接 Spotify，可以回到 DuckEase Music 了。")
		select {
		case got <- result{code: q.Get("code")}:
		default:
		}
	})}
	go srv.Serve(l.ln)
	defer srv.Close()
	var r result
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-l.cancel:
		return errors.New("spotify: login canceled")
	case r = <-got:
	}
	if r.err != nil {
		return r.err
	}
	s := &session{ClientID: l.clientID}
	if err := s.token(url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {r.code},
		"redirect_uri":  {RedirectURI},
		"code_verifier": {l.verifier},
	}); err != nil {
		return err
	}
	mu.Lock()
	before := cur
	cur, loadedAt = s, time.Now()
	mu.Unlock()
	me, err := Me()
	if err != nil {
		mu.Lock()
		cur = before // not connected after all
		mu.Unlock()
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	s.User, s.Name = me.ID, me.DisplayName
	return save(s)
}

// Cancel gives up a login before Spotify sends the listener back.
func (l *Login) Cancel() { l.once.Do(func() { close(l.cancel) }) }

// token asks Spotify for tokens with form, and puts them in s.
func (s *session) token(form url.Values) error {
	form.Set("client_id", s.ClientID)
	res, err := httpClient.PostForm(tokenURL, form)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var t struct {
		Access    string `json:"access_token"`
		Refresh   string `json:"refresh_token"`
		ExpiresIn int    `json:"expires_in"`
		Error     string `json:"error"`
		Desc      string `json:"error_description"`
	}
	if err := json.NewDecoder(res.Body).Decode(&t); err != nil {
		return fmt.Errorf("spotify: token: %s", res.Status)
	}
	if t.Error != "" {
		return fmt.Errorf("spotify: token: %s %s", t.Error, t.Desc)
	}
	s.Access = t.Access
	if t.Refresh != "" { // a refresh may or may not bring a new one
		s.Refresh = t.Refresh
	}
	s.Expiry = time.Now().Add(time.Duration(t.ExpiresIn)*time.Second - time.Minute)
	return nil
}

// accessToken returns a token good for a while, refreshing it if needed.
func accessToken() (string, error) {
	mu.Lock()
	s := *load()
	mu.Unlock()
	if s.Refresh == "" {
		return "", ErrNotConnected
	}
	if s.Access != "" && time.Now().Before(s.Expiry) {
		return s.Access, nil
	}
	refreshing.Lock()
	defer refreshing.Unlock()
	// Take what the other process kept: it may have refreshed already,
	// and its refresh token replaced this one.
	fresh := func() session {
		mu.Lock()
		defer mu.Unlock()
		cur = nil
		return *load()
	}
	s = fresh()
	for try := 0; ; try++ {
		if s.Refresh == "" {
			return "", ErrNotConnected
		}
		if s.Access != "" && time.Now().Before(s.Expiry) {
			return s.Access, nil
		}
		next := s
		err := next.token(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.Refresh}})
		if err == nil {
			mu.Lock()
			if err := save(&next); err != nil {
				// The rotated refresh token would be lost with this
				// process: say so, but use the access token meanwhile.
				log.Printf("spotify: keep session: %v", err)
			}
			mu.Unlock()
			return next.Access, nil
		}
		if !strings.Contains(err.Error(), "invalid_grant") {
			return "", err
		}
		// Refused: the other process may have refreshed at the same moment,
		// replacing the token this one sent. Try what it kept, once;
		// refused again, the connection is gone (revoked, or the app
		// deleted), and the listener connects again.
		if again := fresh(); try == 0 && again.Refresh != s.Refresh {
			s = again
			continue
		}
		mu.Lock()
		deleteSecret()
		cur, loadedAt = &session{ClientID: s.ClientID}, time.Now()
		mu.Unlock()
		return "", ErrNotConnected
	}
}
