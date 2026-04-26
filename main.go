package main

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ─── Palette — refined cyber/glass ──────────────────────────────────────────

var (
	colBg       = color.NRGBA{R: 0x07, G: 0x0B, B: 0x14, A: 0xFF}
	colSidebar  = color.NRGBA{R: 0x04, G: 0x07, B: 0x0E, A: 0xFF}
	colPanel    = color.NRGBA{R: 0x10, G: 0x16, B: 0x23, A: 0xFF}
	colPanel2   = color.NRGBA{R: 0x16, G: 0x1E, B: 0x30, A: 0xFF}
	colPanel3   = color.NRGBA{R: 0x1E, G: 0x28, B: 0x3E, A: 0xFF}
	colHairline = color.NRGBA{R: 0x22, G: 0x2E, B: 0x46, A: 0xFF}

	colCyan      = color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0xFF}
	colCyanDim   = color.NRGBA{R: 0x18, G: 0x9D, B: 0xB8, A: 0xFF}
	colCyanDeep  = color.NRGBA{R: 0x07, G: 0x4F, B: 0x68, A: 0xFF}
	colCyanGlow  = color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x33}
	colCyanFaint = color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x14}

	colViolet     = color.NRGBA{R: 0xB7, G: 0x8C, B: 0xFF, A: 0xFF}
	colVioletGlow = color.NRGBA{R: 0xB7, G: 0x8C, B: 0xFF, A: 0x30}

	colGreen   = color.NRGBA{R: 0x4A, G: 0xE3, B: 0x9C, A: 0xFF}
	colAmber   = color.NRGBA{R: 0xFF, G: 0xC4, B: 0x6B, A: 0xFF}
	colSOSRed  = color.NRGBA{R: 0xFF, G: 0x4A, B: 0x6E, A: 0xFF}
	colSOSGlow = color.NRGBA{R: 0xFF, G: 0x4A, B: 0x6E, A: 0x40}

	colTextHi  = color.NRGBA{R: 0xF1, G: 0xF6, B: 0xFF, A: 0xFF}
	colTextMid = color.NRGBA{R: 0xB6, G: 0xC2, B: 0xD8, A: 0xFF}
	colMuted   = color.NRGBA{R: 0x76, G: 0x86, B: 0xA3, A: 0xFF}
)

// silence unused-var warnings for palette aliases that some UI files may reach for.
var _ = []color.NRGBA{colCyanDim, colCyanDeep, colVioletGlow, colPanel3}

// ─── Theme ──────────────────────────────────────────────────────────────────

type bitlinkTheme struct{ fyne.Theme }

func newBitlinkTheme() fyne.Theme { return &bitlinkTheme{Theme: theme.DefaultTheme()} }

var (
	fontCacheMu sync.RWMutex
	fontCache   map[string]fyne.Resource = make(map[string]fyne.Resource)
)

func getLangFont() fyne.Resource {
	lang := CurrentLang()
	if lang == "en" {
		return nil
	}

	fontCacheMu.RLock()
	res, ok := fontCache[string(lang)]
	fontCacheMu.RUnlock()
	if ok {
		return res
	}

	var paths []string
	if lang == "kn" {
		// Tunga is the standard Windows Kannada font
		paths = []string{
			`C:\Windows\Fonts\tunga.ttf`,
			`C:\Windows\Fonts\Tunga.ttf`,
			`C:\Windows\Fonts\Nirmala.ttf`,
			`C:\Windows\Fonts\ARIALUNI.TTF`,
		}
	} else if lang == "hi" {
		// Mangal is the standard Windows Hindi font
		paths = []string{
			`C:\Windows\Fonts\mangal.ttf`,
			`C:\Windows\Fonts\Mangal.ttf`,
			`C:\Windows\Fonts\Nirmala.ttf`,
			`C:\Windows\Fonts\ARIALUNI.TTF`,
		}
	}

	var loaded fyne.Resource
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil {
			loaded = fyne.NewStaticResource(filepath.Base(p), b)
			break
		}
	}

	fontCacheMu.Lock()
	fontCache[string(lang)] = loaded
	fontCacheMu.Unlock()

	return loaded
}

func (t *bitlinkTheme) Font(s fyne.TextStyle) fyne.Resource {
	if f := getLangFont(); f != nil {
		return f
	}
	return t.Theme.Font(s)
}


func (t *bitlinkTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameButton:
		return colPanel2
	case theme.ColorNameDisabledButton:
		return colPanel
	case theme.ColorNameForeground:
		return colTextHi
	case theme.ColorNameForegroundOnPrimary:
		return colBg
	case theme.ColorNameDisabled:
		return colMuted
	case theme.ColorNamePlaceHolder:
		return colMuted
	case theme.ColorNameInputBackground:
		return colPanel2
	case theme.ColorNameInputBorder:
		return colHairline
	case theme.ColorNamePrimary:
		return colCyan
	case theme.ColorNameError:
		return colSOSRed
	case theme.ColorNameWarning:
		return colAmber
	case theme.ColorNameSuccess:
		return colGreen
	case theme.ColorNameSeparator:
		return colHairline
	case theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return colPanel
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x1A}
	case theme.ColorNameSelection:
		return color.NRGBA{R: 0x35, G: 0xF0, B: 0xFF, A: 0x33}
	case theme.ColorNameFocus:
		return colCyan
	case theme.ColorNameScrollBar:
		return colHairline
	}
	return t.Theme.Color(name, theme.VariantDark)
}

