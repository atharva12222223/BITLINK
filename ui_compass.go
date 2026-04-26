package main

import (
	"fmt"
	"image/color"
	"math"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ─── Compass Tab ────────────────────────────────────────────────────────────

type compassTab struct {
	root    *fyne.Container
	compass *compassWidget
}

func buildCompassTab(window fyne.Window) *compassTab {
	t := &compassTab{}
	t.compass = newCompassWidget()

	header := sectionHeader(T("nav.compass"), T("hdr.compass_sub"))

	compassHolder := container.NewCenter(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(320, 320)), t.compass),
	)

	t.root = container.NewVBox(
		header, spacer(8),
		compassHolder,
	)
	return t
}

func (t *compassTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *compassTab) refresh() {}

// ─── Compass Widget ─────────────────────────────────────────────────────────

type compassWidget struct {
	widget.BaseWidget
	mu      sync.RWMutex
	bearing float64 // degrees clockwise from N
}

func newCompassWidget() *compassWidget {
	w := &compassWidget{}
	w.ExtendBaseWidget(w)
	return w
}

func (c *compassWidget) SetBearing(deg float64) {
	c.mu.Lock()
	c.bearing = math.Mod(deg, 360)
	if c.bearing < 0 {
		c.bearing += 360
	}
	c.mu.Unlock()
	c.Refresh()
}

func (c *compassWidget) Bearing() float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.bearing
}

// ─── Compass Renderer ───────────────────────────────────────────────────────

type compassRenderer struct {
	widget      *compassWidget
	outerRing   *canvas.Circle
	innerRing   *canvas.Circle
	centerDot   *canvas.Circle
	needle      *canvas.Line
	tail        *canvas.Line
	bearingText *canvas.Text
	dirTicks    []*canvas.Line
	dirLabels   []*canvas.Text
	degTicks    []*canvas.Line
	objs        []fyne.CanvasObject
}

func (c *compassWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &compassRenderer{widget: c}
	r.setupObjects()
	return r
}

func (r *compassRenderer) setupObjects() {
	r.outerRing = canvas.NewCircle(color.Transparent)
	r.outerRing.StrokeColor = colCyanDim
	r.outerRing.StrokeWidth = 2

	r.innerRing = canvas.NewCircle(color.Transparent)
	r.innerRing.StrokeColor = color.NRGBA{R: colCyan.R, G: colCyan.G, B: colCyan.B, A: 0x33}
	r.innerRing.StrokeWidth = 1

	r.centerDot = canvas.NewCircle(colCyan)
	r.centerDot.Resize(fyne.NewSize(8, 8))

	r.needle = canvas.NewLine(colSOSRed)
	r.needle.StrokeWidth = 3

	r.tail = canvas.NewLine(colCyanDim)
	r.tail.StrokeWidth = 2

	r.bearingText = canvas.NewText("0°", colCyan)
	r.bearingText.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	r.bearingText.Alignment = fyne.TextAlignCenter

	r.objs = []fyne.CanvasObject{
		r.outerRing, r.innerRing, r.centerDot, r.needle, r.tail, r.bearingText,
	}

	for i := 0; i < 8; i++ {
		t := canvas.NewLine(color.Transparent)
		l := canvas.NewText("", color.Transparent)
		r.dirTicks = append(r.dirTicks, t)
		r.dirLabels = append(r.dirLabels, l)
		r.objs = append(r.objs, t, l)
	}

	for i := 0; i < 12; i++ {
		t := canvas.NewLine(colHairline)
		t.StrokeWidth = 1
		r.degTicks = append(r.degTicks, t)
		r.objs = append(r.objs, t)
	}
}

func (r *compassRenderer) Destroy() {}

func (r *compassRenderer) Layout(size fyne.Size) {
	r.update(size)
}

func (r *compassRenderer) MinSize() fyne.Size {
	return fyne.NewSize(280, 280)
}

func (r *compassRenderer) Objects() []fyne.CanvasObject {
	return r.objs
}

func (r *compassRenderer) Refresh() {
	r.update(r.widget.Size())
	canvas.Refresh(r.widget)
}

