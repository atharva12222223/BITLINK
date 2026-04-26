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

// chatTab — master-detail private chat. List of contacts on the left,
// active conversation pane on the right (no more popup windows).
type chatTab struct {
        root         *fyne.Container
        list         *fyne.Container
        pane         *fyne.Container
        emptyPane    fyne.CanvasObject
        currentPeer  string
        feed         *fyne.Container
        scroll       *container.Scroll
        headerHolder *fyne.Container
        inputEntry   *widget.Entry
        searchTerm   string
        searchInput  *widget.Entry
}

func buildChatTab(window fyne.Window) *chatTab {
        t := &chatTab{}

        // Left pane — contact list
        header := sectionHeader(T("hdr.private_chats"), T("hdr.1to1"))
        t.list = container.NewVBox()

        leftScroll := container.NewVScroll(t.list)
        leftScroll.SetMinSize(fyne.NewSize(280, 0))

        leftPane := container.New(layout.NewCustomPaddedLayout(0, 0, 0, 0),
                container.NewBorder(container.NewVBox(header, spacer(8)), nil, nil, nil, leftScroll),
        )

        // Right pane — empty state until a contact is selected
        t.emptyPane = newCardPanel(
                container.NewVBox(
                        spacer(40),
                        emptyState(T("chat.select"),
                                T("chat.select_desc")),
                        spacer(40),
                ), 14)

        t.pane = container.NewStack(t.emptyPane)
        rightPane := container.New(layout.NewCustomPaddedLayout(0, 0, 14, 0), t.pane)

        t.root = container.NewBorder(nil, nil, leftPane, nil, rightPane)

        // Refresh on any change
        prev := Chat.listener
        Chat.SetListener(func() {
                if prev != nil {
                        prev()
                }
                fyne.Do(func() {
                        t.refresh()
                        if t.currentPeer != "" {
                                t.renderConversation()
                        }
                })
        })

        t.refresh()
        return t
}

func (t *chatTab) Container() fyne.CanvasObject {
        return container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root)
}

func (t *chatTab) refresh() {
        t.list.Objects = nil

        // Build a unified list — saved contacts first, then any peer with messages
        type entry struct {
                name     string
                preview  string
                when     string
                unread   int
                online   bool
                rssi     int16
        }
        rows := []entry{}
        seen := map[string]bool{}

        add := func(nick string) {
                if nick == "" || seen[nick] {
                        return
                }
                // When the user has pinned to a single peer (Settings ▸ Direct Pair),
                // hide everyone else so the chat list is uncluttered.
                if !IsPeerNameVisible(nick) {
                        return
                }
                seen[nick] = true
                msgs := Chat.DirectMessages(nick)
                preview := "(no messages yet)"
                when := ""
                if len(msgs) > 0 {
                        m := msgs[len(msgs)-1]
                        preview = m.Sender + ": " + truncate(m.Text, 32)
                        if m.Outgoing {
                                preview = "you: " + truncate(m.Text, 32)
                        }
                        when = relTime(m.Timestamp)
                }
                
                online := false
                var rssi int16
                addr := TopoAddressForName(nick)
                if addr != "" {
                        if n := topoNode(addr); n != nil {
                                online = n.Connected
                                rssi = n.RSSI
                        }
                }

                rows = append(rows, entry{
                        name:     nick,
                        preview:  preview,
                        when:     when,
                        unread:   Chat.Unread("peer:" + nick),
                        online:   online,
                        rssi:     rssi,
                })
        }

        for _, c := range Contacts.All() {
                add(c.Nickname)
        }
        Chat.mu.RLock()
        for name := range Chat.byPeer {
                add(name)
        }
        Chat.mu.RUnlock()
        for _, n := range TopologySnapshot() {
                nick := peerDisplayName(n.Address)
                add(nick)
        }

        sort.Slice(rows, func(i, j int) bool {
                if rows[i].unread != rows[j].unread {
                        return rows[i].unread > rows[j].unread
                }
                return rows[i].when > rows[j].when
        })

        if len(rows) == 0 {
                t.list.Add(emptyState("No peers yet", "Run discovery on Home, then come back."))
        }
        for _, r := range rows {
                t.list.Add(t.contactRow(r.name, r.preview, r.when, r.unread, r.online, r.rssi))
        }
        t.list.Refresh()
}

