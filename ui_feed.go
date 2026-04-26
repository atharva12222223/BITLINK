package main

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// homeTab — clean dashboard: hero radar + live mesh telemetry + quick actions.
// Recent chats and group lists live in their own dedicated tabs now.
type homeTab struct {
	root        *fyne.Container
	statTilePeers, statTileLinked, statTileMsgs *canvas.Text
	statusValue *canvas.Text
	scanBtn     *widget.Button
}

// quickAction — landing tile callbacks injected by main.go.
type quickAction struct {
	OpenChat    func()
	OpenGroups  func()
	OpenSafety  func()
	OpenMap     func()
	BroadcastSOS func()
}

var QuickActions quickAction

func buildHomeTab(window fyne.Window) *homeTab {
	t := &homeTab{}

	// ─── HERO radar ─────────────────────────────────────────────────────────
	pulseSize := float32(150)
	mkRing := func(diameter float32, alpha uint8) fyne.CanvasObject {
		c := canvas.NewCircle(color.Transparent)
		c.StrokeColor = color.NRGBA{R: colCyan.R, G: colCyan.G, B: colCyan.B, A: alpha}
		c.StrokeWidth = 1
		return container.New(layout.NewGridWrapLayout(fyne.NewSize(diameter, diameter)), c)
	}
	ring1 := mkRing(pulseSize*0.40, 0x40)
	ring2 := mkRing(pulseSize*0.70, 0x30)
	ring3 := mkRing(pulseSize, 0x22)

	core := canvas.NewCircle(colCyan)
	coreBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(14, 14)), core)
	coreHalo := canvas.NewCircle(colCyanGlow)
	coreHaloBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(34, 34)), coreHalo)

	pulse := canvas.NewCircle(color.Transparent)
	pulse.StrokeColor = colCyan
	pulse.StrokeWidth = 2
	pulseHolder := container.NewWithoutLayout(pulse)

	pulseBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(pulseSize, pulseSize)),
		container.NewStack(
			container.NewCenter(ring3),
			container.NewCenter(ring2),
			container.NewCenter(ring1),
			pulseHolder,
			container.NewCenter(coreHaloBox),
			container.NewCenter(coreBox),
		),
	)

	safeGo("hero-pulse", func() {
		time.Sleep(200 * time.Millisecond)
		animSize := canvas.NewSizeAnimation(
			fyne.NewSize(20, 20),
			fyne.NewSize(pulseSize, pulseSize),
			2400*time.Millisecond,
			func(s fyne.Size) {
				pulse.Resize(s)
				pulse.Move(fyne.NewPos((pulseSize-s.Width)/2, (pulseSize-s.Height)/2))
				pulse.Refresh()
			})
		animSize.RepeatCount = fyne.AnimationRepeatForever
		animSize.Start()
		animColor := canvas.NewColorRGBAAnimation(
			colCyan,
			color.NRGBA{R: colCyan.R, G: colCyan.G, B: colCyan.B, A: 0},
			2400*time.Millisecond,
			func(c2 color.Color) {
				pulse.StrokeColor = c2
				pulse.Refresh()
			})
		animColor.RepeatCount = fyne.AnimationRepeatForever
		animColor.Start()
	})

	heroEyebrow := canvas.NewText(T("home.hero_eyebrow"), colCyan)
	heroEyebrow.TextSize = 11
	heroEyebrow.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	heroEyebrow.Alignment = fyne.TextAlignCenter

	heroTitle := canvas.NewText(T("home.scan_title"), colTextHi)
	heroTitle.TextSize = 26
	heroTitle.TextStyle = fyne.TextStyle{Bold: true}
	heroTitle.Alignment = fyne.TextAlignCenter

	heroSub := canvas.NewText(T("home.scan_sub1"), colTextMid)
	heroSub.TextSize = 13
	heroSub.Alignment = fyne.TextAlignCenter
	heroSub2 := canvas.NewText(T("home.scan_sub2"), colMuted)
	heroSub2.TextSize = 12
	heroSub2.Alignment = fyne.TextAlignCenter

	t.statusValue = canvas.NewText(T("home.ready"), colCyan)
	t.statusValue.TextSize = 11
	t.statusValue.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	t.statusValue.Alignment = fyne.TextAlignCenter

	t.scanBtn = widget.NewButtonWithIcon(T("home.begin_discovery"), nil, func() {
		t.statusValue.Text = T("home.scanning")
		t.statusValue.Color = colAmber
		t.statusValue.Refresh()
		safeGo("hero-scan", func() {
			time.Sleep(1500 * time.Millisecond)
			fyne.Do(func() {
				t.statusValue.Text = fmt.Sprintf(T("home.found_nodes"), len(TopologySnapshot()))
				t.statusValue.Color = colCyan
				t.statusValue.Refresh()
				t.refresh()
			})
		})
	})
	t.scanBtn.Importance = widget.HighImportance

	heroLeft := container.NewVBox(
		container.NewCenter(pulseBox),
		spacer(8),
		heroEyebrow,
		heroTitle,
		spacer(4),
		heroSub,
		heroSub2,
		spacer(14),
		container.NewCenter(container.New(layout.NewGridWrapLayout(fyne.NewSize(240, 46)), t.scanBtn)),
		spacer(6),
		t.statusValue,
		spacer(4),
	)

	// Live stat tiles — 3 tiles
	t.statTilePeers, _ = makeStatTile("0", T("home.stat_peers"), colCyan)
	t.statTileLinked, _ = makeStatTile("0", T("home.stat_linked"), colGreen)
	t.statTileMsgs, _ = makeStatTile("0", T("home.stat_unread"), colViolet)

	tilesGrid := container.New(layout.NewVBoxLayout(),
		container.NewGridWithColumns(2,
			statTileWrap(t.statTilePeers, mkSubLabel(T("home.stat_peers")), colCyan),
			statTileWrap(t.statTileLinked, mkSubLabel(T("home.stat_linked")), colGreen),
		),
		container.NewGridWithColumns(1,
			statTileWrap(t.statTileMsgs, mkSubLabel(T("home.stat_unread")), colViolet),
		),
	)
	tilesBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(300, 280)), tilesGrid)

	hero := newGlowCard(
		container.NewBorder(nil, nil, nil, tilesBox, heroLeft),
		22, colCyan)

	// ─── Quick Actions ───────────────────────────────────────────────────────
	quickGrid := container.New(layout.NewGridLayout(4),
		actionTile(T("nav.chat"), T("home.qa_chat_desc"), "✉", colCyan, func() {
			if QuickActions.OpenChat != nil {
				QuickActions.OpenChat()
			}
		}),
		actionTile(T("nav.groups"), T("home.qa_groups_desc"), "⏚", colViolet, func() {
			if QuickActions.OpenGroups != nil {
				QuickActions.OpenGroups()
			}
		}),
		actionTile(T("nav.compass"), T("home.qa_compass_desc"), "◎", colGreen, func() {
			if QuickActions.OpenMap != nil {
				QuickActions.OpenMap()
			}
		}),
		actionTile(T("nav.safety"), T("home.qa_safety_desc"), "♥", colSOSRed, func() {
			if QuickActions.OpenSafety != nil {
				QuickActions.OpenSafety()
			}
		}),
	)

	t.root = container.NewVBox(
		hero,
		spacer(18),
		container.NewHBox(
			canvas.NewText("◇", colCyan), spacer(8),
			textBold(T("home.quick_actions"), colTextHi, 16),
		),
		spacer(8),
		quickGrid,
	)

	t.refresh()
	return t
}