func (r *compassRenderer) update(size fyne.Size) {
	if size.Width < 10 || size.Height < 10 {
		return
	}

	cx, cy := size.Width/2, size.Height/2
	radius := float32(math.Min(float64(cx), float64(cy))) - 20

	r.outerRing.Resize(fyne.NewSize(radius*2, radius*2))
	r.outerRing.Move(fyne.NewPos(cx-radius, cy-radius))

	innerR := radius * 0.7
	r.innerRing.Resize(fyne.NewSize(innerR*2, innerR*2))
	r.innerRing.Move(fyne.NewPos(cx-innerR, cy-innerR))

	r.centerDot.Move(fyne.NewPos(cx-4, cy-4))

	bearing := r.widget.Bearing()
	dirs := []struct {
		deg   float64
		label string
		col   color.NRGBA
		big   bool
	}{
		{0, "N", colSOSRed, true}, {45, "NE", colMuted, false},
		{90, "E", colTextHi, true}, {135, "SE", colMuted, false},
		{180, "S", colTextHi, true}, {225, "SW", colMuted, false},
		{270, "W", colTextHi, true}, {315, "NW", colMuted, false},
	}

	for i, d := range dirs {
		angleDeg := d.deg - bearing - 90
		angleRad := angleDeg * math.Pi / 180

		tickLen := float32(12)
		if d.big {
			tickLen = 20
		}
		outerX := cx + float32(math.Cos(angleRad))*radius
		outerY := cy + float32(math.Sin(angleRad))*radius
		innerX := cx + float32(math.Cos(angleRad))*(radius-tickLen)
		innerY := cy + float32(math.Sin(angleRad))*(radius-tickLen)

		t := r.dirTicks[i]
		t.StrokeColor = d.col
		t.StrokeWidth = 1
		if d.big {
			t.StrokeWidth = 2
		}
		t.Position1 = fyne.NewPos(outerX, outerY)
		t.Position2 = fyne.NewPos(innerX, innerY)

		labelR := radius - tickLen - 14
		lx := cx + float32(math.Cos(angleRad))*labelR - 8
		ly := cy + float32(math.Sin(angleRad))*labelR - 7

		l := r.dirLabels[i]
		l.Text = d.label
		l.Color = d.col
		l.TextSize = 14
		l.TextStyle = fyne.TextStyle{}
		if d.big {
			l.TextSize = 16
			l.TextStyle = fyne.TextStyle{Bold: true}
		}
		l.Move(fyne.NewPos(lx, ly))
	}

	tickIdx := 0
	for deg := float64(0); deg < 360; deg += 30 {
		if math.Mod(deg, 45) == 0 {
			continue // skip main dirs
		}
		if tickIdx >= len(r.degTicks) {
			break
		}
		angleDeg := deg - bearing - 90
		angleRad := angleDeg * math.Pi / 180
		outerX := cx + float32(math.Cos(angleRad))*radius
		outerY := cy + float32(math.Sin(angleRad))*radius
		innerX := cx + float32(math.Cos(angleRad))*(radius-8)
		innerY := cy + float32(math.Sin(angleRad))*(radius-8)

		t := r.degTicks[tickIdx]
		t.Position1 = fyne.NewPos(outerX, outerY)
		t.Position2 = fyne.NewPos(innerX, innerY)
		tickIdx++
	}

	needleRad := -90.0 * math.Pi / 180
	needleLen := radius * 0.55
	r.needle.Position1 = fyne.NewPos(cx, cy)
	r.needle.Position2 = fyne.NewPos(cx+float32(math.Cos(needleRad))*float32(needleLen), cy+float32(math.Sin(needleRad))*float32(needleLen))

	tailRad := 90.0 * math.Pi / 180
	tailLen := radius * 0.25
	r.tail.Position1 = fyne.NewPos(cx, cy)
	r.tail.Position2 = fyne.NewPos(cx+float32(math.Cos(tailRad))*float32(tailLen), cy+float32(math.Sin(tailRad))*float32(tailLen))

	r.bearingText.Text = fmt.Sprintf("%.0f°", bearing)
	r.bearingText.TextSize = 28
	r.bearingText.Resize(fyne.NewSize(size.Width, r.bearingText.MinSize().Height))
	r.bearingText.Move(fyne.NewPos(0, cy+radius+4))
}
