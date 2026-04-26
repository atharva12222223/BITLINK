package main

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type safetyTab struct {
	root    *fyne.Container
	resList *fyne.Container
	faCards *fyne.Container
	clCards *fyne.Container
	window  fyne.Window
}

func buildSafetyTab(window fyne.Window) *safetyTab {
	t := &safetyTab{window: window}

	header := sectionHeader(T("hdr.safety"), T("hdr.safety_sub"))

	// Search bar
	searchEntry := widget.NewEntry()
	searchEntry.SetPlaceHolder(T("safety.search"))

	t.faCards = container.New(layout.NewGridLayout(2))
	t.clCards = container.New(layout.NewGridLayout(2))

	rebuildCards := func(query string) {
		t.faCards.Objects = nil
		for _, e := range FirstAidLibrary {
			if query != "" && !containsCI(T(e.Title), query) && !containsCI(T(e.Tag), query) {
				continue
			}
			t.faCards.Add(t.firstAidCard(e))
		}
		t.faCards.Refresh()

		t.clCards.Objects = nil
		for _, c := range SurvivalChecklists {
			if query != "" && !containsCI(T(c.Title), query) && !containsCI(T(c.Tag), query) {
				continue
			}
			t.clCards.Add(t.checklistCard(c))
		}
		t.clCards.Refresh()
	}

	rebuildCards("")
	searchEntry.OnChanged = func(s string) { rebuildCards(s) }

	t.root = container.NewVBox(
		header, spacer(8),
		searchEntry,
		spacer(8),
		container.NewHBox(
			canvas.NewText("◇", colSOSRed), spacer(8),
			textBold(T("safety.first_aid"), colTextHi, 16),
		),
		spacer(6),
		t.faCards,
		spacer(14),
		container.NewHBox(
			canvas.NewText("◇", colAmber), spacer(8),
			textBold(T("safety.checklists"), colTextHi, 16),
		),
		spacer(6),
		t.clCards,
	)

	t.refresh()
	Safety.SetListener(func() { fyne.Do(t.refresh) })
	return t
}

func (t *safetyTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *safetyTab) refresh() {
}


func (t *safetyTab) firstAidCard(e FirstAidEntry) fyne.CanvasObject {
	title := canvas.NewText(T(e.Title), colTextHi)
	title.TextSize = 15
	title.TextStyle = fyne.TextStyle{Bold: true}
	tag := pill(T(e.Tag), colSOSRed, color.NRGBA{R: colSOSRed.R, G: colSOSRed.G, B: colSOSRed.B, A: 0x22})

	steps := container.NewVBox()
	for i, s := range e.Steps {
		num := canvas.NewText(fmt.Sprintf("%02d", i+1), colSOSRed)
		num.TextSize = 11
		num.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
		body := widget.NewLabel(T(s))
		body.Wrapping = fyne.TextWrapWord
		row := container.NewBorder(nil, nil, container.New(layout.NewGridWrapLayout(fyne.NewSize(28, 18)), num), nil, body)
		steps.Add(row)
	}

	return container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4),
		newGlowCard(container.NewVBox(
			container.NewBorder(nil, nil, title, tag),
			spacer(4),
			steps,
		), 14, colSOSRed),
	)
}

func (t *safetyTab) checklistCard(c Checklist) fyne.CanvasObject {
	title := canvas.NewText(T(c.Title), colTextHi)
	title.TextSize = 15
	title.TextStyle = fyne.TextStyle{Bold: true}
	tag := pill(T(c.Tag), colAmber, color.NRGBA{R: colAmber.R, G: colAmber.G, B: colAmber.B, A: 0x22})

	items := container.NewVBox()
	for _, s := range c.Items {
		check := widget.NewCheck(T(s), nil)
		items.Add(check)
	}

	return container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4),
		newGlowCard(container.NewVBox(
			container.NewBorder(nil, nil, title, tag),
			spacer(4),
			items,
		), 14, colAmber),
	)
}

func containsCI(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
