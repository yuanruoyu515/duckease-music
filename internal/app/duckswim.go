package app

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The duck of the app's icon swims across a pond as the songs of a
// Spotify playlist are found on NetEase, one by one: how far it got is
// how far the matching is.

// The duck alone, facing right, from icon/duck/duck-round.svg, in
// layers: an icon is drawn in one color, and the duck has four.
func duckLayer(paths string) *ui.SVG {
	return ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="110 140 800 730">` + paths + `</svg>`))
}

const (
	duckBodyPath = `M160 525C200 530 220 560 258 578C330 575 410 560 425 505C435 460 405 420 410 340C420 250 495 205 595 205C690 205 760 275 768 380L752 470C745 500 735 525 745 555C790 650 790 760 640 815C520 855 330 850 230 805C130 760 120 660 192 607C165 590 140 560 150 535Z`
	duckBeakPath = `M764 380C812 385 860 402 885 430C852 458 805 470 756 465Z`
)

var (
	duckBody  = duckLayer(`<path d="` + duckBodyPath + `" fill="#000"/>`)
	duckBeak  = duckLayer(`<path d="` + duckBeakPath + `" fill="#000"/>`)
	duckCheek = duckLayer(`<ellipse cx="640" cy="418" rx="25" ry="13" fill="#000"/>`)
	duckLines = duckLayer(`<g stroke="#000" stroke-width="30" stroke-linecap="round" stroke-linejoin="round" fill="none">
<path d="M590 200C585 182 572 170 556 160M596 200C596 182 604 168 616 158"/>
<path d="` + duckBodyPath + `"/><path d="` + duckBeakPath + `"/>
<path d="M330 690C380 740 520 745 570 700" stroke-width="24"/></g>
<ellipse cx="667" cy="348" rx="13" ry="17" fill="#000"/>`)
)

// drawDuck draws the duck in r, in its colors.
func drawDuck(pt *ui.Painter, r ui.Rect) {
	pt.Icon(duckBody, r, ui.Hex("#FCFCFC"))
	pt.Icon(duckBeak, r, ui.Hex("#FFB547"))
	pt.Icon(duckCheek, r, ui.Hex("#FFD5C2"))
	pt.Icon(duckLines, r, ui.Hex("#30363B"))
}

// Sizes of the pond.
const (
	pondWidth  = 360
	pondHeight = 64
	duckSize   = 40
)

// spotLoading shows a Spotify playlist being read and its songs found:
// the duck swimming, and what is going on under it.
func (a *app) spotLoading(c *ui.Context, pg *spotPage) {
	p := a.pal
	frac, label, note := float32(0), "正在读取 Spotify 歌单…", ""
	if pg != nil {
		switch {
		case !pg.resume.IsZero():
			left := max(time.Until(pg.resume), 0)
			label = fmt.Sprintf("网易云说搜得太快了，%d 秒后接着匹配", int(left.Seconds())+1)
			c.After(time.Second)
		case pg.total > 0 && pg.done < pg.total:
			label = fmt.Sprintf("正在网易云匹配歌曲 %d / %d", pg.done, pg.total)
		case pg.total > 0:
			label = pg.progress
		}
		if pg.total > 0 {
			frac = float32(pg.done) / float32(pg.total)
			note = "每首歌之间会停一下，免得触发网易云风控；找到的会记下来，下次打开就快了"
		}
	}
	top, bottom := a.insets(c)
	ui.Column(c).Grow(1).Padding(top, 0, bottom, 0).Center().Gap(14).Children(func() {
		pond := ui.Box(c.Key("pond")).Size(pondWidth, pondHeight).Label(label)
		at := pond.AnimateTo("at", frac, ui.Animation{Duration: 600 * time.Millisecond})
		resting := pg != nil && !pg.resume.IsZero()
		// Water is blue in either theme; the pond not yet swum, a hairline.
		water, bg := ui.Hex("#3D9BF0"), p.hairline
		pond.Draw(func(pt *ui.Painter, r ui.Rect) {
			still := pt.Preferences().ReduceMotion
			t := float64(pt.Now().UnixNano()) / 1e9
			if still {
				t = 0
			}
			line := r.Y + r.H - 16
			// The pond, and the water swum through, rippling.
			pt.Fill(ui.Rect{X: r.X, Y: line - 1.5, W: r.W, H: 3}, bg, 1.5)
			x := r.X + at*(r.W-duckSize)
			swum := x - r.X + duckSize*0.35
			if swum > 0 {
				var pts [][2]float32
				for dx := float32(0); dx <= swum; dx += 3 {
					y := line + float32(math.Sin(float64(dx)/9-t*3))*1.4
					pts = append(pts, [2]float32{r.X + dx, y})
				}
				pt.StrokePolyline(pts, 3, water)
			}
			// The wake: little arcs behind the duck, drifting back and
			// fading.
			if !resting {
				for i := range 3 {
					phase := float32(math.Mod(t*0.9+float64(i)/3, 1))
					wx := x + duckSize*0.15 - phase*34
					if wx < r.X {
						continue
					}
					alpha := (1 - phase) * 0.9
					w := 6 + phase*8
					pt.StrokePolyline([][2]float32{{wx, line - 4 - w/3}, {wx - w/2, line - 2}, {wx, line}}, 1.6, water.Alpha(0.4*alpha))
				}
			}
			// The duck, bobbing, and leaning into each stroke.
			bob := float32(math.Sin(t*2*math.Pi/1.3)) * 2.2
			lean := float32(math.Sin(t*2*math.Pi/1.3+0.8)) * 4
			if resting {
				bob, lean = bob*0.4, 0
			}
			box := ui.Rect{X: x, Y: line - duckSize + 13 + bob, W: duckSize, H: duckSize}
			pt.Transform(ui.Rotation(lean), box.X+box.W/2, box.Y+box.H, func() {
				drawDuck(pt, box)
			})
			if !still {
				pt.AnimationFrame()
			}
		})
		ui.Text(c, label).FontSize(13).TextColor(p.muted).FontFeatures("tnum")
		if note != "" {
			ui.Text(c, note).FontSize(11.5).TextColor(p.faint)
		}
	})
}
