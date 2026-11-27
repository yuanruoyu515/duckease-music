// Package mcp lets an agent such as Codex or Claude Code manage the
// listener's NetEase playlists, as a Model Context Protocol server over
// stdio: `DuckEase Music mcp`. The app's own binary serves it, so it reads
// the session the app keeps in the keychain.
//
// Replies are plain text, one song or playlist a line with its fields
// split by tabs, which costs an agent far fewer tokens than JSON.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"netease-music-mygo/internal/netease"
)

const protocolVersion = "2025-06-18"

// supported are the protocol versions the server speaks; it answers a
// client asking for another with the latest.
// 2025-03-26 is left out: it has JSON-RPC batches, which the server
// doesn't take.
var supported = map[string]bool{"2024-11-05": true, "2025-06-18": true}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Serve answers the requests read from in on out until in ends. Tools
// run one at a time, in the order asked, so that NetEase sees no bursts;
// a ping or a list of the tools is answered meanwhile, a call the client
// cancels before it starts doesn't run, and one it cancels while it runs
// stops at its next song. A call that runs past callLimit is answered
// with an error, and those after it, while it still runs, are told so.
func Serve(in io.Reader, out io.Writer, version string) error {
	s := &server{version: version, tools: newTools()}
	enc := json.NewEncoder(out)
	var mu sync.Mutex
	send := func(r any) {
		mu.Lock()
		defer mu.Unlock()
		if err := enc.Encode(r); err != nil {
			log.Printf("mcp: write: %v", err)
		}
	}
	var cancelMu sync.Mutex
	cancelled := map[string]bool{}
	var running *callCtx // the call under way, which a cancel stops
	var runningID string
	calls := make(chan request, 64)
	done := make(chan struct{})
	go func() {
		defer close(done)
		var stuck chan struct{} // closed when a call past callLimit ends
		var stuckName string
		var stuckSince time.Time
		for req := range calls {
			cancelMu.Lock()
			skip := cancelled[string(req.ID)]
			delete(cancelled, string(req.ID))
			cancelMu.Unlock()
			if skip {
				continue
			}
			if stuck != nil {
				select {
				case <-stuck:
					stuck = nil
				case <-time.After(busyWait):
					msg := fmt.Sprintf("error: the earlier call %s is still running (%s so far), maybe waiting on a keychain dialog on the user's Mac or on NetEase. Wait a minute and try again; don't repeat write calls before checking what they did.", stuckName, time.Since(stuckSince).Round(time.Second))
					send(response{JSONRPC: "2.0", ID: req.ID, Result: errorResult(msg)})
					continue
				}
			}
			ctx := &callCtx{deadline: time.Now().Add(callBudget)}
			if token := progressToken(req.Params); token != nil {
				ctx.progress = func(done, total int, message string) {
					send(map[string]any{"jsonrpc": "2.0", "method": "notifications/progress", "params": map[string]any{
						"progressToken": token, "progress": done, "total": total, "message": message,
					}})
				}
			}
			cancelMu.Lock()
			running, runningID = ctx, string(req.ID)
			cancelMu.Unlock()
			finished := make(chan struct{})
			var result any
			var rerr *rpcError
			go func() {
				defer close(finished)
				setActive(ctx)
				defer setActive(nil)
				result, rerr = s.handle(req)
			}()
			select {
			case <-finished:
				send(response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr})
			case <-time.After(callLimit):
				stuck, stuckName, stuckSince = finished, toolName(req.Params), time.Now().Add(-callLimit)
				ctx.cancelled.Store(true) // stop at its next song, if it gets to one
				msg := fmt.Sprintf("error: %s took over %s and was told to stop; it may still finish what it was doing. Check the playlists before calling it again.", stuckName, callLimit)
				send(response{JSONRPC: "2.0", ID: req.ID, Result: errorResult(msg)})
			}
			cancelMu.Lock()
			running, runningID = nil, ""
			cancelMu.Unlock()
		}
	}()
	defer func() {
		close(calls)
		<-done
	}()
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			send(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "parse error"}})
			continue
		}
		switch {
		case req.Method == "notifications/cancelled":
			var p struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			json.Unmarshal(req.Params, &p)
			cancelMu.Lock()
			if running != nil && runningID == string(p.RequestID) {
				running.cancelled.Store(true)
			} else {
				cancelled[string(p.RequestID)] = true
			}
			cancelMu.Unlock()
			continue
		case req.Method == "tools/call" && len(req.ID) > 0:
			calls <- req
			continue
		}
		result, rerr := s.handle(req)
		if len(req.ID) == 0 { // a notification: no reply
			continue
		}
		send(response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr})
	}
	return sc.Err()
}

