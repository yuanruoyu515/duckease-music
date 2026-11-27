package spotify

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// EmbedLimit is how many songs a playlist's public page shows: of a
// playlist the Web API keeps from the app, all that can be read.
const EmbedLimit = 100

// embedURL is where a playlist's public player page is; tests point it
// elsewhere.
var embedURL = "https://open.spotify.com/embed/playlist/"

var nextData = regexp.MustCompile(`(?s)<script id="__NEXT_DATA__" type="application/json">(.*?)</script>`)

// embedItems reads the songs of playlist id off its public player page,
// the one a site embeds, which needs no account: the first EmbedLimit,
// with their titles, artists and lengths but no albums. A private
// playlist has no such page.
func embedItems(id string) ([]Track, error) {
	req, _ := http.NewRequest("GET", embedURL+url.PathEscape(id), nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15")
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, ErrNotReadable
	}
	b, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	return parseEmbed(b)
}

// parseEmbed takes the songs out of a public player page.
func parseEmbed(page []byte) ([]Track, error) {
	m := nextData.FindSubmatch(page)
	if m == nil {
		return nil, ErrNotReadable
	}
	var d struct {
		Props struct {
			PageProps struct {
				State struct {
					Data struct {
						Entity struct {
							TrackList []struct {
								URI        string `json:"uri"`
								Title      string `json:"title"`
								Subtitle   string `json:"subtitle"`
								Duration   int64  `json:"duration"`
								EntityType string `json:"entityType"`
							} `json:"trackList"`
						} `json:"entity"`
					} `json:"data"`
				} `json:"state"`
			} `json:"pageProps"`
		} `json:"props"`
	}
	if err := json.Unmarshal(m[1], &d); err != nil {
		return nil, fmt.Errorf("spotify: public page: %w", err)
	}
	list := d.Props.PageProps.State.Data.Entity.TrackList
	if list == nil {
		return nil, ErrNotReadable
	}
	out := make([]Track, 0, len(list))
	for _, it := range list {
		kind, id := splitURI(it.URI)
		if kind != "track" || id == "" {
			continue
		}
		t := Track{ID: id, URI: it.URI, Name: it.Title, Type: "track", Duration: it.Duration}
		for _, name := range strings.Split(it.Subtitle, ", ") {
			if name = strings.TrimSpace(name); name != "" {
				t.Artists = append(t.Artists, struct {
					Name string `json:"name"`
				}{name})
			}
		}
		out = append(out, t)
	}
	return out, nil
}