func mkSubLabel(s string) *canvas.Text {
	l := canvas.NewText(s, colMuted)
	l.TextSize = 9
	l.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	return l
}

func textBold(s string, c color.NRGBA, size float32) fyne.CanvasObject {
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

func makeStatTile(value, label string, accent color.NRGBA) (*canvas.Text, *canvas.Text) {
	v := canvas.NewText(value, accent)
	v.TextSize = 22
	v.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	l := canvas.NewText(label, colMuted)
	l.TextSize = 9
	l.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	return v, l
}

func statTileWrap(value, label *canvas.Text, accent color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel2)
	bg.CornerRadius = 10
	bg.StrokeColor = color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x33}
	bg.StrokeWidth = 1
	bar := canvas.NewRectangle(accent)
	bar.SetMinSize(fyne.NewSize(24, 2))
	bar.CornerRadius = 1

	sizer := canvas.NewRectangle(color.Transparent)
	sizer.SetMinSize(fyne.NewSize(120, 78))

	body := container.New(layout.NewCustomPaddedLayout(10, 10, 12, 12),
		container.NewVBox(value, spacer(4), bar, spacer(4), label),
	)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 4, 4),
		container.NewStack(sizer, bg, body))
}

func actionTile(title, sub, glyph string, accent color.NRGBA, onTap func()) fyne.CanvasObject {
	icon := canvas.NewText(glyph, accent)
	icon.TextSize = 30
	icon.Alignment = fyne.TextAlignCenter
	tt := canvas.NewText(title, colTextHi)
	tt.TextSize = 16
	tt.Alignment = fyne.TextAlignCenter
	tt.TextStyle = fyne.TextStyle{Bold: true}
	st := canvas.NewText(sub, colMuted)
	st.TextSize = 9
	st.Alignment = fyne.TextAlignCenter
	st.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	body := container.NewVBox(spacer(10), icon, spacer(2), tt, st, spacer(10))
	card := newGlowCard(body, 14, accent)

	hover := canvas.NewRectangle(color.Transparent)
	hover.CornerRadius = 16

	tap := newTappableArea(onTap)
	tap.onHover = func(in bool) {
		if in {
			hover.FillColor = color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x18}
		} else {
			hover.FillColor = color.Transparent
		}
		hover.Refresh()
	}
	return container.New(layout.NewCustomPaddedLayout(2, 2, 2, 2),
		container.NewStack(card, hover, tap),
	)
}