// progressToken returns the token a call asked progress notifications
// with, or nil.
func progressToken(params json.RawMessage) any {
	var p struct {
		Meta struct {
			ProgressToken any `json:"progressToken"`
		} `json:"_meta"`
	}
	json.Unmarshal(params, &p)
	return p.Meta.ProgressToken
}

// toolName returns the name of the tool a call asks for.
func toolName(params json.RawMessage) string {
	var p struct {
		Name string `json:"name"`
	}
	json.Unmarshal(params, &p)
	return p.Name
}

func errorResult(msg string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": msg}}, "isError": true}
}

type server struct {
	version string
	tools   []tool
}

func (s *server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(req.Params, &p)
		v := protocolVersion
		if supported[p.ProtocolVersion] {
			v = p.ProtocolVersion
		}
		return map[string]any{
			"protocolVersion": v,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "duckease-music", "version": s.version},
			"instructions":    instructions,
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		list := make([]map[string]any, len(s.tools))
		for i, t := range s.tools {
			list[i] = t.describe()
		}
		return map[string]any{"tools": list}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "invalid params"}
		}
		for _, t := range s.tools {
			if t.name == p.Name {
				if !sessionRead() {
					return errorResult(sessionNote), nil
				}
				return call(t, p.Arguments), nil
			}
		}
		return nil, &rpcError{-32602, "unknown tool " + p.Name}
	}
	if len(req.ID) == 0 {
		return nil, nil
	}
	return nil, &rpcError{-32601, "method not found: " + req.Method}
}

// call runs a tool; a failure goes back to the agent as the tool's text,
// marked as an error, so that it can read it and try otherwise.
func call(t tool, args json.RawMessage) map[string]any {
	if len(args) == 0 || string(args) == "null" {
		args = json.RawMessage("{}")
	}
	text, err := func() (text string, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("internal error: %v", r)
			}
		}()
		return t.run(args)
	}()
	if err != nil {
		msg := "error: " + err.Error()
		if blocked, _ := netease.Blocked(err); blocked {
			msg = "error: " + coolingNote(err)
		}
		return errorResult(msg)
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// coolingNote tells an agent NetEase's risk control refused, and how long
// to leave it be: retrying sooner only lengthens it.
func coolingNote(err error) string {
	_, wait := netease.Blocked(err)
	if wait <= 0 {
		wait = time.Minute
	}
	if tooOften(err) {
		return fmt.Sprintf("NetEase says searches come too often (405). Make no NetEase searches for %s, then go on; other calls work meanwhile. Work already done stays done.", wait.Round(time.Second))
	}
	return fmt.Sprintf("NetEase risk control (-460) is refusing requests. Make no NetEase calls for %s, then try again; don't retry sooner, it lengthens the block. Work already done stays done.", wait.Round(time.Second))
}

const instructions = `Manage the user's NetEase Cloud Music (网易云音乐) playlists in DuckEase Music.
Replies are tab-separated lines with a header; song lines are: id, title, artists, album, length; "!" before an id marks a song the user can't stream.
Work by id: list playlists, read one (use query to filter instead of reading long lists), find songs with search or match (match takes many "title - artist" lines at once), then add, remove or move by id.
Spotify tools (spotify_*, sync_*) work once Spotify is connected in the app's settings. Playlist id "liked" is the user's Spotify Liked Songs; of playlists others made only the first 100 songs can be read, and only the user's own or collaborative ones changed. Syncing only adds songs.
Long calls (match, sync_playlist, spotify_import) stop after about 45 seconds so the reply comes in time, keeping what they found; call again as the reply says.
Write tools change the user's real account. NetEase's risk control (-460) watches bursts: the server spaces every request and stops all of them for a while after a -460, saying how long; wait that long. Prefer one call with many ids over many calls with one, and don't call tools in parallel.`