func (t *bitlinkTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 10
	case theme.SizeNameInnerPadding, theme.SizeNamePadding:
		return 8
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 11
	}
	return t.Theme.Size(name)
}

// ─── Self identity ──────────────────────────────────────────────────────────
//
// All access to selfName / selfColor goes through SelfName() / SelfColor() /
// setSelf(). The mutex eliminates the data race that existed when background
// goroutines (mesh advertiser, scanner, packet dispatcher) read these fields
// while the UI thread wrote them via the Settings tab.

var (
	selfMu    sync.RWMutex
	selfName  string
	selfColor = colCyan
	prefs     fyne.Preferences
)

// SelfName returns the current operator callsign in a goroutine-safe way.
func SelfName() string {
	selfMu.RLock()
	defer selfMu.RUnlock()
	return selfName
}

// SelfColor returns the current avatar color in a goroutine-safe way.
func SelfColor() color.NRGBA {
	selfMu.RLock()
	defer selfMu.RUnlock()
	return selfColor
}

func setSelf(name string, c color.NRGBA) {
	selfMu.Lock()
	selfName = name
	selfColor = c
	selfMu.Unlock()
	if prefs != nil {
		prefs.SetString("self.name", name)
		b, _ := json.Marshal([4]uint8{c.R, c.G, c.B, c.A})
		prefs.SetString("self.color", string(b))
	}
	if cb := getOnSelfChange(); cb != nil {
		cb()
	}
}

func loadSelf() {
	if prefs == nil {
		return
	}
	name := prefs.String("self.name")
	col := colCyan
	if s := prefs.String("self.color"); s != "" {
		var arr [4]uint8
		if err := json.Unmarshal([]byte(s), &arr); err == nil {
			col = color.NRGBA{R: arr[0], G: arr[1], B: arr[2], A: arr[3]}
		}
	}
	selfMu.Lock()
	selfName = name
	selfColor = col
	selfMu.Unlock()
	loadPairLock()
	loadOrInitMyPairCode()
}

// ─── Direct-pair (1-to-1 lock) ──────────────────────────────────────────────
//
// SECURITY NOTE: The original implementation derived the pair code from a
// hash of the username, which meant anyone who knew your callsign could
// compute your pair code. We now generate a 4-digit code at random on first
// launch, persist it, and broadcast it inside the BLE beacon's manufacturer
// data so peers can match against it without any handshake. The code is
// independent of the username.

var (
	pairLockMu   sync.RWMutex
	pairLockCode string

	myPairCodeMu sync.RWMutex
	myPairCode   string
)

// MyPairCode returns this device's 4-digit pair code, generating one on the
// first call if none has been persisted yet.
func MyPairCode() string {
	myPairCodeMu.RLock()
	c := myPairCode
	myPairCodeMu.RUnlock()
	if c != "" {
		return c
	}
	loadOrInitMyPairCode()
	myPairCodeMu.RLock()
	defer myPairCodeMu.RUnlock()
	return myPairCode
}

func loadOrInitMyPairCode() {
	myPairCodeMu.Lock()
	defer myPairCodeMu.Unlock()
	if prefs != nil {
		if s := prefs.String("pair.selfCode"); len(s) == 4 {
			myPairCode = s
			return
		}
	}
	// Generate a fresh 4-digit code from crypto/rand.
	var b [2]byte
	_, _ = rand.Read(b[:])
	n := int(binary.BigEndian.Uint16(b[:])) % 10000
	myPairCode = fmt.Sprintf("%04d", n)
	if prefs != nil {
		prefs.SetString("pair.selfCode", myPairCode)
	}
}

// MyPairCodeBytes returns the 4-digit code as a 2-byte big-endian unsigned
// integer for embedding in BLE manufacturer data. Returns {0,0} when the
// code is not yet generated.
func MyPairCodeBytes() [2]byte {
	c := MyPairCode()
	var out [2]byte
	if len(c) != 4 {
		return out
	}
	var n uint16
	for i := 0; i < 4; i++ {
		ch := c[i]
		if ch < '0' || ch > '9' {
			return [2]byte{}
		}
		n = n*10 + uint16(ch-'0')
	}
	binary.BigEndian.PutUint16(out[:], n)
	return out
}

// PairCodeFromBytes decodes a 2-byte big-endian unsigned integer into the
// 4-digit string used in the UI. Returns "" for codes outside [0, 9999].
func PairCodeFromBytes(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	n := binary.BigEndian.Uint16(b[:2])
	if n > 9999 {
		return ""
	}
	return fmt.Sprintf("%04d", n)
}

// PairLock returns the currently-locked peer code, or "" if unlocked.
func PairLock() string {
	pairLockMu.RLock()
	defer pairLockMu.RUnlock()
	return pairLockCode
}

