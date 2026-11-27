package match

import (
	"testing"
	"time"
)

// Versions' additions to a title don't stop it matching the original.
func TestTitleDropsVersions(t *testing.T) {
	for in, want := range map[string]string{
		"Shape of You - Remastered 2011": "shapeofyou",
		"Stay (feat. Justin Bieber)":     "stay",
		"晴天（Live）":                       "晴天",
		"Ｈｅｌｌｏ":                          "hello",
		"[Intro]":                        "intro",
	} {
		if got := Title(in); got != want {
			t.Errorf("Title(%q) = %q, want %q", in, got, want)
		}
	}
}

// A cover by someone else loses to the original, and a song too unlike the
// one asked for isn't taken at all.
func TestBestPrefersTheOriginal(t *testing.T) {
	want := Song{Title: "晴天", Artists: []string{"周杰伦"}, Length: 269 * time.Second}
	cands := []Song{
		{Title: "晴天 (原唱 周杰伦)", Artists: []string{"RyaVocal"}, Length: 270 * time.Second},
		{Title: "晴天", Artists: []string{"周杰伦"}, Length: 269 * time.Second},
	}
	if i, _ := Best(want, cands); i != 1 {
		t.Errorf("Best picked %d, want the original", i)
	}
	if i, _ := Best(want, []Song{{Title: "七里香", Artists: []string{"周杰伦"}}}); i != -1 {
		t.Errorf("Best took another song of the artist: %d", i)
	}
}

// Parse splits an agent's "title - artists" line.
func TestParse(t *testing.T) {
	s := Parse("Stay - The Kid LAROI, Justin Bieber")
	if s.Title != "Stay" || len(s.Artists) != 2 || s.Artists[1] != "Justin Bieber" {
		t.Errorf("Parse = %+v", s)
	}
	if s := Parse("晴天"); s.Title != "晴天" || s.Artists != nil {
		t.Errorf("Parse title alone = %+v", s)
	}
}

// The same title by someone else isn't taken for the song unless the
// length is the same too, as a cover of another length isn't.
func TestOtherArtistNeedsSameLength(t *testing.T) {
	want := Song{Title: "Stay", Artists: []string{"Rihanna"}, Length: 240 * time.Second}
	cover := Song{Title: "Stay", Artists: []string{"Someone"}, Length: 246 * time.Second}
	if s := Score(want, cover); s >= Threshold {
		t.Errorf("a cover scored %.2f", s)
	}
}

// An artist's name inside another's isn't the same artist: Jay isn't Jay
// Chou, as a live search took him for; The Weeknd is Weeknd.
func TestArtistNameInsideAnother(t *testing.T) {
	if sameArtist(Clean("Jay"), Clean("Jay Chou")) {
		t.Error("Jay taken for Jay Chou")
	}
	if !sameArtist(Clean("The Weeknd"), Clean("Weeknd")) {
		t.Error("The Weeknd not taken for Weeknd")
	}
}
