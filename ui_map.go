package main

import (
	"fmt"
	"image/color"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type mapTab struct {
	root    *fyne.Container
	canvas  *mapCanvas
	pinList *fyne.Container
	window  fyne.Window
}

type mapCanvas struct {
	widget.BaseWidget
	tab    *mapTab
	zoom   float32
	offset fyne.Position
}

func newMapCanvas() *mapCanvas {
	c := &mapCanvas{zoom: 1.0}
	c.ExtendBaseWidget(c)
	return c
}

func (c *mapCanvas) MinSize() fyne.Size               { return fyne.NewSize(560, 420) }
func (c *mapCanvas) CreateRenderer() fyne.WidgetRenderer {
	r := &mapRenderer{c: c, root: container.NewWithoutLayout()}
	r.Refresh()
	return r
}
func (c *mapCanvas) Tapped(ev *fyne.PointEvent) {
	if c.tab == nil {
		return
	}
	c.tab.handleTap(ev.Position, c.Size(), c.zoom, c.offset)
}

func (c *mapCanvas) Scrolled(ev *fyne.ScrollEvent) {
	delta := ev.Scrolled.DY / 20
	oldZoom := c.zoom
	c.zoom += delta
	if c.zoom < 0.2 {
		c.zoom = 0.2
	}
	if c.zoom > 10.0 {
		c.zoom = 10.0
	}

	// Adjust offset to zoom toward mouse position
	if oldZoom != c.zoom {
		c.Refresh()
	}
}

func (c *mapCanvas) Dragged(ev *fyne.DragEvent) {
	c.offset.X += ev.Dragged.DX
	c.offset.Y += ev.Dragged.DY
	c.Refresh()
}

func (c *mapCanvas) DragEnd() {}

type mapRenderer struct {
	c    *mapCanvas
	root *fyne.Container
}

func (r *mapRenderer) Destroy() {}
func (r *mapRenderer) Layout(size fyne.Size) {
	r.root.Resize(size)
	r.Refresh()
}
func (r *mapRenderer) MinSize() fyne.Size { return r.c.MinSize() }
func (r *mapRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.root}
}
func (r *mapRenderer) Refresh() {
	size := r.c.Size()
	if size.Width <= 0 || size.Height <= 0 {
		size = r.MinSize()
	}
	r.root.Objects = r.buildObjects(size, r.c.zoom, r.c.offset)
	r.root.Refresh()
	canvas.Refresh(r.c)
}