func (t *homeTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *homeTab) refresh() {
	if t == nil {
		return
	}
	peers := len(TopologySnapshot())
	linked := ActivePeerCount()
	unread := Chat.UnreadTotal()

	t.statTilePeers.Text = fmt.Sprintf("%d", peers)
	t.statTileLinked.Text = fmt.Sprintf("%d", linked)
	if unread > 999 {
		t.statTileMsgs.Text = "999+"
	} else {
		t.statTileMsgs.Text = fmt.Sprintf("%d", unread)
	}
	t.statTilePeers.Refresh()
	t.statTileLinked.Refresh()
	t.statTileMsgs.Refresh()
}

// ─── Empty state helper ─────────────────────────────────────────────────────

func emptyState(title, sub string) fyne.CanvasObject {
	icon := canvas.NewText("◇", colCyanDim)
	icon.TextSize = 32
	icon.Alignment = fyne.TextAlignCenter
	t := canvas.NewText(title, colTextMid)
	t.TextSize = 14
	t.Alignment = fyne.TextAlignCenter
	t.TextStyle = fyne.TextStyle{Bold: true}
	s := canvas.NewText(sub, colMuted)
	s.TextSize = 11
	s.Alignment = fyne.TextAlignCenter
	body := container.NewVBox(spacer(12), icon, spacer(4), t, s, spacer(12))
	return container.New(layout.NewCustomPaddedLayout(8, 8, 8, 8), body)
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}


// ─── exportConversation — used by Chat & Groups tabs ──────────────────────

