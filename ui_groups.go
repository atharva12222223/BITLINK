package main

import (
	"fmt"
	"image/color"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type groupsTab struct {
	root         *fyne.Container
	list         *fyne.Container
	pane         *fyne.Container
	currentGroup string
	feed         *fyne.Container
	scroll       *container.Scroll
	inputEntry   *widget.Entry
	searchInput  *widget.Entry
	searchTerm   string
}

func buildGroupsTab(window fyne.Window, _ func(name string)) *groupsTab {
	t := &groupsTab{}

	header := sectionHeader(T("hdr.groups"), T("hdr.mesh_broadcasts"))

	createBtn := widget.NewButtonWithIcon("Create", theme.ContentAddIcon(), func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Group name")
		dialog.ShowForm("Create Group", "Create", "Cancel",
			[]*widget.FormItem{{Text: "Name", Widget: entry}},
			func(ok bool) {
				if !ok {
					return
				}
				name := strings.TrimSpace(entry.Text)
				if name == "" {
					return
				}
				if Groups.Has(name) {
					dialog.ShowInformation("Cannot create", "Group \""+name+"\" already exists.", window)
					return
				}
				Groups.Create(name)
				t.refresh()
			}, window)
	})
	createBtn.Importance = widget.HighImportance

	// Join an existing group by typing its name. Group channels in BitLink
	// are just shared labels — anyone who subscribes to the same name on the
	// mesh sees and sends messages on that channel. This lets the user join
	// without waiting for an invite from a peer.
	joinBtn := widget.NewButtonWithIcon("Join", theme.LoginIcon(), func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("Group name to join")
		hint := canvas.NewText("Type the exact group name. Case-sensitive.", colMuted)
		hint.TextSize = 10
		dialog.ShowForm("Join Group", "Join", "Cancel",
			[]*widget.FormItem{
				{Text: "Name", Widget: entry},
				{Text: "", Widget: container.NewVBox(hint)},
			},
			func(ok bool) {
				if !ok {
					return
				}
				name := strings.TrimSpace(entry.Text)
				if name == "" {
					return
				}
				if Groups.Has(name) {
					dialog.ShowInformation("Already joined", "You are already in \""+name+"\".", window)
					t.openGroup(name)
					return
				}
				Groups.Join(name)
				ConsumeInvite(name)
				t.refresh()
				t.openGroup(name)
			}, window)
	})



	t.list = container.NewVBox()
	leftScroll := container.NewVScroll(t.list)
	leftScroll.SetMinSize(fyne.NewSize(280, 0))

	leftPane := container.NewBorder(
		container.NewVBox(header,
			container.NewHBox(createBtn, joinBtn),
			spacer(8),
		), nil, nil, nil, leftScroll,
	)

	emptyPane := newCardPanel(
		container.NewVBox(
			spacer(40),
			emptyState("Select a group", "Pick a channel on the left to start broadcasting."),
			spacer(40),
		), 14)
	t.pane = container.NewStack(emptyPane)
	rightPane := container.New(layout.NewCustomPaddedLayout(0, 0, 14, 0), t.pane)

	t.root = container.NewBorder(nil, nil, leftPane, nil, rightPane)

	Groups.SetListener(func() { fyne.Do(t.refresh) })

	// Hook chat listener for live group msg refresh
	prev := Chat.listener
	Chat.SetListener(func() {
		if prev != nil {
			prev()
		}
		fyne.Do(func() {
			t.refresh()
			if t.currentGroup != "" {
				t.renderConversation()
			}
		})
	})

	t.refresh()
	return t
}

func (t *groupsTab) Container() fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root)
}

func (t *groupsTab) refresh() {
	t.list.Objects = nil
	all := Groups.All()
	sort.Slice(all, func(i, j int) bool {
		ui, uj := Chat.Unread("group:"+all[i].Name), Chat.Unread("group:"+all[j].Name)
		if ui != uj {
			return ui > uj
		}
		return all[i].Name < all[j].Name
	})
	if len(all) == 0 {
		t.list.Add(emptyState("No groups", "Create a channel or accept an invite."))
	}
	for _, g := range all {
		t.list.Add(t.groupRow(g))
	}
	t.list.Refresh()
}

func (t *groupsTab) groupRow(g Group) fyne.CanvasObject {
	icon := canvas.NewText("⏚", colCyan)
	icon.TextSize = 22
	icon.TextStyle = fyne.TextStyle{Bold: true}

	title := canvas.NewText(g.Name, colTextHi)
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}

	msgs := Chat.GroupMessages(g.Name)
	last := "no messages yet"
	when := ""
	if n := len(msgs); n > 0 {
		m := msgs[n-1]
		last = m.Sender + ": " + truncate(m.Text, 32)
		if m.Outgoing {
			last = "you: " + truncate(m.Text, 32)
		}
		when = relTime(m.Timestamp)
	}
	prev := canvas.NewText(last, colTextMid)
	prev.TextSize = 11
	timeLbl := canvas.NewText(when, colMuted)
	timeLbl.TextSize = 9
	timeLbl.TextStyle = fyne.TextStyle{Monospace: true}

	right := fyne.CanvasObject(spacer(0))
	if u := Chat.Unread("group:" + g.Name); u > 0 {
		txt := fmt.Sprintf("%d", u)
		if u > 99 {
			txt = "99+"
		}
		right = pill(txt, colBg, colCyan)
	}

	body := container.NewBorder(nil, nil,
		container.NewHBox(container.NewCenter(icon), spacer(10), container.NewVBox(title, prev)),
		container.NewVBox(
			container.NewHBox(layout.NewSpacer(), timeLbl),
			container.NewHBox(layout.NewSpacer(), right),
		),
	)

	bg := canvas.NewRectangle(colPanel)
	bg.CornerRadius = 12
	bg.StrokeColor = colHairline
	bg.StrokeWidth = 1
	if t.currentGroup == g.Name {
		bg.StrokeColor = colCyan
		bg.FillColor = colCyanFaint
	}
	hover := canvas.NewRectangle(color.Transparent)
	hover.CornerRadius = 12
	padded := container.New(layout.NewCustomPaddedLayout(10, 10, 12, 12), body)

	tap := newTappableArea(func() { t.openGroup(g.Name) })
	tap.onHover = func(in bool) {
		if in {
			hover.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x10}
		} else {
			hover.FillColor = color.Transparent
		}
		hover.Refresh()
	}
	return container.New(layout.NewCustomPaddedLayout(2, 2, 2, 2),
		container.NewStack(bg, hover, padded, tap),
	)
}