// SetPairLock pins the UI to a single peer code (4 digits) or unlocks when
// passed an empty string. The change is persisted across launches.
func SetPairLock(code string) {
	pairLockMu.Lock()
	pairLockCode = code
	pairLockMu.Unlock()
	if prefs != nil {
		if code == "" {
			prefs.RemoveValue("pair.lockCode")
		} else {
			prefs.SetString("pair.lockCode", code)
		}
	}
	if cb := getOnSelfChange(); cb != nil {
		cb()
	}
}

func loadPairLock() {
	if prefs == nil {
		return
	}
	pairLockMu.Lock()
	pairLockCode = prefs.String("pair.lockCode")
	pairLockMu.Unlock()
}

// IsPeerNameVisible reports whether a peer with the given callsign should be
// shown in the UI given the current pair lock. Unlocked ⇒ everyone visible.
// When locked, we look up the peer's broadcast pair code in topology and
// match against the lock.
func IsPeerNameVisible(name string) bool {
	lock := PairLock()
	if lock == "" {
		return true
	}
	addr := TopoAddressForName(name)
	if addr == "" {
		return false
	}
	if n := topoNode(addr); n != nil && n.PairCode != "" {
		return n.PairCode == lock
	}
	return false
}

// performHardReset wipes EVERY persisted preference key BitLink owns.
func performHardReset() {
	Chat.mu.Lock()
	Chat.byPeer = map[string][]Message{}
	Chat.byGroup = map[string][]Message{}
	Chat.unread = map[string]int{}
	Chat.mu.Unlock()

	topoMu.Lock()
	topology = map[string]*Node{}
	topoMu.Unlock()
	nameMu.Lock()
	nameToAddr = map[string]string{}
	addrToName = map[string]string{}
	nameMu.Unlock()
	seenMu.Lock()
	seenIDs = map[string]time.Time{}
	seenQ = nil
	seenMu.Unlock()

	selfMu.Lock()
	selfName = ""
	selfColor = colCyan
	selfMu.Unlock()

	if prefs == nil {
		return
	}
	for _, key := range []string{
		"self.name",
		"self.color",
		"chat.byPeer",
		"chat.byGroup",
		"chat.unread",
		"contacts.items",
		"groups.items",
		"files.items",
		"safety.res",
		"safety.pins",
		"safety.vitals",
		"data.migrated.v2",
		"pair.lockCode",
		"pair.selfCode",
		"crypto.meshKey",
	} {
		prefs.RemoveValue(key)
	}
	pairLockMu.Lock()
	pairLockCode = ""
	pairLockMu.Unlock()
	myPairCodeMu.Lock()
	myPairCode = ""
	myPairCodeMu.Unlock()
}

func displayName() string {
	if n := SelfName(); n != "" {
		return n
	}
	return "Operator"
}

var (
	onSelfChangeMu sync.RWMutex
	onSelfChange   func()
)

func setOnSelfChange(f func()) {
	onSelfChangeMu.Lock()
	onSelfChange = f
	onSelfChangeMu.Unlock()
}

func getOnSelfChange() func() {
	onSelfChangeMu.RLock()
	defer onSelfChangeMu.RUnlock()
	return onSelfChange
}

// ─── safeGo — panic recovery wrapper ────────────────────────────────────────

func safeGo(label string, f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[BitLink] panic in %s: %v\n%s\n", label, r, debug.Stack())
			}
		}()
		f()
	}()
}

// ─── UI helpers ─────────────────────────────────────────────────────────────

func newCardPanel(content fyne.CanvasObject, pad float32) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel)
	bg.CornerRadius = 14
	bg.StrokeColor = colHairline
	bg.StrokeWidth = 1
	padded := container.New(layout.NewCustomPaddedLayout(pad, pad, pad, pad), content)
	return container.NewStack(bg, padded)
}

func newGlowCard(content fyne.CanvasObject, pad float32, accent color.NRGBA) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel)
	bg.CornerRadius = 16
	bg.StrokeColor = color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x55}
	bg.StrokeWidth = 1
	glow := canvas.NewRectangle(color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x10})
	glow.CornerRadius = 16
	padded := container.New(layout.NewCustomPaddedLayout(pad, pad, pad, pad), content)
	return container.NewStack(glow, bg, padded)
}

func newInsetPanel(content fyne.CanvasObject, pad float32) fyne.CanvasObject {
	bg := canvas.NewRectangle(colPanel2)
	bg.CornerRadius = 10
	padded := container.New(layout.NewCustomPaddedLayout(pad, pad, pad, pad), content)
	return container.NewStack(bg, padded)
}

func sectionHeader(title, sub string) fyne.CanvasObject {
	t := canvas.NewText(title, colTextHi)
	t.TextSize = 22
	t.TextStyle = fyne.TextStyle{Bold: true}
	s := canvas.NewText(sub, colMuted)
	s.TextSize = 10
	s.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	accent := canvas.NewRectangle(colCyan)
	accent.SetMinSize(fyne.NewSize(3, 22))
	accent.CornerRadius = 2
	titleBox := container.NewHBox(accent, spacer(8), t)
	return container.NewBorder(nil, nil, titleBox, container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), s))
}

