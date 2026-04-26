package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"tinygo.org/x/bluetooth"
)

// ─── BLE adapter & state ─────────────────────────────────────────────────────
//
// NOTE (v6 mesh): connection-less mesh. Every node continuously rotates its
// BLE advertisement between a "beacon" frame (LocalName + manufacturer tag
// "BL"+pairCode) and one or more "chunk" frames (manufacturer data carrying
// pieces of an outbound Packet). No adapter.Connect() calls anywhere.

var adapter = bluetooth.DefaultAdapter

// Local GATT characteristic handles (kept registered so old-build peers that
// still try to GATT-write to us can deliver packets — purely additive).
var bitlinkChatChar bluetooth.Characteristic
var bitlinkFileChar bluetooth.Characteristic
var bitlinkGroupChar bluetooth.Characteristic
var bitlinkSOSChar bluetooth.Characteristic
var bitlinkThreatChar bluetooth.Characteristic

var (
	nameMu     sync.RWMutex
	nameToAddr = map[string]string{}
	addrToName = map[string]string{}
)

// Topology cache (address -> Node)
type Node struct {
	Address   string
	Name      string
	RSSI      int16
	LastSeen  time.Time
	Connected bool
	PairCode  string // peer's broadcast 4-digit pair code, empty if unknown
}

var (
	topoMu   sync.RWMutex
	topology = map[string]*Node{}
)

// Dedup cache (last 200 packet IDs)
var (
	seenMu  sync.Mutex
	seenIDs = map[string]time.Time{}
	seenQ   []string
)

var (
	onPacket    func(p Packet, source string, rssi int16)
	onDiscover  func(addr string, rssi int16, name string)
	onConnError func(err error)
)

// ─── UUIDs ───────────────────────────────────────────────────────────────────

var bitlinkServiceUUID = bluetooth.NewUUID([16]byte{
	0xBB, 0xBB, 0x00, 0x01, 0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
})

var bitlinkChatUUID = bluetooth.NewUUID([16]byte{
	0xBB, 0xBB, 0x00, 0x02, 0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
})

var bitlinkFileUUID = bluetooth.NewUUID([16]byte{
	0xBB, 0xBB, 0x00, 0x03, 0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
})

var bitlinkGroupUUID = bluetooth.NewUUID([16]byte{
	0xBB, 0xBB, 0x00, 0x04, 0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
})

var bitlinkSOSUUID = bluetooth.NewUUID([16]byte{
	0xBB, 0xBB, 0x00, 0x05, 0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
})

var bitlinkThreatUUID = bluetooth.NewUUID([16]byte{
	0xBB, 0xBB, 0x00, 0x06, 0x00, 0x00, 0x10, 0x00,
	0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB,
})

// ─── Packet types ────────────────────────────────────────────────────────────

const (
	PktChat        byte = 1
	PktGroup       byte = 2
	PktAck         byte = 3
	PktSOS         byte = 4
	PktThreat      byte = 5
	PktFileMeta    byte = 6
	PktFileChunk   byte = 7
	PktFileEnd     byte = 8
	PktGroupInvite byte = 9
)

// Packet — unified BitLink mesh frame (binary). Wire-compatible with v5.
type Packet struct {
	Type      byte
	TTL       byte
	ID        [8]byte
	Sender    string
	Recipient string
	Group     string
	Timestamp time.Time
	Data      []byte
}

// MaxPacketDataSize caps the Data field so a single Packet always fits within
// the 255-chunk × 16-byte ceiling of our advertisement-based mesh.
const MaxPacketDataSize = 3500 // 4080 chunk bytes minus headroom for headers

func (p *Packet) Encode() []byte {
	var buf bytes.Buffer
	buf.WriteByte(0xBB)
	buf.WriteByte(0xBB)
	buf.WriteByte(0x01)
	buf.WriteByte(p.Type)
	buf.WriteByte(p.TTL)
	buf.Write(p.ID[:])
	writeStr := func(s string) {
		b := []byte(s)
		if len(b) > 255 {
			b = b[:255]
		}
		buf.WriteByte(byte(len(b)))
		buf.Write(b)
	}
	writeStr(p.Sender)
	writeStr(p.Recipient)
	writeStr(p.Group)
	ts := p.Timestamp.UnixNano()
	if ts <= 0 {
		ts = time.Now().UnixNano()
	}
	_ = binary.Write(&buf, binary.BigEndian, ts)
	dl := uint16(len(p.Data))
	_ = binary.Write(&buf, binary.BigEndian, dl)
	buf.Write(p.Data)
	return buf.Bytes()
}