func exportConversation(window fyne.Window, defaultName string, msgs []Message) {
	if len(msgs) == 0 {
		dialog.ShowInformation("Export", "No messages to export.", window)
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("BitLink conversation export — %s\n\n", time.Now().Format(time.RFC1123)))
	for _, m := range msgs {
		dir := "←"
		if m.Outgoing {
			dir = "→"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s %s: %s\n",
			m.Timestamp.Format("2006-01-02 15:04:05"),
			dir, m.Sender, m.Text))
	}
	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
		if err != nil || uc == nil {
			return
		}
		defer uc.Close()
		_, _ = uc.Write([]byte(sb.String()))
		dialog.ShowInformation("Export", "Conversation saved.", window)
	}, window)
	d.SetFileName(defaultName + ".txt")
	d.SetFilter(storage.NewExtensionFileFilter([]string{".txt"}))
	d.Show()
}

// ─── messageBubble — clean WhatsApp-style chat bubble ──────────────────────

func messageBubble(m Message) fyne.CanvasObject {
	bgC := colPanel2
	stroke := colHairline
	if m.Outgoing {
		bgC = color.NRGBA{R: 0x07, G: 0x40, B: 0x55, A: 0xFF}
		stroke = colCyan
	}
	bg := canvas.NewRectangle(bgC)
	bg.CornerRadius = 14
	bg.StrokeColor = color.NRGBA{R: stroke.R, G: stroke.G, B: stroke.B, A: 0x44}
	bg.StrokeWidth = 1

	bodyWrap := widget.NewLabel(m.Text)
	bodyWrap.Wrapping = fyne.TextWrapWord

	when := canvas.NewText(m.Timestamp.Format("15:04"), colMuted)
	when.TextSize = 9
	when.TextStyle = fyne.TextStyle{Monospace: true}

	status := ""
	statusColor := colMuted
	if m.Outgoing {
		switch m.Status {
		case StatusDelivered:
			status = "✓✓"
			statusColor = colGreen
		default:
			status = "✓"
		}
	}
	statusLbl := canvas.NewText(status, statusColor)
	statusLbl.TextSize = 9
	statusLbl.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	footerRow := container.NewHBox(layout.NewSpacer(), when, spacer(4), statusLbl)

	var content fyne.CanvasObject
	if m.Outgoing {
		content = container.NewVBox(bodyWrap, footerRow)
	} else {
		senderName := m.Sender
		if senderName == "" {
			senderName = m.Peer
		}
		sender := canvas.NewText(senderName, colCyan)
		sender.TextSize = 11
		sender.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
		content = container.NewVBox(sender, bodyWrap, footerRow)
	}
	padded := container.New(layout.NewCustomPaddedLayout(8, 8, 12, 12), content)
	bubble := container.NewStack(bg, padded)

	// Use a container that aligns the bubble but allows it to have a proper width for wrapping.
	// We use a Border layout with a spacer to push the bubble to one side,
	// but we wrap the bubble in a VBox to ensure it respects its own MinSize
	// rather than being stretched to the full width of the screen.
	var aligned fyne.CanvasObject
	if m.Outgoing {
		// Outgoing: Spacer on left, Bubble on right
		// Wrap bubble in a container that caps its width to prevent it from being too wide
		// but allows it to be wide enough for the text to wrap normally.
		c := container.NewVBox(bubble)
		aligned = container.NewBorder(nil, nil, layout.NewSpacer(), nil, c)
	} else {
		// Incoming: Bubble on left, Spacer on right
		c := container.NewVBox(bubble)
		aligned = container.NewBorder(nil, nil, nil, layout.NewSpacer(), c)
	}

	tap := newTappableArea(func() {
		fyne.CurrentApp().Driver().AllWindows()[0].Clipboard().SetContent(m.Text)
	})
	return container.New(layout.NewCustomPaddedLayout(3, 3, 4, 4),
		container.NewStack(aligned, tap),
	)
}
