// Package match finds a song of one service among the songs of another,
// by title, artists and length: a song named on Spotify, or written as
// "title - artist" by an agent, among what a NetEase search returns.
package match

import (
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
	"golang.org/x/text/width"
)

// Song is what is known of a song: Length may be 0 when unknown.
type Song struct {
	Title   string
	Artists []string
	Length  time.Duration
}

// Threshold is the score below which a candidate isn't taken as the song.
const Threshold = 0.6

// Best returns the index of the candidate most like want and its score,
// from 0 to 1, or -1 when none reaches Threshold.
func Best(want Song, candidates []Song) (int, float64) {
	best, score := -1, 0.0
	for i, c := range candidates {
		if s := Score(want, c); s > score {
			best, score = i, s
		}
	}
	if score < Threshold {
		return -1, score
	}
	return best, score
}

// Score says how alike two songs are, from 0 to 1. The title counts most;
// artists and length tell a cover or a live take from the original.
func Score(a, b Song) float64 {
	ta, tb := Title(a.Title), Title(b.Title)
	if ta == "" || tb == "" {
		return 0
	}
	var title float64
	switch {
	case ta == tb:
		title = 1
	case strings.Contains(ta, tb) || strings.Contains(tb, ta):
		title = 0.8
	default:
		title = 1 - float64(distance(ta, tb))/float64(max(len([]rune(ta)), len([]rune(tb))))
		if title < 0.5 {
			return 0
		}
	}
	artist := 0.5 // unknown on either side: neither for nor against
	if len(a.Artists) > 0 && len(b.Artists) > 0 {
		artist = 0
		for _, x := range a.Artists {
			for _, y := range b.Artists {
				nx, ny := Clean(x), Clean(y)
				if nx != "" && ny != "" && (nx == ny || strings.Contains(nx, ny) || strings.Contains(ny, nx)) {
					artist = 1
				}
			}
		}
	}
	length := 0.5
	if a.Length > 0 && b.Length > 0 {
		d := a.Length - b.Length
		if d < 0 {
			d = -d
		}
		switch {
		case d <= 3*time.Second:
			length = 1
		case d <= 10*time.Second:
			length = 0.6
		default:
			length = 0
		}
	}
	return 0.55*title + 0.3*artist + 0.15*length
}

// Title is a song's title cleaned, without what versions add to it: "(feat.
// …)", "- Remastered 2011", "（Live）".
func Title(s string) string {
	s = foldWidth(s)
	if i := strings.Index(s, " - "); i > 0 {
		s = s[:i]
	}
	s = dropBrackets(s)
	return Clean(s)
}

// TitleWords is a song's title without what versions add to it, as Title
// cleans it, but with its words and spaces kept, for a search.
func TitleWords(s string) string {
	s = foldWidth(s)
	if i := strings.Index(s, " - "); i > 0 {
		s = s[:i]
	}
	return strings.Join(strings.Fields(dropBrackets(s)), " ")
}

// Clean lowercases s and keeps only its letters and digits, with
// full-width forms folded to half-width.
func Clean(s string) string {
	s = norm.NFKC.String(foldWidth(s))
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func foldWidth(s string) string { return width.Fold.String(s) }

// dropBrackets takes out what is in brackets, unless that is all there is.
func dropBrackets(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(', '[', '【', '「', '《':
			depth++
		case ')', ']', '】', '」', '》':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	if strings.TrimSpace(b.String()) == "" {
		return s
	}
	return b.String()
}

// distance is the Levenshtein distance between a and b, in runes.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

// Parse reads a song written "title - artist[, artist]", as an agent
// writes it; a line without " - " is a title alone.
func Parse(line string) Song {
	title, artists, ok := strings.Cut(line, " - ")
	s := Song{Title: strings.TrimSpace(title)}
	if ok {
		for _, a := range strings.FieldsFunc(artists, func(r rune) bool { return r == ',' || r == '/' || r == '、' || r == '&' }) {
			if a = strings.TrimSpace(a); a != "" {
				s.Artists = append(s.Artists, a)
			}
		}
	}
	return s
}
