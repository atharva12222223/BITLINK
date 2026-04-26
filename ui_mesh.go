package main

import (
	"fmt"
	"image/color"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type meshTab struct {
	root          *fyne.Container
	canvas        *meshCanvas
	info          *widget.Label
	stats         *canvas.Text
	nodePositions map[string]fyne.Position
	sweepAngle    float64
}

type meshCanvas struct {
	widget.BaseWidget
	tab *meshTab
}

func newMeshCanvas() *meshCanvas {
	c := &meshCanvas{}
	c.ExtendBaseWidget(c)
	return c
}

func (c *meshCanvas) MinSize() fyne.Size               { return fyne.NewSize(420, 380) }
func (c *meshCanvas) CreateRenderer() fyne.WidgetRenderer { return &meshRenderer{c: c} }
func (c *meshCanvas) Tapped(ev *fyne.PointEvent) {
	if c.tab != nil {
		c.tab.handleTap(ev.Position)
	}
}

type meshRenderer struct {
	c       *meshCanvas
	objects []fyne.CanvasObject
}

func (r *meshRenderer) Destroy() {}
func (r *meshRenderer) Layout(size fyne.Size) {
	r.objects = r.buildObjects(size)
}
func (r *meshRenderer) MinSize() fyne.Size { return r.c.MinSize() }
func (r *meshRenderer) Objects() []fyne.CanvasObject {
	if r.objects == nil {
		r.objects = r.buildObjects(r.c.Size())
	}
	return r.objects
}
func (r *meshRenderer) Refresh() {
	r.objects = r.buildObjects(r.c.Size())
	canvas.Refresh(r.c)
}

func (r *meshRenderer) buildObjects(size fyne.Size) []fyne.CanvasObject {
	objs := []fyne.CanvasObject{}

	bg := canvas.NewRectangle(colBg)
	bg.Resize(size)
	objs = append(objs, bg)

	// Concentric range rings — radar feel
	cx, cy := size.Width/2, size.Height/2
	maxR := float32(math.Min(float64(size.Width), float64(size.Height))/2) - 30
	for i := 1; i <= 4; i++ {
		ring := canvas.NewCircle(color.Transparent)
		ring.StrokeColor = color.NRGBA{R: 0x10, G: 0x2A, B: 0x44, A: 0xFF}
		ring.StrokeWidth = 1
		rad := maxR * float32(i) / 4
		ring.Resize(fyne.NewSize(rad*2, rad*2))
		ring.Move(fyne.NewPos(cx-rad, cy-rad))
		objs = append(objs, ring)
	}

	// Cross-hairs
	hLine := canvas.NewLine(color.NRGBA{R: 0x10, G: 0x2A, B: 0x44, A: 0xFF})
	hLine.StrokeWidth = 1
	hLine.Position1 = fyne.NewPos(20, cy)
	hLine.Position2 = fyne.NewPos(size.Width-20, cy)
	objs = append(objs, hLine)
	vLine := canvas.NewLine(color.NRGBA{R: 0x10, G: 0x2A, B: 0x44, A: 0xFF})
	vLine.StrokeWidth = 1
	vLine.Position1 = fyne.NewPos(cx, 20)
	vLine.Position2 = fyne.NewPos(cx, size.Height-20)
	objs = append(objs, vLine)

	// Animated sweep line
	if r.c.tab != nil {
		ang := r.c.tab.sweepAngle
		ex := cx + float32(math.Cos(ang))*maxR
		ey := cy + float32(math.Sin(ang))*maxR
		sweep := canvas.NewLine(colCyan)
		sweep.StrokeWidth = 2
		sweep.Position1 = fyne.NewPos(cx, cy)
		sweep.Position2 = fyne.NewPos(ex, ey)
		objs = append(objs, sweep)
		// Trailing fade segments
		for i := 1; i <= 6; i++ {
			a := ang - float64(i)*0.05
			tx := cx + float32(math.Cos(a))*maxR
			ty := cy + float32(math.Sin(a))*maxR
			seg := canvas.NewLine(color.NRGBA{R: colCyan.R, G: colCyan.G, B: colCyan.B, A: byte(0xC0 - i*0x18)})
			seg.StrokeWidth = 2
			seg.Position1 = fyne.NewPos(cx, cy)
			seg.Position2 = fyne.NewPos(tx, ty)
			objs = append(objs, seg)
		}
	}

	nodes := TopologySnapshot()

	if r.c.tab != nil {
		r.c.tab.nodePositions = map[string]fyne.Position{}
	}

	// Self core
	selfHalo := canvas.NewCircle(colCyanGlow)
	selfHalo.Resize(fyne.NewSize(32, 32))
	selfHalo.Move(fyne.NewPos(cx-16, cy-16))
	objs = append(objs, selfHalo)
	selfDot := canvas.NewCircle(colCyan)
	selfDot.StrokeColor = colTextHi
	selfDot.StrokeWidth = 2
	selfDot.Resize(fyne.NewSize(16, 16))
	selfDot.Move(fyne.NewPos(cx-8, cy-8))
	objs = append(objs, selfDot)

	selfLabel := canvas.NewText(displayName(), colTextHi)
	selfLabel.TextSize = 11
	selfLabel.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	selfLabel.Move(fyne.NewPos(cx+14, cy-6))
	objs = append(objs, selfLabel)

	// Peers radial
	for i, n := range nodes {
		denom := len(nodes)
		if denom < 1 {
			denom = 1
		}
		angle := 2*math.Pi*float64(i)/float64(denom) - math.Pi/2
		// Use RSSI to set radial distance
		pct := rssiToPct(n.RSSI)
		distRatio := 0.85
		if pct > 0 {
			distRatio = 0.35 + (1-float64(pct)/100)*0.55
		}
		radius := maxR * float32(distRatio)
		nx := cx + float32(math.Cos(angle))*radius
		ny := cy + float32(math.Sin(angle))*radius

		lineCol := color.NRGBA{R: colCyan.R, G: colCyan.G, B: colCyan.B, A: 0x55}
		if !n.Connected {
			lineCol = color.NRGBA{R: colMuted.R, G: colMuted.G, B: colMuted.B, A: 0x44}
		}
		line := canvas.NewLine(lineCol)
		line.StrokeWidth = 2
		line.Position1 = fyne.NewPos(cx, cy)
		line.Position2 = fyne.NewPos(nx, ny)
		objs = append(objs, line)

		// Halo if connected
		if n.Connected {
			halo := canvas.NewCircle(colCyanGlow)
			halo.Resize(fyne.NewSize(28, 28))
			halo.Move(fyne.NewPos(nx-14, ny-14))
			objs = append(objs, halo)
		}

		nodeC := canvas.NewCircle(colPanel2)
		if n.Connected {
			nodeC.StrokeColor = colCyan
		} else {
			nodeC.StrokeColor = colMuted
		}
		nodeC.StrokeWidth = 2
		nodeC.Resize(fyne.NewSize(14, 14))
		nodeC.Move(fyne.NewPos(nx-7, ny-7))
		objs = append(objs, nodeC)

		nick := peerDisplayName(n.Address)
		lbl := canvas.NewText(nick, colTextHi)
		lbl.TextSize = 10
		lbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
		lbl.Move(fyne.NewPos(nx+12, ny-6))
		objs = append(objs, lbl)

		rssiLbl := canvas.NewText(fmt.Sprintf("%d%%", pct), colMuted)
		rssiLbl.TextSize = 9
		rssiLbl.TextStyle = fyne.TextStyle{Monospace: true}
		rssiLbl.Move(fyne.NewPos(nx+12, ny+8))
		objs = append(objs, rssiLbl)

		if r.c.tab != nil {
			r.c.tab.nodePositions[n.Address] = fyne.NewPos(nx, ny)
		}
	}

	return objs
}

func buildMeshTab(window fyne.Window) *meshTab {
	t := &meshTab{nodePositions: map[string]fyne.Position{}}
	t.canvas = newMeshCanvas()
	t.canvas.tab = t
	t.info = widget.NewLabel("Tap a node on the radar to inspect.\nDistance from center reflects relative signal strength.")
	t.info.Wrapping = fyne.TextWrapWord
	t.stats = canvas.NewText("nodes 0 · linked 0 · packets seen 0", colCyan)
	t.stats.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	t.stats.TextSize = 11

	header := sectionHeader(T("hdr.mesh"), T("hdr.mesh_sub"))

	statsCard := newGlowCard(container.NewBorder(nil, nil,
		container.NewHBox(canvas.NewText("◉", colGreen), spacer(6), t.stats),
		pill("LIVE", colCyan, colCyanFaint),
	), 10, colCyan)

	t.root = container.NewVBox(
		header, spacer(8),
		newCardPanel(t.canvas, 8),
		spacer(8), statsCard,
		spacer(8),
		newCardPanel(t.info, 14),
	)

	safeGo("mesh-tick", t.tick)
	return t
}

func (t *meshTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *meshTab) tick() {
	for {
		time.Sleep(80 * time.Millisecond)
		t.sweepAngle += 0.15
		if t.sweepAngle > 2*math.Pi {
			t.sweepAngle -= 2 * math.Pi
		}
		fyne.Do(func() {
			snap := TopologySnapshot()
			seenMu.Lock()
			pkts := len(seenIDs)
			seenMu.Unlock()
			t.stats.Text = fmt.Sprintf("nodes %d · linked %d · packets seen %d", len(snap), ActivePeerCount(), pkts)
			t.stats.Refresh()
			t.canvas.Refresh()
		})
	}
}

func (t *meshTab) handleTap(p fyne.Position) {
	if t.nodePositions == nil {
		return
	}
	bestAddr := ""
	bestDist := float32(20)
	for addr, np := range t.nodePositions {
		dx := p.X - np.X
		dy := p.Y - np.Y
		d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if d < bestDist {
			bestDist = d
			bestAddr = addr
		}
	}
	if bestAddr == "" {
		return
	}
	n := topoNode(bestAddr)
	if n == nil {
		return
	}
	nick := n.Name
	if c, ok := Contacts.Get(bestAddr); ok && c.Nickname != "" {
		nick = c.Nickname
	}
	if nick == "" {
		nick = "(unknown)"
	}
	dialog.ShowInformation("Node",
		fmt.Sprintf("Address:   %s\nNickname:  %s\nRSSI:      %d dBm (%d%%)\nLast seen: %s\nLinked:    %v",
			bestAddr, nick, n.RSSI, rssiToPct(n.RSSI), relTime(n.LastSeen), n.Connected),
		fyne.CurrentApp().Driver().AllWindows()[0],
	)
}
