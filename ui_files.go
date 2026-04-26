package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

type filesTab struct {
	root *fyne.Container
	list *fyne.Container
}

func buildFilesTab(window fyne.Window) *filesTab {
	t := &filesTab{}

	header := sectionHeader("Files", "OFFLINE TRANSFER")

	progress := widget.NewProgressBar()
	progress.Hide()

	pickPeer := widget.NewSelect([]string{"(broadcast)"}, nil)
	pickPeer.SetSelected("(broadcast)")
	refreshPeers := func() {
		opts := []string{"(broadcast)"}
		for _, c := range Contacts.All() {
			opts = append(opts, c.Nickname+" — "+c.Address)
		}
		for _, n := range TopologySnapshot() {
			if _, ok := Contacts.Get(n.Address); ok {
				continue
			}
			opts = append(opts, "peer — "+n.Address)
		}
		pickPeer.Options = opts
		pickPeer.Refresh()
	}
	refreshPeers()

	sendBtn := widget.NewButtonWithIcon("Pick & Send File", nil, func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			path := reader.URI().Path()
			peer := ""
			if pickPeer.Selected != "" && pickPeer.Selected != "(broadcast)" {
				s := pickPeer.Selected
				for i := len(s) - 1; i > 0; i-- {
					if s[i] == ' ' {
						peer = s[i+1:]
						break
					}
				}
			}
			progress.SetValue(0)
			progress.Show()
			safeGo("file-send", func() {
				err := Files.SendFile(peer, path, func(p float64) {
					fyne.Do(func() { progress.SetValue(p) })
				})
				fyne.Do(func() {
					progress.Hide()
					if err != nil {
						dialog.ShowError(err, window)
					} else {
						dialog.ShowInformation("File Sent", "Transfer complete.", window)
					}
					t.refresh()
				})
			})
		}, window)
		fd.Show()
	})
	sendBtn.Importance = widget.HighImportance

	limitNote := canvas.NewText("Max 500 KB · 180-byte chunks · 15ms throttle per packet", colMuted)
	limitNote.TextSize = 10
	limitNote.TextStyle = fyne.TextStyle{Monospace: true}

	sendCard := newGlowCard(container.NewVBox(
		container.NewHBox(
			canvas.NewText("📤", colCyan),
			spacer(8),
			func() fyne.CanvasObject {
				t := canvas.NewText("Send File", colTextHi)
				t.TextSize = 14
				t.TextStyle = fyne.TextStyle{Bold: true}
				return t
			}(),
			layout.NewSpacer(),
			pill("BLE", colCyan, colCyanFaint),
		),
		spacer(8),
		pickPeer,
		sendBtn,
		progress,
		limitNote,
	), 14, colCyan)

	t.list = container.NewVBox()
	t.root = container.NewVBox(
		header,
		spacer(8),
		sendCard,
		spacer(14),
		subHeader("Transfer History"),
		t.list,
	)

	Files.SetListener(func() { fyne.Do(t.refresh) })
	t.refresh()
	return t
}

func (t *filesTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *filesTab) refresh() {
	t.list.Objects = nil
	items := Files.Items()
	sort.Slice(items, func(i, j int) bool { return items[i].Time.After(items[j].Time) })
	if len(items) == 0 {
		t.list.Add(emptyState("No transfers yet", "Pick a file above to send it over the mesh."))
		t.list.Refresh()
		return
	}
	for _, e := range items {
		t.list.Add(t.fileRow(e))
	}
	t.list.Refresh()
}

func fileTypeBadge(name string) fyne.CanvasObject {
	ext := strings.ToLower(filepath.Ext(name))
	glyph := "📄"
	c := colCyan
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		glyph = "🖼"
		c = colViolet
	case ".pdf":
		glyph = "📕"
		c = colSOSRed
	case ".txt", ".md", ".log":
		glyph = "📝"
		c = colGreen
	case ".zip", ".tar", ".gz":
		glyph = "📦"
		c = colAmber
	case ".mp3", ".wav", ".ogg", ".flac":
		glyph = "🎵"
		c = colAmber
	case ".mp4", ".mov", ".mkv":
		glyph = "🎬"
		c = colSOSRed
	case ".go", ".py", ".js", ".ts", ".rs", ".c", ".cpp":
		glyph = "💻"
		c = colCyan
	}
	bg := canvas.NewRectangle(color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x22})
	bg.CornerRadius = 8
	bg.StrokeColor = color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x66}
	bg.StrokeWidth = 1
	t := canvas.NewText(glyph, c)
	t.TextSize = 18
	t.Alignment = fyne.TextAlignCenter
	wrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(40, 40)),
		container.NewStack(bg, container.NewCenter(t)),
	)
	return wrap
}

func (t *filesTab) fileRow(e FileEntry) fyne.CanvasObject {
	badge := fileTypeBadge(e.Name)

	dirGlyph := "↓"
	dirColor := colGreen
	dirText := "RECEIVED"
	if e.Outgoing {
		dirGlyph = "↑"
		dirColor = colCyan
		dirText = "SENT"
	}
	dir := pill(dirGlyph+" "+dirText, dirColor, color.NRGBA{R: dirColor.R, G: dirColor.G, B: dirColor.B, A: 0x22})

	name := canvas.NewText(e.Name, colTextHi)
	name.TextSize = 14
	name.TextStyle = fyne.TextStyle{Bold: true}

	peer := e.Peer
	if peer == "" {
		peer = "broadcast"
	}
	meta := canvas.NewText(fmt.Sprintf("%s · %s · %s", humanBytes(e.Size), peer, relTime(e.Time)), colMuted)
	meta.TextSize = 11
	meta.TextStyle = fyne.TextStyle{Monospace: true}

	open := widget.NewButton("Path", func() {
		uri := storage.NewFileURI(e.Path)
		dialog.ShowInformation("File Path", uri.String(), fyne.CurrentApp().Driver().AllWindows()[0])
	})
	share := widget.NewButton("Copy", func() {
		fyne.CurrentApp().Driver().AllWindows()[0].Clipboard().SetContent(e.Path)
	})

	left := container.NewHBox(badge, spacer(10), container.NewVBox(name, meta))
	right := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), dir),
		spacer(6),
		container.NewHBox(layout.NewSpacer(), open, share),
	)
	row := container.NewBorder(nil, nil, left, right)
	return newCardPanel(row, 12)
}

func humanBytes(n int64) string {
	const k = 1024
	if n < k {
		return fmt.Sprintf("%d B", n)
	}
	if n < k*k {
		return fmt.Sprintf("%.1f KB", float64(n)/float64(k))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/float64(k*k))
}