func subHeader(text string) fyne.CanvasObject {
	t := canvas.NewText(strings.ToUpper(text), colMuted)
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	dot := canvas.NewCircle(colCyan)
	dot.Resize(fyne.NewSize(6, 6))
	dotBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(6, 6)), dot)
	return container.New(layout.NewCustomPaddedLayout(4, 4, 0, 0),
		container.NewHBox(container.NewCenter(dotBox), spacer(6), t))
}

func spacer(h float32) fyne.CanvasObject {
	r := canvas.NewRectangle(color.Transparent)
	r.SetMinSize(fyne.NewSize(1, h))
	return r
}

func pill(text string, fg, bg color.NRGBA) fyne.CanvasObject {
	t := canvas.NewText(text, fg)
	t.TextSize = 10
	t.TextStyle = fyne.TextStyle{Bold: true, Monospace: true}
	r := canvas.NewRectangle(bg)
	r.CornerRadius = 8
	r.StrokeColor = color.NRGBA{R: fg.R, G: fg.G, B: fg.B, A: 0x55}
	r.StrokeWidth = 1
	return container.NewStack(r, container.New(layout.NewCustomPaddedLayout(2, 2, 8, 8), t))
}

func statusDot(c color.NRGBA, size float32) fyne.CanvasObject {
	d := canvas.NewCircle(c)
	d.StrokeColor = color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0x55}
	d.StrokeWidth = 2
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(size, size)), d)
}

func signalBars(rssi int16) fyne.CanvasObject {
	pct := rssiToPct(rssi)
	bars := 0
	switch {
	case pct >= 75:
		bars = 4
	case pct >= 50:
		bars = 3
	case pct >= 25:
		bars = 2
	case pct > 0:
		bars = 1
	}
	hb := container.New(layout.NewCustomPaddedHBoxLayout(2))
	for i := 1; i <= 4; i++ {
		on := i <= bars
		c := colHairline
		if on {
			c = colCyan
		}
		r := canvas.NewRectangle(c)
		r.CornerRadius = 1
		h := float32(4 + i*2)
		w := canvas.NewRectangle(color.Transparent)
		w.SetMinSize(fyne.NewSize(3, 12))
		stack := container.NewStack(w, container.New(layout.NewCustomPaddedLayout(12-h, 0, 0, 0), r))
		hb.Add(container.New(layout.NewGridWrapLayout(fyne.NewSize(3, 12)), stack))
	}
	return hb
}

func avatarCircle(name string, fill color.NRGBA, diameter float32) fyne.CanvasObject {
	c := canvas.NewCircle(fill)
	c.StrokeColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x18}
	c.StrokeWidth = 1
	initials := initialsFor(name)
	t := canvas.NewText(initials, colBg)
	t.TextSize = diameter * 0.40
	t.Alignment = fyne.TextAlignCenter
	t.TextStyle = fyne.TextStyle{Bold: true}
	stack := container.NewStack(c, container.NewCenter(t))
	return container.New(layout.NewGridWrapLayout(fyne.NewSize(diameter, diameter)), stack)
}

func initialsFor(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	parts := strings.Fields(strings.ReplaceAll(name, "-", " "))
	if len(parts) == 1 {
		s := parts[0]
		if len(s) >= 2 {
			return strings.ToUpper(s[:2])
		}
		return strings.ToUpper(s)
	}
	return strings.ToUpper(parts[0][:1] + parts[1][:1])
}

func relTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return t.Format("02 Jan 15:04")
}

func shortAddr(a string) string {
	if len(a) > 8 {
		return a[:4] + "…" + a[len(a)-3:]
	}
	return a
}

// stripBLPrefix removes the "BL-" identity prefix from a broadcast LocalName.
func stripBLPrefix(s string) string {
	return strings.TrimPrefix(s, "BL-")
}

// peerDisplayName resolves a BLE address to the best human-readable name.
func peerDisplayName(addr string) string {
	if nick := Contacts.NicknameFor(addr); nick != "" {
		return nick
	}
	if n := topoNode(addr); n != nil && n.Name != "" {
		return stripBLPrefix(n.Name)
	}
	return shortAddr(addr)
}

func rssiToPct(r int16) int {
	if r == 0 {
		return 0
	}
	v := int((float64(r+90)/60.0)*100.0 + 0.5)
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	return v
}

// ─── Color swatch picker ────────────────────────────────────────────────────

type tappableSwatch struct {
	widget.BaseWidget
	dot   *canvas.Circle
	onTap func()
}

func newTappableSwatch(c color.NRGBA, onTap func()) *tappableSwatch {
	s := &tappableSwatch{dot: canvas.NewCircle(c), onTap: onTap}
	s.dot.StrokeColor = colMuted
	s.dot.StrokeWidth = 1
	s.ExtendBaseWidget(s)
	return s
}

func (s *tappableSwatch) MinSize() fyne.Size {
	return fyne.NewSize(34, 34)
}

func (s *tappableSwatch) CreateRenderer() fyne.WidgetRenderer {
	fixedDot := container.New(layout.NewGridWrapLayout(fyne.NewSize(28, 28)), s.dot)
	return widget.NewSimpleRenderer(container.NewCenter(fixedDot))
}