func DecodePacket(b []byte) (Packet, error) {
	var p Packet
	if len(b) < 5 || b[0] != 0xBB || b[1] != 0xBB {
		return p, errors.New("bad magic")
	}
	r := bytes.NewReader(b[2:])
	var ver, typ, ttl byte
	if err := binary.Read(r, binary.BigEndian, &ver); err != nil {
		return p, err
	}
	if err := binary.Read(r, binary.BigEndian, &typ); err != nil {
		return p, err
	}
	if err := binary.Read(r, binary.BigEndian, &ttl); err != nil {
		return p, err
	}
	p.Type = typ
	p.TTL = ttl
	if _, err := io.ReadFull(r, p.ID[:]); err != nil {
		return p, err
	}
	readStr := func() (string, error) {
		var l byte
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return "", err
		}
		if l == 0 {
			return "", nil
		}
		bb := make([]byte, l)
		if _, err := io.ReadFull(r, bb); err != nil {
			return "", err
		}
		return string(bb), nil
	}
	var err error
	if p.Sender, err = readStr(); err != nil {
		return p, err
	}
	if p.Recipient, err = readStr(); err != nil {
		return p, err
	}
	if p.Group, err = readStr(); err != nil {
		return p, err
	}
	var ts int64
	if err := binary.Read(r, binary.BigEndian, &ts); err != nil {
		return p, err
	}
	p.Timestamp = time.Unix(0, ts)
	var dl uint16
	if err := binary.Read(r, binary.BigEndian, &dl); err != nil {
		return p, err
	}
	if dl > MaxPacketDataSize {
		return p, fmt.Errorf("packet data too large: %d > %d", dl, MaxPacketDataSize)
	}
	if dl > 0 {
		data := make([]byte, dl)
		if _, err := io.ReadFull(r, data); err != nil {
			return p, err
		}
		p.Data = data
	}
	return p, nil
}

// ─── Dedup helper ────────────────────────────────────────────────────────────

func packetSeen(id [8]byte) bool {
	key := fmt.Sprintf("%x", id[:])
	seenMu.Lock()
	defer seenMu.Unlock()
	if _, ok := seenIDs[key]; ok {
		return true
	}
	seenIDs[key] = time.Now()
	seenQ = append(seenQ, key)
	if len(seenQ) > 200 {
		oldest := seenQ[0]
		seenQ = seenQ[1:]
		delete(seenIDs, oldest)
	}
	return false
}

// ─── GATT service registration (back-compat with old build) ─────────────────

func registerGATT() error {
	mk := func(handle *bluetooth.Characteristic, uuid bluetooth.UUID, label string) bluetooth.CharacteristicConfig {
		return bluetooth.CharacteristicConfig{
			Handle: handle,
			UUID:   uuid,
			Flags: bluetooth.CharacteristicWritePermission |
				bluetooth.CharacteristicWriteWithoutResponsePermission |
				bluetooth.CharacteristicNotifyPermission |
				bluetooth.CharacteristicReadPermission,
			WriteEvent: func(_ bluetooth.Connection, _ int, value []byte) {
				_ = label
				handleIncoming(value, "gatt:"+label, 0)
			},
		}
	}
	return adapter.AddService(&bluetooth.Service{
		UUID: bitlinkServiceUUID,
		Characteristics: []bluetooth.CharacteristicConfig{
			mk(&bitlinkChatChar, bitlinkChatUUID, "chat"),
			mk(&bitlinkFileChar, bitlinkFileUUID, "file"),
			mk(&bitlinkGroupChar, bitlinkGroupUUID, "group"),
			mk(&bitlinkSOSChar, bitlinkSOSUUID, "sos"),
			mk(&bitlinkThreatChar, bitlinkThreatUUID, "threat"),
		},
	})
}

// ─── Public Mesh API ─────────────────────────────────────────────────────────