func (t *chatTab) contactRow(nick, preview, when string, unread int, online bool, rssi int16) fyne.CanvasObject {
        avatar := avatarCircle(nick, colCyan, 40)

        name := canvas.NewText(nick, colTextHi)
        name.TextSize = 14
        name.TextStyle = fyne.TextStyle{Bold: true}

        prev := canvas.NewText(preview, colTextMid)
        prev.TextSize = 11

        timeLbl := canvas.NewText(when, colMuted)
        timeLbl.TextSize = 9
        timeLbl.TextStyle = fyne.TextStyle{Monospace: true}

        dotColor := colMuted
        if online {
                dotColor = colGreen
        } else if rssi != 0 {
                dotColor = colAmber
        }
        statusBox := container.NewHBox(layout.NewSpacer(), statusDot(dotColor, 8))

        right := fyne.CanvasObject(spacer(0))
        if unread > 0 {
                txt := fmt.Sprintf("%d", unread)
                if unread > 99 {
                        txt = "99+"
                }
                right = pill(txt, colBg, colCyan)
        }

        body := container.NewBorder(nil, nil,
                container.NewHBox(avatar, spacer(10), container.NewVBox(name, prev)),
                container.NewVBox(
                        container.NewHBox(layout.NewSpacer(), timeLbl),
                        container.NewHBox(layout.NewSpacer(), right),
                        statusBox,
                ),
        )

        bg := canvas.NewRectangle(colPanel)
        bg.CornerRadius = 12
        bg.StrokeColor = colHairline
        bg.StrokeWidth = 1
        if t.currentPeer == nick {
                bg.StrokeColor = colCyan
                bg.FillColor = colCyanFaint
        }
        hover := canvas.NewRectangle(color.Transparent)
        hover.CornerRadius = 12
        padded := container.New(layout.NewCustomPaddedLayout(10, 10, 12, 12), body)

        tap := newTappableArea(func() { t.openPeer(nick) })
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

func (t *chatTab) openPeer(name string) {
        t.currentPeer = name
        Chat.ClearUnread("peer:" + name)

        t.feed = container.NewVBox()
        t.scroll = container.NewVScroll(t.feed)

        t.searchInput = widget.NewEntry()
        t.searchInput.SetPlaceHolder("Search messages…")
        t.searchInput.OnChanged = func(s string) {
                t.searchTerm = s
                t.renderConversation()
        }

        t.inputEntry = widget.NewEntry()
        t.inputEntry.SetPlaceHolder("Type a message…")
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

func (t *chatTab) send(s string) {
        if s == "" {
                return
        }
        Chat.SendDirect(t.currentPeer, s)
        t.inputEntry.SetText("")
        t.renderConversation()
}

func (t *chatTab) buildPaneHeader(name string) fyne.CanvasObject {
        rssi := int16(0)
        online := false
        addr := TopoAddressForName(name)
        if addr != "" {
                if c, ok := Contacts.Get(addr); ok {
                        rssi = c.RSSI
                }
                if n := topoNode(addr); n != nil {
                        rssi = n.RSSI
                        online = n.Connected
                }
        }
        statusColor := colMuted
        statusText := "OFFLINE"
        if online {
                statusColor = colGreen
                statusText = "LINKED"
        } else if rssi != 0 {
                statusColor = colAmber
                statusText = "DISCOVERED"
        }

        avatar := avatarCircle(name, colCyan, 44)
        titleText := canvas.NewText(name, colTextHi)
        titleText.TextSize = 17
        titleText.TextStyle = fyne.TextStyle{Bold: true}
        addrText := canvas.NewText(addr, colMuted)
        addrText.TextSize = 10
        addrText.TextStyle = fyne.TextStyle{Monospace: true}
        statusPill := pill(statusText, statusColor, color.NRGBA{R: statusColor.R, G: statusColor.G, B: statusColor.B, A: 0x22})
        sigPct := canvas.NewText(fmt.Sprintf("%d%%", rssiToPct(rssi)), colCyan)
        sigPct.TextSize = 10
        sigPct.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
        sigBox := container.NewHBox(signalBars(rssi), spacer(4), sigPct)

        w := fyne.CurrentApp().Driver().AllWindows()[0]
        exportBtn := widget.NewButtonWithIcon("Export", theme.DocumentSaveIcon(), func() {
                exportConversation(w, "chat-"+sanitizeFilename(name), Chat.DirectMessages(name))
        })
        clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
                dialog.ShowConfirm("Clear conversation", "Delete all messages with "+name+"?", func(ok bool) {
                        if !ok {
                                return
                        }
                        Chat.ClearDirect(name)
                        t.renderConversation()
                }, w)
        })

        return newCardPanel(
                container.NewBorder(nil, nil,
                        container.NewHBox(avatar, spacer(12), container.NewVBox(titleText, addrText)),
                        container.NewVBox(
                                container.NewHBox(layout.NewSpacer(), statusPill),
                                container.NewHBox(layout.NewSpacer(), sigBox),
                                container.NewHBox(layout.NewSpacer(), exportBtn, clearBtn),
                        ),
                ), 12)
}

func (t *chatTab) renderConversation() {
        if t.feed == nil || t.currentPeer == "" {
                return
        }
        t.feed.Objects = nil
        msgs := Chat.DirectMessages(t.currentPeer)
        shown := 0
        for _, m := range msgs {
                if t.searchTerm != "" && !strings.Contains(strings.ToLower(m.Text), strings.ToLower(t.searchTerm)) {
                        continue
                }
                t.feed.Add(messageBubble(m))
                shown++
        }
        if len(msgs) == 0 {
                t.feed.Add(emptyState("No messages yet", "Send the first packet to establish the link."))
        } else if shown == 0 {
                t.feed.Add(emptyState("No matches", "Try a different search term."))
        }
        t.feed.Refresh()
        t.scroll.ScrollToBottom()
}
