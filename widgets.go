package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// tappableArea — a transparent widget that captures Tap events without
// painting any background. Implements desktop.Hoverable so callers can
// react to mouse-in / mouse-out (used to drive a separate hover overlay).
type tappableArea struct {
	widget.BaseWidget
	onTap   func()
	onHover func(bool)
	rect    *canvas.Rectangle
}

func newTappableArea(onTap func()) *tappableArea {
	t := &tappableArea{
		onTap: onTap,
		rect:  canvas.NewRectangle(color.Transparent),
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tappableArea) Tapped(*fyne.PointEvent)          { if t.onTap != nil { t.onTap() } }
func (t *tappableArea) TappedSecondary(*fyne.PointEvent) {}

// desktop.Hoverable
func (t *tappableArea) MouseIn(*desktop.MouseEvent) {
	if t.onHover != nil {
		t.onHover(true)
	}
}
func (t *tappableArea) MouseMoved(*desktop.MouseEvent) {}
func (t *tappableArea) MouseOut() {
	if t.onHover != nil {
		t.onHover(false)
	}
}

// desktop.Cursorable — show pointer cursor on hover
func (t *tappableArea) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *tappableArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.rect)
}

func (t *tappableArea) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

// Compile-time interface assertions
var (
	_ fyne.Tappable      = (*tappableArea)(nil)
	_ desktop.Hoverable  = (*tappableArea)(nil)
	_ desktop.Cursorable = (*tappableArea)(nil)
)