func StartMeshListener(
	onNodeDiscovered func(addr string, rssi int16, name string),
	onPacketReceived func(p Packet, source string, rssi int16),
	onErr func(err error),
) {
	onDiscover = onNodeDiscovered
	onPacket = onPacketReceived
	onConnError = onErr

	if err := adapter.Enable(); err != nil {
		fmt.Println("[BitLink] adapter.Enable FAILED:", err)
		fmt.Println("[BitLink] Bluetooth is unavailable on this PC.")
		fmt.Println("[BitLink]   - Windows: turn Bluetooth ON in Settings.")
		fmt.Println("[BitLink]   - Linux: ensure bluetoothd is running and run with sudo,")
		fmt.Println("[BitLink]            or grant: sudo setcap 'cap_net_raw,cap_net_admin+eip' ./bitlink")
		fmt.Println("[BitLink]   - macOS: grant Bluetooth permission when prompted.")
		if onConnError != nil {
			onConnError(err)
		}
		return
	}
	if err := registerGATT(); err != nil {
		fmt.Println("[BitLink] registerGATT:", err)
	}

	fmt.Println("[BitLink] BLE adapter enabled. Advertising as:", selfAdvName())
	fmt.Println("[BitLink] Pair code:", MyPairCode(), " — looking for nearby BL- peers...")

	go advertiserLoop()
	go scanLoop()
	go reaper()

	// Bring up the Wi-Fi fallback in parallel with BLE. On Windows ↔ Windows
	// pairs (where BLE peripheral mode is blocked by the OS driver) this is
	// what actually carries discovery and chat. On working BLE setups it just
	// duplicates frames and the dedup ring drops them.
	startLAN()
}

// ─── Advertisement engine ────────────────────────────────────────────────────

const (
	chunkPayloadMax  = 16
	chunkMagic0      = 'B'
	chunkMagic1      = 'M'
	chunkRepeats     = 2
	chunkAirtime     = 220 * time.Millisecond
	beaconAirtime    = 1500 * time.Millisecond
	rxAssemblyTTL    = 12 * time.Second
	peerActiveWindow = 30 * time.Second
	maxRxBuffers     = 64 // hard cap to prevent partial-chunk DoS
)

// advState tracks which kind of frame is currently on air so the loop can
// avoid re-issuing identical beacon configurations. Constant Stop/Configure/
// Start cycles cause some BLE stacks (notably Windows WinRT and BlueZ) to
// drop the broadcast entirely between cycles — the symptom users see as
// "the other PC isn't visible". Holding a steady beacon when idle is the
// single most important reliability fix.
type advState int

const (
	advNone advState = iota
	advBeacon
	advSilent
	advChunk
)

var currentAdv advState

type pendingTx struct {
	chunks   [][]byte
	enqueued time.Time
}

var (
	advMu    sync.Mutex
	advLive  bool
	txMu     sync.Mutex
	txQueue  []*pendingTx
	chunkSeq uint16

	// quietBeaconUntil suppresses the identity-bearing beacon between cycles
	// so anonymous broadcasts (PktThreat) don't get correlated with the
	// sender's beacon name in the exact same time window.
	quietMu          sync.Mutex
	quietBeaconUntil time.Time
)

func init() {
	// Seed chunkSeq from crypto/rand so msg-IDs from a fresh restart don't
	// collide with any in-flight reassembly buffers on neighbouring peers.
	var b [2]byte
	_, _ = rand.Read(b[:])
	chunkSeq = binary.BigEndian.Uint16(b[:])
	if chunkSeq == 0 {
		chunkSeq = 1
	}
}

// SetBeaconQuietWindow suppresses identity beacons for d. Used by anonymous
// broadcasts (threat reports) so the BLE name isn't visible at the same time
// the binary packet is on air.
func SetBeaconQuietWindow(d time.Duration) {
	quietMu.Lock()
	quietBeaconUntil = time.Now().Add(d)
	quietMu.Unlock()
}

func beaconQuiet() bool {
	quietMu.Lock()
	defer quietMu.Unlock()
	return time.Now().Before(quietBeaconUntil)
}

// nextMsgID derives a 4-byte message id from a Packet ID + a rolling sequence.
func nextMsgID(packetID [8]byte) [4]byte {
	txMu.Lock()
	chunkSeq++
	seq := chunkSeq
	txMu.Unlock()
	var out [4]byte
	out[0] = packetID[0] ^ packetID[4]
	out[1] = packetID[1] ^ packetID[5]
	out[2] = byte(seq >> 8)
	out[3] = byte(seq & 0xFF)
	return out
}