func (t *groupsTab) openGroup(name string) {
	t.currentGroup = name
	Chat.ClearUnread("group:" + name)

	t.feed = container.NewVBox()
	t.scroll = container.NewVScroll(t.feed)

	t.searchInput = widget.NewEntry()
	t.searchInput.SetPlaceHolder("Search messages…")
	t.searchInput.OnChanged = func(s string) {
		t.searchTerm = s
		t.renderConversation()
	}

	t.inputEntry = widget.NewEntry()
	t.inputEntry.SetPlaceHolder("Send to " + name + "…")
	t.inputEntry.OnSubmitted = func(s string) { t.send(s) }
	sendBtn := widget.NewButtonWithIcon("Send", theme.MailSendIcon(), func() { t.send(t.inputEntry.Text) })
	sendBtn.Importance = widget.HighImportance

	header := t.buildPaneHeader(name)
	bottom := newCardPanel(container.NewBorder(nil, nil, nil, sendBtn, t.inputEntry), 8)
	body := container.NewBorder(
		container.NewVBox(header, spacer(6), t.searchInput),
		container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), bottom),
		nil, nil,
		t.scroll,
	)

	t.pane.Objects = []fyne.CanvasObject{body}
	t.pane.Refresh()
	t.refresh()
	t.renderConversation()
}

func (t *groupsTab) buildPaneHeader(name string) fyne.CanvasObject {
	icon := canvas.NewText("⏚", colCyan)
	icon.TextSize = 28
	icon.TextStyle = fyne.TextStyle{Bold: true}

	title := canvas.NewText(name, colTextHi)
	title.TextSize = 17
	title.TextStyle = fyne.TextStyle{Bold: true}

	sub := canvas.NewText("group channel", colMuted)
	sub.TextSize = 10
	sub.TextStyle = fyne.TextStyle{Monospace: true}

	count := canvas.NewText(fmt.Sprintf("%d msgs", len(Chat.GroupMessages(name))), colCyan)
	count.TextSize = 10
	count.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

	w := fyne.CurrentApp().Driver().AllWindows()[0]
	exportBtn := widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), func() {
		exportConversation(w, "group-"+sanitizeFilename(name), Chat.GroupMessages(name))
	})
	clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		dialog.ShowConfirm("Clear group", "Delete all messages in "+name+"?", func(ok bool) {
			if !ok {
				return
			}
			Chat.ClearGroup(name)
			t.renderConversation()
		}, w)
	})
	leaveBtn := widget.NewButton("Leave", func() {
		dialog.ShowConfirm("Leave group?", "Stop receiving "+name+" broadcasts?", func(ok bool) {
			if ok && name != GroupBroadcast {
				Groups.Leave(name)
				t.currentGroup = ""
				t.refresh()
			}
		}, w)
	})
	if name == GroupBroadcast {
		leaveBtn.Disable()
	}

	return newCardPanel(
		container.NewBorder(nil, nil,
			container.NewHBox(container.NewCenter(icon), spacer(12), container.NewVBox(title, sub)),
			container.NewVBox(
				container.NewHBox(layout.NewSpacer(), pill("BROADCASTING", colCyan, colCyanFaint)),
				container.NewHBox(layout.NewSpacer(), count),
				container.NewHBox(layout.NewSpacer(), exportBtn, clearBtn, leaveBtn),
			),
		), 12)
}

func (t *groupsTab) send(s string) {
	if s == "" {
		return
	}
	Chat.SendGroup(t.currentGroup, s)
	t.inputEntry.SetText("")
	t.renderConversation()
}

func (t *groupsTab) renderConversation() {
	if t.feed == nil || t.currentGroup == "" {
		return
	}
	t.feed.Objects = nil
	msgs := Chat.GroupMessages(t.currentGroup)
	shown := 0
	for _, m := range msgs {
		if t.searchTerm != "" && !strings.Contains(strings.ToLower(m.Text), strings.ToLower(t.searchTerm)) {
			continue
		}
		t.feed.Add(messageBubble(m))
		shown++
	}
	if len(msgs) == 0 {
		t.feed.Add(emptyState("Quiet on the channel", "Be the first to broadcast."))
	} else if shown == 0 {
		t.feed.Add(emptyState("No matches", "Try a different search term."))
	}
	t.feed.Refresh()
	t.scroll.ScrollToBottom()
}
