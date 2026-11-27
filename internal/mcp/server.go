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
)

const protocolVersion = "2025-06-18"

// supported are the protocol versions the server speaks; it answers a
// client asking for another with the latest.
var supported = map[string]bool{"2024-11-05": true, "2025-03-26": true, "2025-06-18": true}

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

// Serve answers the requests read from in on out until in ends. Requests
// are handled one at a time, so that NetEase sees no bursts.
func Serve(in io.Reader, out io.Writer, version string) error {
	s := &server{version: version, tools: newTools()}
	enc := json.NewEncoder(out)
	var mu sync.Mutex
	send := func(r response) {
		mu.Lock()
		defer mu.Unlock()
		if err := enc.Encode(r); err != nil {
			log.Printf("mcp: write: %v", err)
		}
	}
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
		result, rerr := s.handle(req)
		if len(req.ID) == 0 { // a notification: no reply
			continue
		}
		send(response{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rerr})
	}
	return sc.Err()
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
		return map[string]any{"content": []map[string]any{{"type": "text", "text": "error: " + err.Error()}}, "isError": true}
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

const instructions = `Manage the user's NetEase Cloud Music (网易云音乐) playlists in DuckEase Music.
Replies are tab-separated lines with a header; song lines are: id, title, artists, album, length; "!" before an id marks a song the user can't stream.
Work by id: list playlists, read one (use query to filter instead of reading long lists), find songs with search or match (match takes many "title - artist" lines at once), then add, remove or move by id.
Spotify tools (spotify_*, sync_*) work once Spotify is connected in the app's settings; Spotify lets them read the songs only of playlists the user made or collaborates on. Syncing only adds songs.
Write tools change the user's real account. NetEase rate-limits bursts (error -460): the server already spaces its requests; don't call write tools in parallel.`