// chunkPacket splits an encoded Packet into airtime-sized manufacturer
// payloads. Returns nil and an error if the packet exceeds the 255-chunk cap.
func chunkPacket(p Packet) ([][]byte, error) {
	enc := p.Encode()
	msgID := nextMsgID(p.ID)
	totalChunks := (len(enc) + chunkPayloadMax - 1) / chunkPayloadMax
	if totalChunks == 0 {
		totalChunks = 1
	}
	if totalChunks > 255 {
		return nil, fmt.Errorf("packet too large: %d bytes => %d chunks (max 255)", len(enc), totalChunks)
	}
	out := make([][]byte, 0, totalChunks)
	for i := 0; i < totalChunks; i++ {
		start := i * chunkPayloadMax
		end := start + chunkPayloadMax
		if end > len(enc) {
			end = len(enc)
		}
		body := enc[start:end]
		buf := make([]byte, 0, 8+len(body))
		buf = append(buf, chunkMagic0, chunkMagic1)
		buf = append(buf, msgID[:]...)
		buf = append(buf, byte(i), byte(totalChunks))
		buf = append(buf, body...)
		out = append(out, buf)
	}
	return out, nil
}

func enqueuePacketForBroadcast(p Packet) {
	chunks, err := chunkPacket(p)
	if err != nil {
		fmt.Println("[BitLink] enqueue:", err)
		return
	}
	if len(chunks) == 0 {
		return
	}
	item := &pendingTx{chunks: chunks, enqueued: time.Now()}
	txMu.Lock()
	txQueue = append(txQueue, item)
	if len(txQueue) > 32 {
		fmt.Println("[BitLink] txQueue overflow; dropping oldest packets")
		txQueue = txQueue[len(txQueue)-32:]
	}
	txMu.Unlock()
}

// advertiserLoop is the only place that calls into adapter.DefaultAdvertisement().
//
// Strategy:
//   - When idle: hold the identity beacon up steadily. Re-issue it ONLY when
//     the desired state actually changes (entering/leaving the quiet window
//     or our callsign changing). Continuous Stop/Configure/Start cycles are
//     what historically prevented peer discovery on Windows and Linux.
//   - When sending: pause to broadcast the chunk frames, then restore the
//     beacon once at the end (not after every chunk).
func advertiserLoop() {
	// Bring up the identity beacon immediately so peers see us right away.
	desiredIdle()
	lastBeaconName := selfAdvName()

	for {
		txMu.Lock()
		var item *pendingTx
		if len(txQueue) > 0 {
			item = txQueue[0]
			txQueue = txQueue[1:]
		}
		txMu.Unlock()

		if item == nil {
			// Re-configure ONLY when the desired state actually changes.
			name := selfAdvName()
			want := advBeacon
			if beaconQuiet() {
				want = advSilent
			}
			if currentAdv != want || name != lastBeaconName {
				desiredIdle()
				lastBeaconName = name
			}
			time.Sleep(beaconAirtime)
			continue
		}

		// Send a packet: switch to chunk frames for the duration, then
		// restore the beacon once at the end.
		for r := 0; r < chunkRepeats; r++ {
			for _, chunk := range item.chunks {
				setChunkAdvertisement(chunk)
				time.Sleep(chunkAirtime)
			}
		}
		desiredIdle()
		lastBeaconName = selfAdvName()
	}
}

// desiredIdle puts the radio into the correct idle frame (identity beacon or
// silent beacon) for the current quiet-window state.
func desiredIdle() {
	if beaconQuiet() {
		setSilentBeaconAdvertisement()
	} else {
		setBeaconAdvertisement()
	}
}

// setBeaconAdvertisement broadcasts the discovery beacon: LocalName "BL-<callsign>"
// plus a manufacturer marker "BL"+pairCode(2 bytes) so peers can match pair locks.
func setBeaconAdvertisement() {
	pc := MyPairCodeBytes()
	mfg := []byte{'B', 'L', pc[0], pc[1]}
	startAdvertising(true, []bluetooth.ManufacturerDataElement{
		{CompanyID: 0xFFFF, Data: mfg},
	})
	currentAdv = advBeacon
}

// setSilentBeaconAdvertisement broadcasts the same payload but without the
// LocalName, so the sender's identity isn't visible during anonymous bursts.
func setSilentBeaconAdvertisement() {
	pc := MyPairCodeBytes()
	mfg := []byte{'B', 'L', pc[0], pc[1]}
	startAdvertising(false, []bluetooth.ManufacturerDataElement{
		{CompanyID: 0xFFFF, Data: mfg},
	})
	currentAdv = advSilent
}

