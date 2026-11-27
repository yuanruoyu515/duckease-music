package spotify

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const apiBase = "https://api.spotify.com/v1"

var httpClient = &http.Client{Timeout: 20 * time.Second}

// APIError is a refusal from the Web API.
type APIError struct {
	Status  int
	Message string
	Reason  string // e.g. QUOTA_EXCEEDED
}

func (e *APIError) Error() string {
	s := fmt.Sprintf("spotify: %d %s", e.Status, e.Message)
	if e.Reason != "" {
		s += " (" + e.Reason + ")"
	}
	return s
}

// maxWait is the longest Retry-After waited out; beyond it, the 429 goes
// back to the caller.
const maxWait = 10 * time.Second

// call sends a request to the Web API and decodes its reply into out.
func call(method, path string, query url.Values, body, out any) error {
	tok, err := accessToken()
	if err != nil {
		return err
	}
	u := path
	if !strings.HasPrefix(u, "https://") {
		u = apiBase + path
	}
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	for try := 0; ; try++ {
		req, _ := http.NewRequest(method, u, bytes.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+tok)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		res, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		b, _ := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if res.StatusCode == http.StatusTooManyRequests && try < 2 {
			wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			if d := time.Duration(max(wait, 1)) * time.Second; d <= maxWait && !bytes.Contains(b, []byte("QUOTA_EXCEEDED")) {
				time.Sleep(d)
				continue
			}
		}
		if res.StatusCode >= 300 {
			var e struct {
				Error struct {
					Status  int    `json:"status"`
					Message string `json:"message"`
					Reason  string `json:"reason"`
				} `json:"error"`
			}
			json.Unmarshal(b, &e)
			if e.Error.Message == "" {
				e.Error.Message = res.Status
			}
			return &APIError{Status: res.StatusCode, Message: e.Error.Message, Reason: e.Error.Reason}
		}
		if out == nil || len(b) == 0 {
			return nil
		}
		return json.Unmarshal(b, out)
	}
}

// User is a Spotify account.
type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

// Me returns who is connected.
func Me() (*User, error) {
	var u User
	return &u, call("GET", "/me", nil, nil, &u)
}

// Track is a song on Spotify.
type Track struct {
	ID       string `json:"id"`
	URI      string `json:"uri"`
	Name     string `json:"name"`
	Type     string `json:"type"` // track or episode
	Duration int64  `json:"duration_ms"`
	Artists  []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Album struct {
		Name   string `json:"name"`
		Images []struct {
			URL   string `json:"url"`
			Width int    `json:"width"`
		} `json:"images"`
	} `json:"album"`
	ExternalIDs struct {
		ISRC string `json:"isrc"`
	} `json:"external_ids"`
	IsLocal bool `json:"is_local"`
}

// ArtistNames joins the track's artists' names.
func (t *Track) ArtistNames() string {
	names := make([]string, len(t.Artists))
	for i, a := range t.Artists {
		names[i] = a.Name
	}
	return strings.Join(names, " / ")
}

// Length is how long the track is.
func (t *Track) Length() time.Duration { return time.Duration(t.Duration) * time.Millisecond }

// Cover returns the track's smallest album art at least size pixels
// wide, or its largest when none is.
func (t *Track) Cover(size int) string {
	best, bestW := "", 0
	for _, im := range t.Album.Images {
		fits, bestFits := im.Width >= size, bestW >= size
		if best == "" || (fits && (!bestFits || im.Width < bestW)) || (!fits && !bestFits && im.Width > bestW) {
			best, bestW = im.URL, im.Width
		}
	}
	return best
}

// Playlist is a Spotify playlist, without its songs.
type Playlist struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Collaborative bool   `json:"collaborative"`
	Public        bool   `json:"public"`
	SnapshotID    string `json:"snapshot_id"`
	Owner         User   `json:"owner"`
	Images        []struct {
		URL string `json:"url"`
	} `json:"images"`
	// Since February 2026 Spotify calls the songs items; older replies
	// say tracks.
	Items  *struct{ Total int } `json:"items"`
	Tracks *struct{ Total int } `json:"tracks"`
}