func (r *mapRenderer) buildObjects(size fyne.Size, zoom float32, offset fyne.Position) []fyne.CanvasObject {
	objs := []fyne.CanvasObject{}

	// Center of the coordinate system
	cx, cy := size.Width/2+offset.X, size.Height/2+offset.Y
	scaledW, scaledH := size.Width*zoom, size.Height*zoom

	// Background image (Tactical Map of India)
	img := canvas.NewImageFromFile("map_india.png")
	img.FillMode = canvas.ImageFillStretch
	img.Resize(fyne.NewSize(scaledW, scaledH))
	img.Move(fyne.NewPos(cx-scaledW/2, cy-scaledH/2))
	objs = append(objs, img)

	// Grid (scales with zoom)
	gridCol := color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x18}
	step := float32(40) * zoom
	if step < 10 {
		step = 10
	}
	// Calculate grid boundaries
	for x := cx - scaledW/2; x <= cx+scaledW/2; x += step {
		if x < 0 || x > size.Width {
			continue
		}
		l := canvas.NewLine(gridCol)
		l.StrokeWidth = 0.5
		l.Position1 = fyne.NewPos(x, fyne.Max(0, cy-scaledH/2))
		l.Position2 = fyne.NewPos(x, fyne.Min(size.Height, cy+scaledH/2))
		objs = append(objs, l)
	}
	for y := cy - scaledH/2; y <= cy+scaledH/2; y += step {
		if y < 0 || y > size.Height {
			continue
		}
		l := canvas.NewLine(gridCol)
		l.StrokeWidth = 0.5
		l.Position1 = fyne.NewPos(fyne.Max(0, cx-scaledW/2), y)
		l.Position2 = fyne.NewPos(fyne.Min(size.Width, cx+scaledW/2), y)
		objs = append(objs, l)
	}

	// Center cross-hair (world center)
	hLine := canvas.NewLine(color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x44})
	hLine.StrokeWidth = 1
	hLine.Position1 = fyne.NewPos(fyne.Max(0, cx-20*zoom), cy)
	hLine.Position2 = fyne.NewPos(fyne.Min(size.Width, cx+20*zoom), cy)
	objs = append(objs, hLine)
	vLine := canvas.NewLine(color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x44})
	vLine.StrokeWidth = 1
	vLine.Position1 = fyne.NewPos(cx, fyne.Max(0, cy-20*zoom))
	vLine.Position2 = fyne.NewPos(cx, fyne.Min(size.Height, cy+20*zoom))
	objs = append(objs, vLine)

	// Self marker (always at world center)
	selfHalo := canvas.NewCircle(colCyanGlow)
	shSize := float32(28) * zoom
	if shSize < 10 {
		shSize = 10
	}
	selfHalo.Resize(fyne.NewSize(shSize, shSize))
	selfHalo.Move(fyne.NewPos(cx-shSize/2, cy-shSize/2))
	objs = append(objs, selfHalo)

	selfDot := canvas.NewCircle(colCyan)
	sdSize := float32(12) * zoom
	if sdSize < 4 {
		sdSize = 4
	}
	selfDot.StrokeColor = colTextHi
	selfDot.StrokeWidth = 1
	selfDot.Resize(fyne.NewSize(sdSize, sdSize))
	selfDot.Move(fyne.NewPos(cx-sdSize/2, cy-sdSize/2))
	objs = append(objs, selfDot)

	// Pins
	for _, p := range Safety.Pins() {
		col := pinKindColor(p.Kind)
		px := cx + float32(p.X/100)*(scaledW/2-20*zoom)
		py := cy + float32(p.Y/100)*(scaledH/2-20*zoom)

		if px < 0 || px > size.Width || py < 0 || py > size.Height {
			continue
		}

		haloSize := float32(22) * zoom
		if haloSize > 4 {
			halo := canvas.NewCircle(color.NRGBA{R: col.R, G: col.G, B: col.B, A: 0x35})
			halo.Resize(fyne.NewSize(haloSize, haloSize))
			halo.Move(fyne.NewPos(px-haloSize/2, py-haloSize/2))
			objs = append(objs, halo)
		}

		dotSize := float32(12) * zoom
		if dotSize > 2 {
			dot := canvas.NewCircle(col)
			dot.StrokeColor = colTextHi
			dot.StrokeWidth = 1
			dot.Resize(fyne.NewSize(dotSize, dotSize))
			dot.Move(fyne.NewPos(px-dotSize/2, py-dotSize/2))
			objs = append(objs, dot)
		}

		if zoom > 0.8 {
			lbl := canvas.NewText(p.Title, colTextHi)
			lbl.TextSize = 10 * zoom
			if lbl.TextSize > 12 {
				lbl.TextSize = 12
			}
			lbl.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
			lbl.Move(fyne.NewPos(px+dotSize/2+2, py-dotSize/2))
			objs = append(objs, lbl)
		}
	}
	return objs
}

func pinKindColor(k string) color.NRGBA {
	switch k {
	case "safe":
		return colGreen
	case "danger":
		return colSOSRed
	case "supply":
		return colAmber
	case "shelter":
		return colCyan
	case "water":
		return colCyan
	default:
		return colViolet
	}
}

func buildMapTab(window fyne.Window) *mapTab {
	t := &mapTab{window: window}
	t.canvas = newMapCanvas()
	t.canvas.tab = t

	header := sectionHeader(T("hdr.map"), T("hdr.map_sub"))

	addBtn := widget.NewButtonWithIcon("Drop Pin (center)", theme.ContentAddIcon(), func() {
		t.showAddPin(window, 0, 0)
	})
	addBtn.Importance = widget.HighImportance

	clearBtn := widget.NewButton("Clear all pins", func() {
		dialog.ShowConfirm("Clear pins?", "Remove every pin from the board?", func(ok bool) {
			if !ok {
				return
			}
			for _, p := range Safety.Pins() {
				Safety.DeletePin(p.ID)
			}
		}, window)
	})

	legend := container.NewHBox(
		legendDot("Safe", colGreen), spacer(10),
		legendDot("Danger", colSOSRed), spacer(10),
		legendDot("Supply", colAmber), spacer(10),
		legendDot("Shelter", colCyan), spacer(10),
		legendDot("Other", colViolet),
	)

	help := canvas.NewText("Tap anywhere on the grid to drop a marker. Coords are relative to YOU at center (-100..100 each axis).", colMuted)
	help.TextSize = 11

	t.pinList = container.NewVBox()

	t.root = container.NewVBox(
		header, spacer(8),
		newGlowCard(container.NewVBox(
			container.NewHBox(addBtn, clearBtn, layout.NewSpacer(), legend),
			spacer(6),
			help,
			spacer(8),
			t.canvas,
		), 14, colCyan),
		spacer(14),
		container.NewHBox(
			canvas.NewText("◇", colCyan), spacer(8),
			textBold("Pin Index", colTextHi, 16),
		),
		spacer(6),
		t.pinList,
	)

	t.refresh()
	Safety.SetListener(func() { fyne.Do(t.refresh) })
	return t
}