func setChunkAdvertisement(payload []byte) {
	startAdvertising(false, []bluetooth.ManufacturerDataElement{
		{CompanyID: 0xFFFF, Data: payload},
	})
	currentAdv = advChunk
}

func startAdvertising(withIdentity bool, extraMfg []bluetooth.ManufacturerDataElement) {
	advMu.Lock()
	defer advMu.Unlock()

	adv := adapter.DefaultAdvertisement()
	if advLive {
		_ = adv.Stop()
	}
	opts := bluetooth.AdvertisementOptions{}
	if withIdentity {
		opts.LocalName = selfAdvName()
	}
	if len(extraMfg) > 0 {
		opts.ManufacturerData = extraMfg
	} else {
		opts.ManufacturerData = []bluetooth.ManufacturerDataElement{
			{CompanyID: 0xFFFF, Data: []byte("BL")},
		}
	}
	if err := adv.Configure(opts); err != nil {
		fmt.Println("[BitLink] advertise.configure:", err)
		return
	}
	if err := adv.Start(); err != nil {
		fmt.Println("[BitLink] advertise.start:", err)
		return
	}
	advLive = true
}

// ─── Scanner & chunk reassembly ──────────────────────────────────────────────

type rxAssembly struct {
	total    byte
	received map[byte][]byte
	started  time.Time
	source   string
	rssi     int16
}

var (
	rxMu      sync.Mutex
	rxBuffers = map[[4]byte]*rxAssembly{}
)

func scanLoop() {
	err := adapter.Scan(func(_ *bluetooth.Adapter, result bluetooth.ScanResult) {
		addr := result.Address.String()
		name := result.LocalName()

		if name != "" && name == selfAdvName() {
			return
		}

		isBitlinkPeer := strings.HasPrefix(name, "BL-")

		// Track peer pair code from beacon mfg data (4-byte: 'B','L',codeHi,codeLo)
		var peerPairCode string

		for _, m := range result.ManufacturerData() {
			if m.CompanyID != 0xFFFF {
				continue
			}
			data := m.Data
			if len(data) >= 8 && data[0] == chunkMagic0 && data[1] == chunkMagic1 {
				handleChunkFrame(data, addr, result.RSSI)
				continue
			}
			if len(data) >= 2 && data[0] == 'B' && data[1] == 'L' {
				if len(data) >= 4 {
					peerPairCode = PairCodeFromBytes(data[2:4])
				}
				continue
			}
			if isBitlinkPeer {
				handleManufacturer(string(data), addr, result.RSSI)
			}
		}

		if !isBitlinkPeer {
			return
		}

		updateTopology(addr, name, result.RSSI, peerPairCode)
		if onDiscover != nil {
			onDiscover(addr, result.RSSI, name)
		}
	})
	if err != nil {
		fmt.Println("[BitLink] scan err:", err)
		if onConnError != nil {
			onConnError(err)
		}
	}
}

func handleChunkFrame(frame []byte, source string, rssi int16) {
	if len(frame) < 8 {
		return
	}
	var msgID [4]byte
	copy(msgID[:], frame[2:6])
	idx := frame[6]
	total := frame[7]
	body := frame[8:]
	if total == 0 || idx >= total {
		return
	}

	rxMu.Lock()
	ra, ok := rxBuffers[msgID]
	if !ok {
		// Cap on number of in-flight reassembly buffers to defeat DoS
		// from random partial-chunk floods.
		if len(rxBuffers) >= maxRxBuffers {
			// Evict the oldest buffer instead of unbounded growth.
			var oldestKey [4]byte
			var oldestStart time.Time
			first := true
			for k, v := range rxBuffers {
				if first || v.started.Before(oldestStart) {
					oldestKey = k
					oldestStart = v.started
					first = false
				}
			}
			delete(rxBuffers, oldestKey)
		}
		ra = &rxAssembly{
			total:    total,
			received: map[byte][]byte{},
			started:  time.Now(),
			source:   source,
			rssi:     rssi,
		}
		rxBuffers[msgID] = ra
	}
	if _, dup := ra.received[idx]; dup {
		rxMu.Unlock()
		return
	}
	cp := make([]byte, len(body))
	copy(cp, body)
	ra.received[idx] = cp
	complete := len(ra.received) == int(ra.total)
	if !complete {
		rxMu.Unlock()
		return
	}
	full := make([]byte, 0, int(ra.total)*chunkPayloadMax)
	for i := byte(0); i < ra.total; i++ {
		piece, ok := ra.received[i]
		if !ok {
			rxMu.Unlock()
			return
		}
		full = append(full, piece...)
	}
	delete(rxBuffers, msgID)
	src := ra.source
	rs := ra.rssi
	rxMu.Unlock()

	handleIncoming(full, src, rs)
}