// Total is how many songs the playlist has.
func (p *Playlist) Total() int {
	switch {
	case p.Items != nil:
		return p.Items.Total
	case p.Tracks != nil:
		return p.Tracks.Total
	}
	return 0
}

// Cover returns the playlist's image, or "".
func (p *Playlist) Cover() string {
	if len(p.Images) == 0 {
		return ""
	}
	return p.Images[0].URL
}

// Editable reports whether user may change the playlist's songs: their
// own, or a collaborative one. Only those does Spotify let a Development
// Mode app read the songs of, too.
func (p *Playlist) Editable(user string) bool { return p.Owner.ID == user || p.Collaborative }

// Playlists returns the listener's playlists: those they made, follow or
// collaborate on, at most 500.
func Playlists() ([]Playlist, error) {
	var all []Playlist
	next := "/me/playlists?limit=50"
	for next != "" && len(all) < 500 {
		var page struct {
			Items []Playlist `json:"items"`
			Next  string     `json:"next"`
		}
		if err := call("GET", next, nil, nil, &page); err != nil {
			return all, err
		}
		all = append(all, page.Items...)
		next = page.Next
	}
	return all, nil
}

// ErrNotReadable is what reading the songs of a playlist the listener
// neither made nor collaborates on gives: Spotify keeps them from
// Development Mode apps.
var ErrNotReadable = errors.New("spotify: only the songs of playlists you made or collaborate on can be read")

// PlaylistItems returns the songs of playlist id, in order, without local
// files and podcast episodes, at most 5000.
func PlaylistItems(id string) ([]Track, error) {
	var all []Track
	next := "/playlists/" + url.PathEscape(id) + "/items?limit=50&additional_types=track"
	for next != "" && len(all) < 5000 {
		var page struct {
			Items []struct {
				Item  *Track `json:"item"`
				Track *Track `json:"track"` // before February 2026
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := call("GET", next, nil, nil, &page); err != nil {
			var e *APIError
			if errors.As(err, &e) && (e.Status == 403 || e.Status == 404) && len(all) == 0 {
				return nil, ErrNotReadable
			}
			return all, err
		}
		for _, it := range page.Items {
			t := it.Item
			if t == nil {
				t = it.Track
			}
			if t == nil || t.IsLocal || t.ID == "" || (t.Type != "" && t.Type != "track") {
				continue
			}
			all = append(all, *t)
		}
		next = page.Next
	}
	return all, nil
}

// Search returns up to limit tracks matching query, at most 10, as many as
// Spotify gives a Development Mode app at once.
func Search(query string, limit int) ([]Track, error) {
	var res struct {
		Tracks struct {
			Items []Track `json:"items"`
		} `json:"tracks"`
	}
	q := url.Values{"q": {query}, "type": {"track"}, "limit": {strconv.Itoa(min(max(limit, 1), 10))}}
	err := call("GET", "/search", q, nil, &res)
	return res.Tracks.Items, err
}

// CreatePlaylist makes a private playlist for the listener.
func CreatePlaylist(name, description string) (*Playlist, error) {
	var p Playlist
	body := map[string]any{"name": name, "public": false, "description": description}
	return &p, call("POST", "/me/playlists", nil, body, &p)
}

// AddItems adds tracks, by URI, to the end of playlist id, 100 at a time.
func AddItems(id string, uris []string) error {
	for len(uris) > 0 {
		n := min(len(uris), 100)
		if err := call("POST", "/playlists/"+url.PathEscape(id)+"/items", nil, map[string]any{"uris": uris[:n]}, nil); err != nil {
			return err
		}
		uris = uris[n:]
	}
	return nil
}

// RemoveItems takes tracks, by URI, out of playlist id, 100 at a time.
func RemoveItems(id string, uris []string) error {
	for len(uris) > 0 {
		n := min(len(uris), 100)
		items := make([]map[string]string, n)
		for i, u := range uris[:n] {
			items[i] = map[string]string{"uri": u}
		}
		if err := call("DELETE", "/playlists/"+url.PathEscape(id)+"/items", nil, map[string]any{"items": items}, nil); err != nil {
			return err
		}
		uris = uris[n:]
	}
	return nil
}
