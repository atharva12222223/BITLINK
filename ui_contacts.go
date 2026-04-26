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
        "fyne.io/fyne/v2/widget"
)

type contactsTab struct {
        root     *fyne.Container
        list     *fyne.Container
        openChat func(name string)
}

func buildContactsTab(window fyne.Window, openChat func(name string)) *contactsTab {
        t := &contactsTab{openChat: openChat}

        header := sectionHeader(T("hdr.contacts"), T("hdr.contacts_sub"))
        t.list = container.NewVBox()
        t.root = container.NewVBox(header, spacer(8), t.list)
        t.refresh()
        Contacts.SetListener(func() { fyne.Do(t.refresh) })
        return t
}

func (t *contactsTab) Container() fyne.CanvasObject {
        return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

func (t *contactsTab) refresh() {
        t.list.Objects = nil

        topo := TopologySnapshot()
        sort.Slice(topo, func(i, j int) bool { return topo[i].LastSeen.After(topo[j].LastSeen) })

        if len(topo) == 0 && len(Contacts.All()) == 0 {
                t.list.Add(emptyState(T("con.no_peers"),
                        T("con.discover")))
                t.list.Refresh()
                return
        }

        saved := Contacts.All()
        sort.Slice(saved, func(i, j int) bool { return saved[i].LastSeen.After(saved[j].LastSeen) })


        if len(saved) > 0 {
                t.list.Add(subHeader(fmt.Sprintf("Saved · %d", len(saved))))
                for _, c := range saved {
                        t.list.Add(t.contactRow(c))
                }
                t.list.Add(spacer(10))
        }

        disc := []Node{}
        for _, n := range topo {
                if _, ok := Contacts.Get(n.Address); ok {
                        continue
                }
                disc = append(disc, n)
        }
        if len(disc) > 0 {
                t.list.Add(subHeader(fmt.Sprintf("Discovered · %d", len(disc))))
                for _, n := range disc {
                        t.list.Add(t.discoveredRow(n))
                }
        }
        t.list.Refresh()
}

func (t *contactsTab) contactRow(c Contact) fyne.CanvasObject {
        avatar := avatarCircle(c.Nickname, colCyan, 38)
        name := canvas.NewText(c.Nickname, colTextHi)
        name.TextSize = 14
        name.TextStyle = fyne.TextStyle{Bold: true}

        addr := canvas.NewText(c.Address, colMuted)
        addr.TextSize = 10
        addr.TextStyle = fyne.TextStyle{Monospace: true}

        last := canvas.NewText("seen "+relTime(c.LastSeen), colMuted)
        last.TextSize = 10
        last.TextStyle = fyne.TextStyle{Monospace: true}

        online := false
        if n := topoNode(c.Address); n != nil && n.Connected {
                online = true
        }
        statusColor := colMuted
        statusLabel := "OFFLINE"
        if online {
                statusColor = colGreen
                statusLabel = "LINKED"
        } else if c.RSSI != 0 {
                statusColor = colAmber
                statusLabel = "IN RANGE"
        }

        chat := widget.NewButtonWithIcon("Chat", nil, func() {
                if t.openChat != nil {
                        t.openChat(c.Nickname)
                }
        })
        chat.Importance = widget.HighImportance
        del := widget.NewButtonWithIcon("", deleteIcon(), func() { Contacts.Delete(c.Address) })
        del.Importance = widget.LowImportance

        left := container.NewHBox(avatar, spacer(10), container.NewVBox(name, addr, last))
        rightTop := container.NewHBox(layout.NewSpacer(),
                signalBars(c.RSSI), spacer(6),
                pill(statusLabel, statusColor, color.NRGBA{R: statusColor.R, G: statusColor.G, B: statusColor.B, A: 0x22}),
        )
        rightBottom := container.NewHBox(layout.NewSpacer(), chat, del)
        right := container.NewVBox(rightTop, spacer(8), rightBottom)
        row := container.NewBorder(nil, nil, left, right)
        return newCardPanel(row, 12)
}

func (t *contactsTab) discoveredRow(n Node) fyne.CanvasObject {
        nick := peerDisplayName(n.Address)
        avatar := avatarCircle(nick, colViolet, 38)

        name := canvas.NewText(nick, colTextHi)
        name.TextSize = 14
        name.TextStyle = fyne.TextStyle{Bold: true}

        addr := canvas.NewText(n.Address, colMuted)
        addr.TextSize = 10
        addr.TextStyle = fyne.TextStyle{Monospace: true}

        rssi := canvas.NewText(fmt.Sprintf("%d dBm · %d%%", n.RSSI, rssiToPct(n.RSSI)), colCyan)
        rssi.TextSize = 11
        rssi.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}

        save := widget.NewButtonWithIcon("Save", nil, func() {
                entry := widget.NewEntry()
                entry.SetPlaceHolder("Nickname")
                entry.SetText(nick)
                w := fyne.CurrentApp().Driver().AllWindows()[0]
                dialog.ShowForm("Save Contact", "Save", "Cancel",
                        []*widget.FormItem{{Text: "Nickname", Widget: entry}},
                        func(ok bool) {
                                if !ok || entry.Text == "" {
                                        return
                                }
                                Contacts.Save(n.Address, entry.Text, n.RSSI)
                        }, w)
        })
        save.Importance = widget.HighImportance

        left := container.NewHBox(avatar, spacer(10), container.NewVBox(name, addr))
        right := container.NewVBox(
                container.NewHBox(layout.NewSpacer(), signalBars(n.RSSI), spacer(6), rssi),
                spacer(6),
                container.NewHBox(layout.NewSpacer(), save),
        )
        row := container.NewBorder(nil, nil, left, right)
        return newCardPanel(row, 12)
}

func deleteIcon() fyne.Resource { return iconResource("✕") }