func reaper() {
	for {
		time.Sleep(4 * time.Second)
		now := time.Now()
		rxMu.Lock()
		for k, ra := range rxBuffers {
			if now.Sub(ra.started) > rxAssemblyTTL {
				delete(rxBuffers, k)
			}
		}
		rxMu.Unlock()

		topoMu.Lock()
		for addr, n := range topology {
			if now.Sub(n.LastSeen) > 2*time.Minute {
				delete(topology, addr)
				// Also drop identity mappings so TopoAddressForName stays consistent
				nameMu.Lock()
				if name, ok := addrToName[addr]; ok {
					delete(nameToAddr, name)
					delete(addrToName, addr)
				}
				nameMu.Unlock()
			} else {
				n.Connected = now.Sub(n.LastSeen) <= peerActiveWindow
			}
		}
		topoMu.Unlock()

		// Reap stalled file receivers (>30s with no progress).
		Files.reapStalled(30 * time.Second)
	}
}

func updateTopology(addr, name string, rssi int16, pairCode string) {
	if !strings.HasPrefix(name, "BL-") {
		return
	}
	cleanName := name[3:]
	if cleanName == "" {
		return
	}

	topoMu.Lock()
	defer topoMu.Unlock()

	nameMu.Lock()
	oldAddr, hadOld := nameToAddr[cleanName]
	if hadOld && oldAddr != addr {
		delete(topology, oldAddr)
	}
	// Drop any reverse entry pointing at the old address.
	if prev, ok := addrToName[addr]; ok && prev != cleanName {
		if mapped, ok := nameToAddr[prev]; ok && mapped == addr {
			delete(nameToAddr, prev)
		}
	}
	nameToAddr[cleanName] = addr
	addrToName[addr] = cleanName
	nameMu.Unlock()

	n, ok := topology[addr]
	if !ok {
		n = &Node{Address: addr}
		topology[addr] = n
	}
	n.Name = name
	n.RSSI = rssi
	n.LastSeen = time.Now()
	n.Connected = true
	if pairCode != "" {
		n.PairCode = pairCode
	}
}

// ─── Inbound dispatch ────────────────────────────────────────────────────────

func handleIncoming(raw []byte, source string, rssi int16) {
	if len(raw) >= 5 && raw[0] == 0xBB && raw[1] == 0xBB {
		p, err := DecodePacket(raw)
		if err != nil {
			fmt.Println("[BitLink] decode:", err)
			return
		}
		// Decrypt Data field if a mesh key is configured.
		if dec, ok := tryDecryptData(p.Data); ok {
			p.Data = dec
		}
		if packetSeen(p.ID) {
			return
		}
		if source == "loopback" || source == "self" {
			return
		}
		if p.Sender != "" && p.Sender == SelfName() {
			return
		}
		if onPacket != nil {
			onPacket(p, source, rssi)
		}
		// Skip relay for direct chat / direct file messages NOT addressed to us.
		// In a flat broadcast mesh every neighbour already heard the packet, so
		// relaying just amplifies noise and risks privacy bleed.
		if shouldSkipRelay(p) {
			return
		}
		relayIfNeeded(p)
		return
	}
	if onPacket != nil {
		onPacket(Packet{
			Type:      PktChat,
			Sender:    "Shout",
			Data:      raw,
			Timestamp: time.Now(),
		}, source, rssi)
	}
}

func shouldSkipRelay(p Packet) bool {
	switch p.Type {
	case PktChat, PktAck, PktFileMeta, PktFileChunk, PktFileEnd:
		// Direct messages with a recipient: only relay if not for us
		// (so it can hop further). If for us, no need to re-broadcast.
		if p.Recipient != "" && p.Recipient == SelfName() {
			return true
		}
	}
	return false
}

