package spotify

import "testing"

// A playlist's public page gives its songs with each artist apart, and
// leaves out episodes.
func TestParseEmbed(t *testing.T) {
	page := []byte(`<html><script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"state":{"data":{"entity":{"trackList":[
{"uri":"spotify:track:abc","title":"BbY WOW","subtitle":"KAROL G, Judeline, rusowsky","duration":201000,"entityType":"track"},
{"uri":"spotify:episode:xyz","title":"A show","subtitle":"Host","duration":60000,"entityType":"episode"}
]}}}}}}</script></html>`)
	got, err := parseEmbed(page)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d songs, want 1", len(got))
	}
	tr := got[0]
	if tr.ID != "abc" || tr.Name != "BbY WOW" || tr.Duration != 201000 || len(tr.Artists) != 3 || tr.Artists[2].Name != "rusowsky" {
		t.Errorf("parsed %+v", tr)
	}
}

// A page without the data, as a private playlist's, can't be read.
func TestParseEmbedMissing(t *testing.T) {
	if _, err := parseEmbed([]byte("<html>nothing</html>")); err != ErrNotReadable {
		t.Errorf("err %v, want ErrNotReadable", err)
	}
}