func (s *tappableSwatch) Tapped(_ *fyne.PointEvent) {
	if s.onTap != nil {
		s.onTap()
	}
}

func (s *tappableSwatch) TappedSecondary(_ *fyne.PointEvent) {}

type colorSwatchRow struct {
	Container *fyne.Container
	current   color.NRGBA
	OnChange  func(color.NRGBA)
	swatches  []*canvas.Circle
}

func newColorSwatchRow() *colorSwatchRow {
	row := &colorSwatchRow{}
	palette := []color.NRGBA{
		colCyan,
		colViolet,
		{R: 0xFF, G: 0x6B, B: 0x9E, A: 0xFF},
		colGreen,
		colAmber,
		colSOSRed,
	}
	hbox := container.NewHBox()
	for _, c := range palette {
		c := c
		sw := newTappableSwatch(c, func() { row.SetColor(c) })
		row.swatches = append(row.swatches, sw.dot)
		hbox.Add(sw)
	}
	row.Container = hbox
	return row
}

func (r *colorSwatchRow) SetColor(c color.NRGBA) {
	r.current = c
	for _, sw := range r.swatches {
		nrgba, ok := sw.FillColor.(color.NRGBA)
		if ok && nrgba == c {
			sw.StrokeColor = colCyan
			sw.StrokeWidth = 3
		} else {
			sw.StrokeColor = colMuted
			sw.StrokeWidth = 1
		}
		sw.Refresh()
	}
	if r.OnChange != nil {
		r.OnChange(c)
	}
}

func (r *colorSwatchRow) Color() color.NRGBA { return r.current }

func iconResource(_ string) fyne.Resource {
	return theme.NewThemedResource(theme.CancelIcon())
}

// ─── Notification banner ────────────────────────────────────────────────────

func showNotificationWithDuration(window fyne.Window, msg string, isAlert bool, d time.Duration) {
	if window == nil {
		return
	}
	accent := colCyan
	bgC := colPanel
	if isAlert {
		accent = colSOSRed
		bgC = color.NRGBA{R: 0x2A, G: 0x0E, B: 0x18, A: 0xFF}
	}
	bg := canvas.NewRectangle(bgC)
	bg.CornerRadius = 12
	bg.StrokeColor = color.NRGBA{R: accent.R, G: accent.G, B: accent.B, A: 0x88}
	bg.StrokeWidth = 1
	bar := canvas.NewRectangle(accent)
	bar.SetMinSize(fyne.NewSize(3, 0))
	bar.CornerRadius = 2
	lbl := canvas.NewText(msg, colTextHi)
	lbl.TextSize = 13
	lbl.TextStyle = fyne.TextStyle{Bold: true}
	icon := canvas.NewText("◆", accent)
	icon.TextSize = 14
	icon.TextStyle = fyne.TextStyle{Bold: true}
	row := container.NewBorder(nil, nil,
		container.NewHBox(spacer(2), bar, spacer(8), icon, spacer(4)),
		nil,
		container.New(layout.NewCustomPaddedLayout(8, 8, 4, 14), lbl),
	)
	body := container.NewStack(bg, row)
	pop := widget.NewPopUp(body, window.Canvas())
	winSize := window.Canvas().Size()
	pop.ShowAtPosition(fyne.NewPos(winSize.Width/2-180, 16))
	safeGo("notif-dismiss", func() {
		time.Sleep(d)
		fyne.Do(pop.Hide)
	})
}

func showNotification(window fyne.Window, msg string, isAlert bool) {
	showNotificationWithDuration(window, msg, isAlert, 3*time.Second)
}

// ─── Sidebar nav ────────────────────────────────────────────────────────────

type navItem struct {
	id    string
	label string
	icon  fyne.Resource
	build func(window fyne.Window) fyne.CanvasObject
	tab   interface{}
}

func sidebarButton(item navItem, active bool, onTap func()) fyne.CanvasObject {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 10
	if active {
		bg.FillColor = colCyanFaint
		bg.StrokeColor = color.NRGBA{R: colCyan.R, G: colCyan.G, B: colCyan.B, A: 0x55}
		bg.StrokeWidth = 1
	}

	hover := canvas.NewRectangle(color.Transparent)
	hover.CornerRadius = 10

	accent := canvas.NewRectangle(color.Transparent)
	if active {
		accent.FillColor = colCyan
		accent.CornerRadius = 2
	}
	accent.SetMinSize(fyne.NewSize(3, 22))
	accentBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(3, 22)), accent)

	icon := widget.NewIcon(item.icon)
	iconBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(20, 20)), icon)

	labelColor := colTextMid
	if active {
		labelColor = colTextHi
	}
	t := canvas.NewText(item.label, labelColor)
	t.TextSize = 14
	if active {
		t.TextStyle = fyne.TextStyle{Bold: true}
	}

	row := container.NewHBox(
		spacer(8),
		container.NewCenter(accentBox),
		spacer(10),
		container.NewCenter(iconBox),
		spacer(12),
		container.NewCenter(t),
	)
	rowPadded := container.New(layout.NewCustomPaddedLayout(8, 8, 0, 8), row)

	tap := newTappableArea(onTap)
	tap.onHover = func(in bool) {
		if active {
			return
		}
		if in {
			hover.FillColor = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0x10}
		} else {
			hover.FillColor = color.Transparent
		}
		hover.Refresh()
	}

	return container.NewStack(bg, hover, rowPadded, tap)
}