func handleManufacturer(s, addr string, rssi int16) {
	switch {
	case strings.HasPrefix(s, "SOS:"):
		HandleIncomingSOS(s[4:], addr, rssi)
	case strings.HasPrefix(s, "THREAT:"):
		HandleIncomingThreat(s[7:])
	case strings.HasPrefix(s, "GINV:"):
		HandleIncomingGroupInvite(s[5:], addr)
	}
}

func relayIfNeeded(p Packet) {
	if p.TTL == 0 {
		return
	}
	p.TTL--
	if p.TTL == 0 {
		return
	}
	// Go through BroadcastPacket (not enqueuePacketForBroadcast directly) so
	// the relayed copy is RE-encrypted with our local mesh key (handleIncoming
	// decrypted p.Data in place) and is forwarded on BOTH transports — making
	// this node a true BLE↔Wi-Fi bridge for packets it just heard.
	BroadcastPacket(p)
}

// ─── Outbound (public) ───────────────────────────────────────────────────────
//
// IMPORTANT: BroadcastPacket no longer loopbacks the packet to onPacket. That
// behaviour caused outgoing chat messages to be re-recorded as "incoming from
// yourself" and SOS broadcasts to display the SOS modal on the sender. Each
// caller (Chat.SendDirect / Chat.SendGroup / Files.SendFile / sos.go) is
// responsible for updating its own local UI state before broadcasting.

func BroadcastPacket(p Packet) {
	// Encrypt Data field if a mesh key is configured.
	if enc, ok := tryEncryptData(p.Data); ok {
		p.Data = enc
	}
	packetSeen(p.ID)
	enqueuePacketForBroadcast(p)
	// Wi-Fi fallback transport — see lan.go. No-op when LAN didn't start.
	lanBroadcastPacket(p)
}

func SendToPeer(addr string, p Packet) error {
	BroadcastPacket(p)
	return nil
}

func SendToPeerByName(name string, p Packet) error {
	BroadcastPacket(p)
	return nil
}

// ShoutText broadcasts a short manufacturer-only frame for the legacy short
// shout protocol (SOS:, GINV:). Wrapped in safeGo for panic recovery and
// gated through the same advertiser lock so it doesn't race the broadcast loop
// catastrophically.
func ShoutText(prefix, msg string) {
	full := prefix + msg
	if len(full) > 24 {
		full = full[:24]
	}
	safeGo("shout-"+prefix, func() {
		end := time.Now().Add(3 * time.Second)
		for time.Now().Before(end) {
			startAdvertising(false, []bluetooth.ManufacturerDataElement{
				{CompanyID: 0xFFFF, Data: []byte(full)},
			})
			time.Sleep(300 * time.Millisecond)
		}
		setBeaconAdvertisement()
	})
}

// SendMessage — backwards-compat: broadcast as group message.
func SendMessage(text string) {
	p := newPacket(PktGroup, []byte(text))
	p.Sender = SelfName()
	p.Group = "Group"
	BroadcastPacket(p)
}

// ─── Topology snapshot for UI ────────────────────────────────────────────────

func TopoAddressForName(name string) string {
	nameMu.RLock()
	addr, ok := nameToAddr[name]
	nameMu.RUnlock()
	if ok && addr != "" {
		return addr
	}

	topoMu.RLock()
	defer topoMu.RUnlock()
	for addr, n := range topology {
		if stripBLPrefix(n.Name) == name && n.Connected {
			return addr
		}
	}
	for addr, n := range topology {
		if stripBLPrefix(n.Name) == name {
			return addr
		}
	}
	return ""
}

// topoNode returns a snapshot copy of a Node by address, or nil if missing.
func topoNode(addr string) *Node {
	topoMu.RLock()
	defer topoMu.RUnlock()
	if n, ok := topology[addr]; ok {
		cp := *n
		return &cp
	}
	return nil
}

func TopologySnapshot() []Node {
	topoMu.RLock()
	defer topoMu.RUnlock()
	out := make([]Node, 0, len(topology))
	for _, n := range topology {
		out = append(out, *n)
	}
	return out
}

func ActivePeerCount() int {
	topoMu.RLock()
	defer topoMu.RUnlock()
	now := time.Now()
	c := 0
	for _, n := range topology {
		if now.Sub(n.LastSeen) <= peerActiveWindow {
			c++
		}
	}
	return c
}

func selfAdvName() string {
	if n := SelfName(); n != "" {
		out := "BL-" + n
		if len(out) > 12 {
			out = out[:12]
		}
		return out
	}
	return "BL-Node"
}
