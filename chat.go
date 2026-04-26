package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// Message represents one chat record stored locally.
type Message struct {
	ID        string    `json:"id"`
	Peer      string    `json:"peer"`   // address ("" = group/broadcast)
	Group     string    `json:"group"`  // group name ("" = direct)
	Sender    string    `json:"sender"` // display name
	Text      string    `json:"text"`
	Timestamp time.Time `json:"ts"`
	Outgoing  bool      `json:"out"`
	Status    string    `json:"status"` // sent / delivered / received
}

const (
	StatusSent      = "sent"
	StatusDelivered = "delivered"
	StatusReceived  = "received"

	GroupBroadcast = "Broadcast"
)

// In-memory chat store, persisted to Fyne preferences.
type chatStore struct {
	mu       sync.RWMutex
	prefs    fyne.Preferences
	byPeer   map[string][]Message // peer addr -> msgs
	byGroup  map[string][]Message // group name -> msgs
	unread   map[string]int       // key (peer:addr | group:name) -> count
	listener func()               // refresh hook
}

var Chat = &chatStore{
	byPeer:  map[string][]Message{},
	byGroup: map[string][]Message{},
	unread:  map[string]int{},
}

func (c *chatStore) Bind(p fyne.Preferences) {
	c.prefs = p
	c.load()
}

func (c *chatStore) SetListener(f func()) { c.listener = f }

func (c *chatStore) notify() {
	if c.listener != nil {
		c.listener()
	}
}

func (c *chatStore) load() {
	if c.prefs == nil {
		return
	}
	if s := c.prefs.String("chat.byPeer"); s != "" {
		_ = json.Unmarshal([]byte(s), &c.byPeer)
	}
	if s := c.prefs.String("chat.byGroup"); s != "" {
		_ = json.Unmarshal([]byte(s), &c.byGroup)
	}
	if s := c.prefs.String("chat.unread"); s != "" {
		_ = json.Unmarshal([]byte(s), &c.unread)
	}
}

func (c *chatStore) save() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.prefs == nil {
		return
	}
	if b, err := json.Marshal(c.byPeer); err == nil {
		c.prefs.SetString("chat.byPeer", string(b))
	}
	if b, err := json.Marshal(c.byGroup); err == nil {
		c.prefs.SetString("chat.byGroup", string(b))
	}
	if b, err := json.Marshal(c.unread); err == nil {
		c.prefs.SetString("chat.unread", string(b))
	}
}

// ─── Sending ─────────────────────────────────────────────────────────────────

func (c *chatStore) SendDirect(peerName, text string) Message {
	id := randomHex(8)
	self := SelfName()
	msg := Message{
		ID: id, Peer: peerName, Sender: self, Text: text,
		Timestamp: time.Now(), Outgoing: true, Status: StatusSent,
	}
	c.mu.Lock()
	c.byPeer[peerName] = append(c.byPeer[peerName], msg)
	c.mu.Unlock()
	c.save()
	c.notify()

	p := newPacket(PktChat, []byte(text))
	p.Sender = self
	p.Recipient = peerName
	if raw, err := hex.DecodeString(id); err == nil {
		copy(p.ID[:], raw)
	}
	_ = SendToPeerByName(peerName, p)
	return msg
}

func (c *chatStore) SendGroup(group, text string) Message {
	if group == "" {
		group = GroupBroadcast
	}
	id := randomHex(8)
	self := SelfName()
	msg := Message{
		ID: id, Group: group, Sender: self, Text: text,
		Timestamp: time.Now(), Outgoing: true, Status: StatusSent,
	}
	c.mu.Lock()
	c.byGroup[group] = append(c.byGroup[group], msg)
	c.mu.Unlock()
	c.save()
	c.notify()

	p := newPacket(PktGroup, []byte(text))
	p.Sender = self
	p.Group = group
	if raw, err := hex.DecodeString(id); err == nil {
		copy(p.ID[:], raw)
	}
	BroadcastPacket(p)
	return msg
}