// ─── Tab index (atomic for safe cross-goroutine reads) ──────────────────────

var (
	currentTabIdxAtomic atomic.Int32
	currentTabNameMu    sync.RWMutex
	currentTabName      = "Home"
)

func currentlyOnTabName() string {
	currentTabNameMu.RLock()
	defer currentTabNameMu.RUnlock()
	return currentTabName
}

func setCurrentTab(idx int, name string) {
	currentTabIdxAtomic.Store(int32(idx))
	currentTabNameMu.Lock()
	currentTabName = name
	currentTabNameMu.Unlock()
}

func currentTabIdx() int { return int(currentTabIdxAtomic.Load()) }

// ─── Main ───────────────────────────────────────────────────────────────────

func main() {
	a := app.NewWithID("io.bitlink.app")
	a.Settings().SetTheme(newBitlinkTheme())

	prefs = a.Preferences()
	LoadLang()
	loadSelf()
	loadMeshKey()

	const dataMigrationKey = "data.migrated.v2"
	if prefs.String(dataMigrationKey) != "1" {
		prefs.RemoveValue("chat.byPeer")
		prefs.RemoveValue("chat.byGroup")
		prefs.RemoveValue("chat.unread")
		prefs.RemoveValue("files.items")
		prefs.SetString(dataMigrationKey, "1")
	}

	Chat.Bind(prefs)
	Contacts.Bind(prefs)
	Groups.Bind(prefs)
	Safety.Bind(prefs)

	docDir := filepath.Join(a.Storage().RootURI().Path(), "files")
	Files.Bind(prefs, docDir)

	w := a.NewWindow("BitLink — Offline Mesh Network")
	w.Resize(fyne.NewSize(1280, 800))

	home := buildHomeTab(w)
	chat := buildChatTab(w)
	groups := buildGroupsTab(w, nil)
	compassRef := buildCompassTab(w)
	safety := buildSafetyTab(w)
	contacts := buildContactsTab(w, func(name string) {
		if openChatPeer != nil {
			openChatPeer(name)
		}
	})
	_ = buildFilesTab(w) // files tab removed from sidebar but keep data layer
	mesh := buildMeshTab(w)
	settings := buildSettingsTab(w)

	type tabRef struct {
		nav     navItem
		content fyne.CanvasObject
		refresh func()
	}
	tabs := []tabRef{
		{nav: navItem{id: "home", label: T("nav.home"), icon: theme.HomeIcon()}, content: home.Container(), refresh: home.refresh},
		{nav: navItem{id: "chat", label: T("nav.chat"), icon: theme.MailComposeIcon()}, content: chat.Container(), refresh: chat.refresh},
		{nav: navItem{id: "groups", label: T("nav.groups"), icon: theme.AccountIcon()}, content: groups.Container(), refresh: groups.refresh},
		{nav: navItem{id: "compass", label: T("nav.compass"), icon: theme.SearchIcon()}, content: compassRef.Container(), refresh: compassRef.refresh},
		{nav: navItem{id: "safety", label: T("nav.safety"), icon: theme.WarningIcon()}, content: safety.Container(), refresh: safety.refresh},
		{nav: navItem{id: "contacts", label: T("nav.contacts"), icon: theme.ComputerIcon()}, content: contacts.Container(), refresh: contacts.refresh},
		{nav: navItem{id: "mesh", label: T("nav.mesh"), icon: theme.GridIcon()}, content: mesh.Container(), refresh: nil},
		{nav: navItem{id: "settings", label: T("nav.settings"), icon: theme.SettingsIcon()}, content: settings.Container(), refresh: nil},
	}

	contentArea := container.NewStack()
	switchTab := func(i int) {
		setCurrentTab(i, tabs[i].nav.label)
		contentArea.Objects = []fyne.CanvasObject{tabs[i].content}
		contentArea.Refresh()
		if tabs[i].refresh != nil {
			tabs[i].refresh()
		}
	}

	idxOf := func(id string) int {
		for i, t := range tabs {
			if t.nav.id == id {
				return i
			}
		}
		return 0
	}
	QuickActions.OpenChat = func() { switchTab(idxOf("chat")); rebuildNavLater() }
	QuickActions.OpenGroups = func() { switchTab(idxOf("groups")); rebuildNavLater() }
	QuickActions.OpenMap = func() { switchTab(idxOf("compass")); rebuildNavLater() }
	QuickActions.OpenSafety = func() { switchTab(idxOf("safety")); rebuildNavLater() }

	openChatPeer = func(name string) {
		switchTab(idxOf("chat"))
		rebuildNavLater()
		chat.openPeer(name)
	}

	logoMark := canvas.NewText("◈", colCyan)
	logoMark.TextSize = 22
	logoMark.TextStyle = fyne.TextStyle{Bold: true}
	logoText := canvas.NewText("BitLink", colTextHi)
	logoText.TextSize = 20
	logoText.TextStyle = fyne.TextStyle{Bold: true}
	logoTag := canvas.NewText("MESH OS", colMuted)
	logoTag.TextSize = 9
	logoTag.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	logoBox := container.NewVBox(
		container.NewHBox(logoMark, spacer(6), logoText),
		container.New(layout.NewCustomPaddedLayout(0, 0, 28, 0), logoTag),
	)

	profileAvatar := avatarCircle(displayName(), SelfColor(), 38)
	profileName := canvas.NewText(displayName(), colTextHi)
	profileName.TextSize = 13
	profileName.TextStyle = fyne.TextStyle{Bold: true}
	profileRole := canvas.NewText(T("status.mesh_operator"), colCyan)
	profileRole.TextSize = 8
	profileRole.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}

	profileStatusDot := canvas.NewCircle(colMuted)
	profileStatusDot.Resize(fyne.NewSize(7, 7))
	profileStatusText := canvas.NewText(T("status.offline"), colMuted)
	profileStatusText.TextSize = 8
	profileStatusText.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	statusRow := container.NewHBox(
		container.NewCenter(container.New(layout.NewGridWrapLayout(fyne.NewSize(7, 7)), profileStatusDot)),
		spacer(4),
		profileStatusText,
	)

	avatarHolder := container.NewStack(profileAvatar)
	updateProfileBlock := func() {
		avatarHolder.Objects = []fyne.CanvasObject{avatarCircle(displayName(), SelfColor(), 38)}
		avatarHolder.Refresh()
	}
	avatarBox := container.New(layout.NewGridWrapLayout(fyne.NewSize(46, 46)), avatarHolder)
	profileTextBlock := container.NewVBox(profileName, profileRole, statusRow)
	profileBlock := container.NewBorder(nil, nil, avatarBox, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 8, 0), profileTextBlock),
	)
	profileCard := newCardPanel(profileBlock, 10)
	setOnSelfChange(func() {
		fyne.Do(func() {
			profileName.Text = displayName()
			profileName.Refresh()
			updateProfileBlock()
		})
	})

	nav := container.NewVBox()
	rebuildNav := func() {
		nav.Objects = nil
		nav.Add(container.New(layout.NewCustomPaddedLayout(4, 0, 6, 6), subHeader(T("nav.navigation"))))
		for i, tr := range tabs {
			i := i
			nav.Add(sidebarButton(tr.nav, i == currentTabIdx(), func() {
				switchTab(i)
				rebuildNavLater()
			}))
			nav.Add(spacer(2))
		}
		nav.Refresh()
	}
	rebuildNavLater = func() { fyne.Do(rebuildNav) }

	sosBg := canvas.NewRectangle(colSOSRed)
	sosBg.CornerRadius = 12
	sosBg.StrokeColor = colSOSRed
	sosBg.StrokeWidth = 1
	sosGlow := canvas.NewRectangle(colSOSGlow)
	sosGlow.CornerRadius = 14
	sosLabel := canvas.NewText("◉  "+T("sos.broadcast"), colTextHi)
	sosLabel.TextSize = 13
	sosLabel.TextStyle = fyne.TextStyle{Bold: true}
	sosLabel.Alignment = fyne.TextAlignCenter
	sosTap := newTappableArea(func() {
		entry := widget.NewEntry()
		entry.SetPlaceHolder("e.g. Trapped, 3rd floor, Bldg A")
		dialog.ShowForm("Broadcast SOS", "Broadcast", "Cancel",
			[]*widget.FormItem{{Text: "Message", Widget: entry}},
			func(ok bool) {
				if !ok {
					return
				}
				txt := entry.Text
				if txt == "" {
					txt = "HELP"
				}
				BroadcastSOSWithText(txt)
				showNotification(w, "SOS broadcast: "+txt, true)
			}, w)
	})
	sosCard := container.NewStack(sosGlow,
		container.New(layout.NewCustomPaddedLayout(2, 2, 2, 2), container.NewStack(
			sosBg,
			container.New(layout.NewCustomPaddedLayout(10, 10, 0, 0), sosLabel),
			sosTap,
		)),
	)

	netLabel := canvas.NewText("0 peers • 0 linked", colMuted)
	netLabel.TextSize = 10
	netLabel.TextStyle = fyne.TextStyle{Monospace: true, Bold: true}
	netDot := canvas.NewCircle(colMuted)
	netDot.Resize(fyne.NewSize(7, 7))
	netRow := container.NewHBox(
		spacer(4),
		container.NewCenter(container.New(layout.NewGridWrapLayout(fyne.NewSize(7, 7)), netDot)),
		spacer(6),
		netLabel,
	)

	sidebar := container.NewBorder(
		container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(8, 8, 8, 8), logoBox),
			container.New(layout.NewCustomPaddedLayout(0, 4, 8, 8), profileCard),
		),
		container.NewVBox(
			container.New(layout.NewCustomPaddedLayout(0, 4, 10, 10), netRow),
			container.New(layout.NewCustomPaddedLayout(0, 10, 8, 8), sosCard),
		),
		nil, nil,
		container.New(layout.NewCustomPaddedLayout(2, 4, 4, 4), nav),
	)
	sidebarBg := canvas.NewRectangle(colSidebar)
	rightDivider := canvas.NewRectangle(colHairline)
	rightDivider.SetMinSize(fyne.NewSize(1, 0))
	sidebarStack := container.NewStack(sidebarBg, container.NewBorder(nil, nil, nil, rightDivider, sidebar))

	split := container.NewHSplit(
		container.New(layout.NewGridWrapLayout(fyne.NewSize(232, 760)), sidebarStack),
		contentArea,
	)
	split.Offset = 0.0
	w.SetContent(split)

	updateNetStatus := func() {
		peers := len(TopologySnapshot())
		linked := ActivePeerCount()
		netLabel.Text = fmt.Sprintf("%d peer%s • %d linked", peers, plural(peers), linked)
		profileStatusText.Text = T("status.offline")
		profileStatusText.Color = colMuted
		netDot.FillColor = colMuted
		if linked > 0 {
			profileStatusText.Text = T("status.online")
			profileStatusText.Color = colGreen
			netDot.FillColor = colGreen
		} else if peers > 0 {
			profileStatusText.Text = T("status.discovered")
			profileStatusText.Color = colAmber
			netDot.FillColor = colAmber
		}
		profileStatusText.Refresh()
		netLabel.Refresh()
		netDot.Refresh()
	}
	safeGo("net-status-tick", func() {
		for {
			time.Sleep(2 * time.Second)
			fyne.Do(updateNetStatus)
		}
	})

	SetSOSCallback(func(ev SOSEvent) {
		fyne.Do(func() {
			showSOSAlert(w, ev)
		})
	})
	SetThreatCallback(func(text string) {
		fyne.Do(func() { showNotification(w, "THREAT: "+text, true) })
	})
	onGroupInvite = func(group, from string) {
		fyne.Do(func() {
			showNotification(w, fmt.Sprintf("Group invite: %s (from %s)", group, peerDisplayName(from)), false)
			home.refresh()
			groups.refresh()
		})
	}

	prevChatListener := Chat.listener
	Chat.SetListener(func() {
		if prevChatListener != nil {
			prevChatListener()
		}
		fyne.Do(func() {
			home.refresh()
			chat.refresh()
			groups.refresh()
		})
	})
	
	prevContactsListener := Contacts.listener
	Contacts.SetListener(func() {
		if prevContactsListener != nil {
			prevContactsListener()
		}
		fyne.Do(func() {
			contacts.refresh()
			chat.refresh()
			home.refresh()
		})
	})

	if SelfName() == "" {
		showFirstLaunchSetup(w, func() { startBLE(w, home, contacts, chat, groups, mesh) })
	} else {
		startBLE(w, home, contacts, chat, groups, mesh)
	}

	rebuildNav()
	switchTab(0)
	w.ShowAndRun()
}

