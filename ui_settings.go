package main

import (
	"fmt"
	"image/color"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type settingsTab struct {
	root *fyne.Container
}

func buildSettingsTab(window fyne.Window) *settingsTab {
	t := &settingsTab{}

	header := sectionHeader(T("hdr.settings"), T("hdr.settings_sub"))

	// ─── Language card ──────────────────────────────────────────────────────
	langOptions := []string{}
	for _, l := range AvailableLangs() {
		langOptions = append(langOptions, LangDisplayName(l))
	}
	langSelect := widget.NewSelect(langOptions, func(sel string) {
		for _, l := range AvailableLangs() {
			if LangDisplayName(l) == sel {
				SetLang(l)
				dialog.ShowInformation(T("set.language"), T("set.lang_restart"), window)
				break
			}
		}
	})
	langSelect.SetSelected(LangDisplayName(CurrentLang()))

	langCard := newGlowCard(container.NewVBox(
		container.NewHBox(
			canvas.NewText("🌐", colCyan),
			spacer(8),
			func() fyne.CanvasObject {
				t := canvas.NewText(T("set.language"), colTextHi)
				t.TextSize = 15
				t.TextStyle = fyne.TextStyle{Bold: true}
				return t
			}(),
		),
		spacer(6),
		langSelect,
	), 16, colViolet)

	// ─── Identity card ──────────────────────────────────────────────────────
	nameEntry := widget.NewEntry()
	nameEntry.SetText(SelfName())
	nameEntry.SetPlaceHolder("Operator callsign")

	colorPicker := newColorSwatchRow()
	colorPicker.SetColor(SelfColor())

	previewWrap := container.NewStack(avatarCircle(nameEntry.Text, SelfColor(), 64))
	updatePreview := func() {
		previewWrap.Objects = []fyne.CanvasObject{
			avatarCircle(nameEntry.Text, colorPicker.Color(), 64),
		}
		previewWrap.Refresh()
	}
	nameEntry.OnChanged = func(string) { updatePreview() }
	colorPicker.OnChange = func(color.NRGBA) { updatePreview() }

	saveBtn := widget.NewButtonWithIcon(T("set.save_profile"), nil, func() {
		setSelf(nameEntry.Text, colorPicker.Color())
		dialog.ShowInformation(T("set.saved"), T("set.profile_updated"), window)
	})
	saveBtn.Importance = widget.HighImportance

	idCard := newGlowCard(container.NewVBox(
		container.NewHBox(
			canvas.NewText("◈", colCyan),
			spacer(8),
			func() fyne.CanvasObject {
				t := canvas.NewText(T("set.identity"), colTextHi)
				t.TextSize = 15
				t.TextStyle = fyne.TextStyle{Bold: true}
				return t
			}(),
		),
		spacer(6),
		container.NewVBox(
			container.NewCenter(previewWrap),
			spacer(6),
			labelMuted(T("set.display_name")),
			nameEntry,
			spacer(4),
			labelMuted(T("set.avatar_color")),
			colorPicker.Container,
			spacer(8),
			saveBtn,
		),
	), 16, colCyan)

	// ─── Mesh card ─────────────────────────────────────────────────────────
	ttlEntry := widget.NewEntry()
	ttlEntry.SetText("3")

	advertiseToggle := widget.NewCheck(T("set.advertise"), func(b bool) { _ = b })
	advertiseToggle.SetChecked(true)

	meshCard := newCardPanel(container.NewVBox(
		container.NewHBox(
			canvas.NewText("⏚", colCyan),
			spacer(8),
			func() fyne.CanvasObject {
				t := canvas.NewText(T("set.mesh"), colTextHi)
				t.TextSize = 15
				t.TextStyle = fyne.TextStyle{Bold: true}
				return t
			}(),
		),
		spacer(6),
		labelMuted(T("set.ttl")),
		ttlEntry,
		spacer(4),
		advertiseToggle,
	), 16)
	_ = meshCard



	// ─── Data card ─────────────────────────────────────────────────────────
	clearChat := widget.NewButton(T("set.clear_chat"), func() {
		dialog.ShowConfirm("Clear all chats?", "This deletes every direct and group conversation locally. Cannot be undone.", func(ok bool) {
			if !ok {
				return
			}
			Chat.mu.Lock()
			Chat.byPeer = map[string][]Message{}
			Chat.byGroup = map[string][]Message{}
			Chat.unread = map[string]int{}
			Chat.mu.Unlock()
			Chat.save()
			Chat.notify()
			dialog.ShowInformation("Cleared", "All conversations deleted.", window)
		}, window)
	})
	clearChat.Importance = widget.DangerImportance

	hardReset := widget.NewButton(T("set.hard_reset"), func() {
		dialog.ShowConfirm(
			"Hard reset?",
			"This wipes EVERYTHING on this device:\n\n"+
				"• Username & avatar color\n"+
				"• Every chat, group, contact, file, safety entry\n"+
				"• Pairing & topology cache\n"+
				"• Mesh encryption key\n"+
				"• Migration markers (next launch shows the welcome screen)\n\n"+
				"Cannot be undone. Continue?",
			func(ok bool) {
				if !ok {
					return
				}
				performHardReset()
				dialog.ShowInformation(
					"Reset complete",
					"All data wiped. The app will now exit — relaunch BitLink to start fresh.",
					window,
				)
				safeGo("hard-reset-quit", func() {
					time.Sleep(900 * time.Millisecond)
					fyne.Do(func() { fyne.CurrentApp().Quit() })
				})
			},
			window,
		)
	})
	hardReset.Importance = widget.DangerImportance


	dataCard := newCardPanel(container.NewVBox(
		container.NewHBox(
			canvas.NewText("◇", colSOSRed),
			spacer(8),
			func() fyne.CanvasObject {
				t := canvas.NewText(T("set.data"), colTextHi)
				t.TextSize = 15
				t.TextStyle = fyne.TextStyle{Bold: true}
				return t
			}(),
		),
		spacer(4),
		labelMuted(T("set.local_data")),
		clearChat,
		spacer(6),
		labelMuted(T("set.nuclear")),
		hardReset,
	), 16)

	// ─── About card ────────────────────────────────────────────────────────
	aboutTitle := canvas.NewText("BitLink", colCyan)
	aboutTitle.TextSize = 22
	aboutTitle.TextStyle = fyne.TextStyle{Bold: true}
	aboutVer := canvas.NewText("v2.0 · Mesh OS", colMuted)
	aboutVer.TextSize = 10
	aboutVer.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	aboutBlurb := widget.NewLabel("Offline mesh communicator. Discovers peers via BLE, exchanges messages, files and SOS broadcasts. Everything stays on your device.")
	aboutBlurb.Wrapping = fyne.TextWrapWord
	stack := canvas.NewText(fmt.Sprintf("Runtime · %s/%s · Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version()), colMuted)
	stack.TextSize = 10
	stack.TextStyle = fyne.TextStyle{Monospace: true}
	libs := canvas.NewText("BLE · tinygo.org/x/bluetooth     UI · fyne.io/fyne/v2", colMuted)
	libs.TextSize = 10
	libs.TextStyle = fyne.TextStyle{Monospace: true}

	aboutCard := newCardPanel(container.NewVBox(
		container.NewBorder(nil, nil,
			container.New(layout.NewCustomPaddedLayout(0, 0, 0, 16), aboutTitle),
			aboutVer,
		),
		spacer(6),
		aboutBlurb,
		spacer(4),
		stack,
		libs,
	), 16)

	t.root = container.NewVBox(
		header, spacer(10),
		langCard,
		spacer(12),
		idCard,
		spacer(12),
		// meshCard,
		// spacer(12),
		dataCard,
		spacer(12),
		aboutCard,
	)
	return t
}

func labelMuted(s string) fyne.CanvasObject {
	t := canvas.NewText(s, colMuted)
	t.TextSize = 11
	t.TextStyle = fyne.TextStyle{Monospace: true}
	return t
}

func (t *settingsTab) Container() fyne.CanvasObject {
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(18, 18, 22, 22), t.root))
}

// ─── First-launch onboarding ───────────────────────────────────────────────

func showFirstLaunchSetup(window fyne.Window, onDone func()) {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("e.g. Alpha-01, Sierra-Actual")

	picker := newColorSwatchRow()
	picker.SetColor(colCyan)

	previewWrap := container.NewStack(container.NewCenter(avatarCircle("?", picker.Color(), 72)))
	nameEntry.OnChanged = func(s string) {
		previewWrap.Objects = []fyne.CanvasObject{
			container.NewCenter(avatarCircle(s, picker.Color(), 72)),
		}
		previewWrap.Refresh()
	}
	picker.OnChange = func(c color.NRGBA) {
		previewWrap.Objects = []fyne.CanvasObject{
			container.NewCenter(avatarCircle(nameEntry.Text, c, 72)),
		}
		previewWrap.Refresh()
	}

	welcome := canvas.NewText("Welcome to BitLink", colCyan)
	welcome.TextSize = 22
	welcome.TextStyle = fyne.TextStyle{Bold: true}
	welcome.Alignment = fyne.TextAlignCenter

	tagline := canvas.NewText("Your offline mesh starts here.", colTextMid)
	tagline.TextSize = 12
	tagline.Alignment = fyne.TextAlignCenter

	form := container.NewVBox(
		welcome,
		tagline,
		spacer(10),
		previewWrap,
		spacer(6),
		labelMuted("Display name"),
		nameEntry,
		spacer(4),
		labelMuted("Avatar color"),
		picker.Container,
		spacer(4),
		labelMuted("Stored locally — no account, no server."),
	)

	d := dialog.NewCustomConfirm("Setup", "Begin", "Skip", form, func(ok bool) {
		n := nameEntry.Text
		if n == "" {
			showFirstLaunchSetup(window, onDone)
			return
		}
		setSelf(n, picker.Color())
		if onDone != nil {
			onDone()
		}
	}, window)
	d.Resize(fyne.NewSize(460, 460))
	d.Show()
}