func legendDot(label string, c color.NRGBA) fyne.CanvasObject {
	return container.NewHBox(statusDot(c, 8), spacer(4),
		func() fyne.CanvasObject {
			t := canvas.NewText(label, colTextMid)
			t.TextSize = 10
			t.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
			return t
		}())
}

func (t *mapTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *mapTab) refresh() {
	t.pinList.Objects = nil
	pins := Safety.Pins()
	sort.Slice(pins, func(i, j int) bool { return pins[i].At.After(pins[j].At) })
	if len(pins) == 0 {
		t.pinList.Add(emptyState("No pins yet", "Tap on the grid to drop the first marker."))
	}
	for _, p := range pins {
		t.pinList.Add(t.pinRow(p))
	}
	t.canvas.Refresh()
	t.pinList.Refresh()
}

func (t *mapTab) pinRow(p MapPin) fyne.CanvasObject {
	col := pinKindColor(p.Kind)
	dot := statusDot(col, 12)

	title := canvas.NewText(p.Title, colTextHi)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}

	meta := canvas.NewText(fmt.Sprintf("%s · (%.0f, %.0f) · %s", p.Kind, p.X, p.Y, relTime(p.At)), colMuted)
	meta.TextSize = 11
	meta.TextStyle = fyne.TextStyle{Monospace: true}

	note := widget.NewLabel(p.Note)
	note.Wrapping = fyne.TextWrapWord
	if p.Note == "" {
		note.SetText("")
	}

	delBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		Safety.DeletePin(p.ID)
	})

	left := container.NewHBox(container.NewCenter(dot), spacer(10), container.NewVBox(title, meta, note))
	row := container.NewBorder(nil, nil, left, delBtn)
	return newCardPanel(row, 12)
}

func (t *mapTab) showAddPin(window fyne.Window, x, y float64) {
	title := widget.NewEntry()
	title.SetPlaceHolder("Title (e.g. Safe house, Sniper line, Cache)")
	kind := widget.NewSelect([]string{"safe", "danger", "supply", "shelter", "water", "other"}, nil)
	kind.SetSelected("safe")
	note := widget.NewMultiLineEntry()
	note.SetMinRowsVisible(2)
	note.SetPlaceHolder("Notes / coords / instructions")

	dialog.ShowForm(fmt.Sprintf("Drop Pin (%.0f, %.0f)", x, y), "Drop", "Cancel",
		[]*widget.FormItem{
			{Text: "Title", Widget: title},
			{Text: "Kind", Widget: kind},
			{Text: "Note", Widget: note},
		},
		func(ok bool) {
			if !ok || title.Text == "" {
				return
			}
			Safety.AddPin(MapPin{
				Title: title.Text,
				Kind:  kind.Selected,
				Note:  note.Text,
				X:     x,
				Y:     y,
			})
		}, window)
}

func (t *mapTab) handleTap(p fyne.Position, size fyne.Size, zoom float32, offset fyne.Position) {
	cx, cy := size.Width/2+offset.X, size.Height/2+offset.Y
	scaledW, scaledH := size.Width*zoom, size.Height*zoom

	// Calculate world coordinates (-100 to 100)
	denomX := scaledW/2 - 20*zoom
	denomY := scaledH/2 - 20*zoom
	if denomX == 0 {
		denomX = 1
	}
	if denomY == 0 {
		denomY = 1
	}

	rx := float64((p.X - cx) / denomX * 100)
	ry := float64((p.Y - cy) / denomY * 100)
	if rx < -100 {
		rx = -100
	}
	if rx > 100 {
		rx = 100
	}
	if ry < -100 {
		ry = -100
	}
	if ry > 100 {
		ry = 100
	}

	win := t.window
	if win == nil {
		win = fyne.CurrentApp().Driver().AllWindows()[0]
	}
	t.showAddPin(win, rx, ry)
}
