package main

// lan.go — Wi-Fi fallback transport.
//
// Runs alongside the BLE mesh. Emits a small UDP beacon every few seconds
// over every connected interface's broadcast address, and re-broadcasts
// every packet that goes through BroadcastPacket. Incoming UDP frames feed
// the same onDiscover / onPacket / updateTopology callbacks the BLE path
// uses, so the rest of the app sees one unified peer set regardless of
// which radio the peer was reached on.
//
// This exists because Windows desktops cannot broadcast BLE advertisements
// through the Go bluetooth library (driver-level limitation, not a Go bug).
// On a Windows ↔ Windows pair the BLE side is silent in both directions;
// the LAN transport is what actually carries discovery and chat between
// them. On macOS / Linux / Android peers the BLE path still works and the
// LAN path simply duplicates messages — the seenIDs ring drops the dupes.

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	lanPort         = 47474
	lanBeaconPeriod = 2500 * time.Millisecond
	lanFrameMagic0  = 'B'
	lanFrameMagic1  = 'L'
	lanFrameVer     = byte(1)
	lanFrameBeacon  = byte(0x01)
	lanFrameData    = byte(0x02)
)

var (
	lanMu       sync.Mutex
	lanConn     *net.UDPConn
	lanLocalIPs = map[string]bool{}
)

// startLAN brings up the Wi-Fi fallback. Safe to call once at startup; if
// the port is already in use (another BitLink instance on this machine)
// it logs and returns — the BLE path keeps running on its own.
func startLAN() {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: lanPort})
	if err != nil {
		fmt.Println("[BitLink] Wi-Fi fallback DISABLED:", err)
		fmt.Println("[BitLink]   (UDP port", lanPort, "may be in use by another instance.)")
		return
	}
	lanMu.Lock()
	lanConn = c
	refreshLocalIPsLocked()
	lanMu.Unlock()

	fmt.Println("[BitLink] Wi-Fi fallback active on UDP port", lanPort)
	fmt.Println("[BitLink]   (Both PCs must be on the same Wi-Fi network or hotspot.)")

	safeGo("lan-rx", lanReceiveLoop)
	safeGo("lan-beacon", lanBeaconLoop)
	safeGo("lan-iface-watcher", lanInterfaceWatcher)
}

// refreshLocalIPsLocked rebuilds the set of IPv4 addresses bound to this
// machine, used to drop our own broadcasts that loop back on Linux/macOS.
func refreshLocalIPsLocked() {
	ips := map[string]bool{}
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				ipnet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				if v4 := ipnet.IP.To4(); v4 != nil {
					ips[v4.String()] = true
				}
			}
		}
	}
	lanLocalIPs = ips
}

// lanInterfaceWatcher periodically refreshes our local-IP set so that
// joining or leaving a Wi-Fi network is picked up without restarting.
func lanInterfaceWatcher() {
	for {
		time.Sleep(15 * time.Second)
		lanMu.Lock()
		refreshLocalIPsLocked()
		lanMu.Unlock()
	}
}

// interfaceBroadcasts returns the per-subnet broadcast address for every
// up, broadcast-capable, non-loopback IPv4 interface. We send to these
// directly instead of 255.255.255.255 to avoid needing SO_BROADCAST set
// on the socket, which keeps the code portable across Windows/macOS/Linux.
func interfaceBroadcasts() []*net.UDPAddr {
	var out []*net.UDPAddr
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 ||
			iface.Flags&net.FlagLoopback != 0 ||
			iface.Flags&net.FlagBroadcast == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			v4 := ipnet.IP.To4()
			if v4 == nil || len(ipnet.Mask) != 4 {
				continue
			}
			bcast := net.IPv4(
				v4[0]|^ipnet.Mask[0],
				v4[1]|^ipnet.Mask[1],
				v4[2]|^ipnet.Mask[2],
				v4[3]|^ipnet.Mask[3],
			)
			out = append(out, &net.UDPAddr{IP: bcast, Port: lanPort})
		}
	}
	// Always include the limited broadcast as a safety net (works on most
	// hotspot setups even when interface enumeration is incomplete).
	out = append(out, &net.UDPAddr{IP: net.IPv4(255, 255, 255, 255), Port: lanPort})
	return out
}

func lanFrame(kind byte, payload []byte) []byte {
	out := make([]byte, 4+len(payload))
	out[0] = lanFrameMagic0
	out[1] = lanFrameMagic1
	out[2] = lanFrameVer
	out[3] = kind
	copy(out[4:], payload)
	return out
}

func lanSendBroadcast(frame []byte) {
	lanMu.Lock()
	c := lanConn
	lanMu.Unlock()
	if c == nil {
		return
	}
	for _, dst := range interfaceBroadcasts() {
		_, _ = c.WriteToUDP(frame, dst)
	}
}

// lanBroadcastPacket is invoked from BroadcastPacket so every chat / file /
// group / sos packet goes out over Wi-Fi as well as Bluetooth.
func lanBroadcastPacket(p Packet) {
	lanSendBroadcast(lanFrame(lanFrameData, p.Encode()))
}

func lanBeaconLoop() {
	for {
		pc := MyPairCodeBytes()
		name := selfAdvName()
		payload := make([]byte, 0, 2+len(name))
		payload = append(payload, pc[0], pc[1])
		payload = append(payload, []byte(name)...)
		lanSendBroadcast(lanFrame(lanFrameBeacon, payload))
		time.Sleep(lanBeaconPeriod)
	}
}

func lanReceiveLoop() {
	buf := make([]byte, 65535)
	for {
		lanMu.Lock()
		c := lanConn
		lanMu.Unlock()
		if c == nil {
			return
		}
		n, src, err := c.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("[BitLink] LAN read error:", err)
			return
		}
		if n < 4 ||
			buf[0] != lanFrameMagic0 ||
			buf[1] != lanFrameMagic1 ||
			buf[2] != lanFrameVer {
			continue
		}

		// Drop frames sent by ourselves bouncing back through the OS.
		lanMu.Lock()
		isLocal := lanLocalIPs[src.IP.String()]
		lanMu.Unlock()
		if isLocal {
			continue
		}

		kind := buf[3]
		body := append([]byte(nil), buf[4:n]...)
		fakeAddr := "lan:" + src.IP.String()

		switch kind {
		case lanFrameBeacon:
			if len(body) < 2 {
				continue
			}
			pairCode := PairCodeFromBytes(body[0:2])
			name := "BL-Node"
			if len(body) > 2 {
				name = string(body[2:])
			}
			if !strings.HasPrefix(name, "BL-") {
				name = "BL-" + name
			}
			updateTopology(fakeAddr, name, -40, pairCode)
			if onDiscover != nil {
				onDiscover(fakeAddr, -40, name)
			}
		case lanFrameData:
			pkt, err := DecodePacket(body)
			if err != nil {
				continue
			}
			// Mirror the BLE receive pipeline so LAN packets get the
			// same treatment: decrypt, dedup, drop our own echoes,
			// then relay onward (which re-broadcasts on BOTH BLE
			// and LAN so this node bridges the two transports).
			if dec, ok := tryDecryptData(pkt.Data); ok {
				pkt.Data = dec
			}
			if packetSeen(pkt.ID) {
				continue
			}
			if pkt.Sender != "" && pkt.Sender == SelfName() {
				continue
			}
			if onPacket != nil {
				onPacket(pkt, fakeAddr, -40)
			}
			if !shouldSkipRelay(pkt) {
				relayIfNeeded(pkt)
			}
		}
	}
}