// ─── Receiving ───────────────────────────────────────────────────────────────

func (c *chatStore) ReceiveDirect(source string, p Packet) {
	id := hex.EncodeToString(p.ID[:])
	peerName := p.Sender
	if peerName == "" {
		peerName = peerDisplayName(source)
	}
	msg := Message{
		ID: id, Peer: peerName, Sender: p.Sender, Text: string(p.Data),
		Timestamp: p.Timestamp, Outgoing: false, Status: StatusReceived,
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	c.mu.Lock()
	c.byPeer[peerName] = append(c.byPeer[peerName], msg)
	c.unread["peer:"+peerName]++
	c.mu.Unlock()
	c.save()
	c.notify()

	// Send ACK back
	ack := newPacket(PktAck, []byte(id))
	ack.Sender = SelfName()
	ack.Recipient = peerName
	_ = SendToPeerByName(peerName, ack)
}

func (c *chatStore) ReceiveGroup(p Packet) {
	id := hex.EncodeToString(p.ID[:])
	group := p.Group
	if group == "" {
		group = GroupBroadcast
	}
	// Drop our own group echoes (extra defence — mesh layer also filters).
	if p.Sender != "" && p.Sender == SelfName() {
		return
	}
	msg := Message{
		ID: id, Group: group, Sender: p.Sender, Text: string(p.Data),
		Timestamp: p.Timestamp, Outgoing: false, Status: StatusReceived,
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	c.mu.Lock()
	c.byGroup[group] = append(c.byGroup[group], msg)
	c.unread["group:"+group]++
	c.mu.Unlock()
	c.save()
	c.notify()
}

func (c *chatStore) HandleAck(p Packet) {
	ackedID := string(p.Data)
	c.mu.Lock()
	for peer, msgs := range c.byPeer {
		for i := range msgs {
			if msgs[i].ID == ackedID && msgs[i].Outgoing {
				c.byPeer[peer][i].Status = StatusDelivered
			}
		}
	}
	c.mu.Unlock()
	c.save()
	c.notify()
}

// ─── Accessors ───────────────────────────────────────────────────────────────

func (c *chatStore) DirectMessages(peer string) []Message {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Message(nil), c.byPeer[peer]...)
}

func (c *chatStore) GroupMessages(group string) []Message {
	if group == "" {
		group = GroupBroadcast
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]Message(nil), c.byGroup[group]...)
}

func (c *chatStore) Unread(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.unread[key]
}

func (c *chatStore) ClearUnread(key string) {
	c.mu.Lock()
	delete(c.unread, key)
	c.mu.Unlock()
	c.save()
	c.notify()
}

func (c *chatStore) UnreadTotal() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t := 0
	for _, v := range c.unread {
		t += v
	}
	return t
}

func (c *chatStore) ClearDirect(peer string) {
	c.mu.Lock()
	delete(c.byPeer, peer)
	delete(c.unread, "peer:"+peer)
	c.mu.Unlock()
	c.save()
	c.notify()
}

func (c *chatStore) ClearGroup(group string) {
	c.mu.Lock()
	delete(c.byGroup, group)
	delete(c.unread, "group:"+group)
	c.mu.Unlock()
	c.save()
	c.notify()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// defaultMeshTTL is the maximum number of hops a packet can take through the
// mesh before it's dropped. Each intermediate peer that hears the packet
// re-broadcasts it (decrementing TTL) so two distant nodes can talk through
// any chain of nearby nodes. 7 is the same value used by Meshtastic and is
// the practical sweet spot — high enough that ~6 relay hops covers a school
// hall / disaster zone scenario, low enough that the seenIDs dedup ring
// (200 entries) easily prevents broadcast storms.
const defaultMeshTTL = 7

func newPacket(typ byte, data []byte) Packet {
	p := Packet{
		Type:      typ,
		TTL:       defaultMeshTTL,
		Timestamp: time.Now(),
		Data:      data,
	}
	_, _ = rand.Read(p.ID[:])
	return p
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
