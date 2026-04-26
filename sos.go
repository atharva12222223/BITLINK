package main

import (
	"fmt"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type SOSEvent struct {
	From     string
	Address  string
	Text     string
	RSSI     int16
	Received time.Time
}

var (
	sosMu       sync.Mutex
	sosCallback func(SOSEvent)
	threatCB    func(text string)
	isSOSOpen   bool
)

func SetSOSCallback(f func(SOSEvent)) {
	sosMu.Lock()
	defer sosMu.Unlock()
	sosCallback = f
}

func SetThreatCallback(f func(text string)) {
	sosMu.Lock()
	defer sosMu.Unlock()
	threatCB = f
}

func BroadcastSOSWithText(text string) {
	if text == "" {
		text = "HELP"
	}
	ShoutText("SOS:", trim(text, 18))
	p := newPacket(PktSOS, []byte(text))
	p.Sender = SelfName()
	BroadcastPacket(p)
}

func BroadcastSOS() { BroadcastSOSWithText("HELP") }

func HandleIncomingSOS(text, addr string, rssi int16) {
	sosMu.Lock()
	cb := sosCallback
	sosMu.Unlock()
	if cb == nil {
		return
	}
	cb(SOSEvent{From: addr, Address: addr, Text: text, RSSI: rssi, Received: time.Now()})
}

// SendThreatReport broadcasts an anonymous threat warning. We deliberately
// suppress identity beacons for ~5s while the binary packet is on air, so
// receivers can't trivially correlate the threat broadcast with the BLE name
// being announced in the same window. The packet's Sender field is left blank.
func SendThreatReport(text string) {
	if text == "" {
		return
	}
	// Open a quiet window covering the shout + a few cycles of the binary
	// packet broadcasting (3s shout + ~2s binary chunk burst).
	SetBeaconQuietWindow(5 * time.Second)
	ShoutText("THREAT:", trim(text, 17))
	p := newPacket(PktThreat, []byte(text))
	p.Sender = "" // anonymous in the binary packet too
	BroadcastPacket(p)
}

func HandleIncomingThreat(text string) {
	sosMu.Lock()
	cb := threatCB
	sosMu.Unlock()
	if cb != nil {
		cb(text)
	}
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// showSOSAlert — non-blocking push notification that auto-dismisses after 5s.
// Multiple SOS alerts can stack without blocking each other.
func showSOSAlert(window fyne.Window, ev SOSEvent) {
	if window == nil {
		return
	}

	bg := canvas.NewRectangle(color.NRGBA{R: 0x2A, G: 0x0E, B: 0x18, A: 0xFF})
	bg.CornerRadius = 14
	bg.StrokeColor = colSOSRed
	bg.StrokeWidth = 2

	hdrIcon := canvas.NewText("◉", colSOSRed)
	hdrIcon.TextSize = 16
	hdrIcon.TextStyle = fyne.TextStyle{Bold: true}

	hdr := canvas.NewText(T("sos.incoming"), colTextHi)
	hdr.TextSize = 15
	hdr.TextStyle = fyne.TextStyle{Bold: true}

	from := canvas.NewText(T("sos.from")+" "+peerDisplayName(ev.From), colTextMid)
	from.TextSize = 11
	from.TextStyle = fyne.TextStyle{Monospace: true}

	sig := canvas.NewText(fmt.Sprintf("SIG %d%% · %d dBm", rssiToPct(ev.RSSI), ev.RSSI), colAmber)
	sig.TextSize = 10
	sig.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	body := canvas.NewText(ev.Text, colTextHi)
	body.TextSize = 14
	body.TextStyle = fyne.TextStyle{Bold: true}

	countdown := canvas.NewText("5", colMuted)
	countdown.TextSize = 10
	countdown.TextStyle = fyne.TextStyle{Monospace: true}

	dismiss := widget.NewButton("✕", nil)
	dismiss.Importance = widget.LowImportance

	row := container.NewBorder(nil, nil,
		container.NewHBox(hdrIcon, spacer(8), container.NewVBox(
			container.NewHBox(hdr, spacer(8), from),
			container.NewHBox(body, spacer(12), sig),
		)),
		container.NewHBox(countdown, spacer(4), dismiss),
	)

	content := container.NewStack(bg, container.New(layout.NewCustomPaddedLayout(10, 10, 14, 14), row))

	overlay := container.NewWithoutLayout(content)
	window.Canvas().Overlays().Add(overlay)

	winSize := window.Canvas().Size()
	content.Resize(content.MinSize())
	content.Move(fyne.NewPos(winSize.Width/2-content.MinSize().Width/2, 16))

	dismiss.OnTapped = func() { window.Canvas().Overlays().Remove(overlay) }

	safeGo("sos-auto-dismiss", func() {
		for i := 5; i > 0; i-- {
			time.Sleep(1 * time.Second)
			ii := i
			fyne.Do(func() {
				countdown.Text = fmt.Sprintf("%d", ii-1)
				countdown.Refresh()
			})
		}
		fyne.Do(func() {
			window.Canvas().Overlays().Remove(overlay)
			showNotificationWithDuration(window, "SOS from "+peerDisplayName(ev.From)+": "+ev.Text, true, 5*time.Second)
		})
	})
}