var openChatPeer func(name string)

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

var rebuildNavLater = func() {}

func startBLE(w fyne.Window, home *homeTab, contacts *contactsTab, chat *chatTab, groups *groupsTab, mesh *meshTab) {
	_ = mesh
	_ = groups
	safeGo("ble-listener", func() {
		StartMeshListener(
			func(addr string, rssi int16, name string) {
				if c, ok := Contacts.Get(addr); ok {
					_ = c
					Contacts.Touch(addr, rssi)
				}
				fyne.Do(func() { home.refresh(); contacts.refresh(); chat.refresh() })
			},
			func(p Packet, source string, rssi int16) {
				fyne.Do(func() {
					switch p.Type {
					case PktChat:
						// Recipient gate — only accept direct messages addressed
						// to us (or with no recipient set, for legacy clients).
						if p.Recipient != "" && p.Recipient != SelfName() {
							return
						}
						Chat.ReceiveDirect(source, p)
						if currentlyOnTabName() != "Home" && currentlyOnTabName() != "Chat" {
							showNotification(w, fmt.Sprintf("%s: %s", p.Sender, string(p.Data)), false)
						}
					case PktGroup:
						Chat.ReceiveGroup(p)
					case PktAck:
						Chat.HandleAck(p)
					case PktSOS:
						HandleIncomingSOS(string(p.Data), source, rssi)
					case PktThreat:
						HandleIncomingThreat(string(p.Data))
					case PktFileMeta:
						if p.Recipient != "" && p.Recipient != SelfName() {
							return
						}
						Files.HandleMeta(p)
					case PktFileChunk:
						if p.Recipient != "" && p.Recipient != SelfName() {
							return
						}
						Files.HandleChunk(p)
					case PktFileEnd:
						if p.Recipient != "" && p.Recipient != SelfName() {
							return
						}
						if e, ok := Files.HandleEnd(p); ok {
							showNotification(w, "File received: "+e.Name, false)
						}
					case PktGroupInvite:
						HandleIncomingGroupInvite(string(p.Data), source)
					}
				})
			},
			func(err error) {
				fmt.Println("[BitLink] Connection error:", err)
			},
		)
	})
}
