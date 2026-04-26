package main

import (
        "encoding/json"
        "sync"
        "time"

        "fyne.io/fyne/v2"
)

// Group represents a named group channel local to this device.
type Group struct {
        Name    string    `json:"name"`
        Members []string  `json:"members"` // peer addresses (informational)
        Created time.Time `json:"created"`
}

type groupStore struct {
        mu       sync.RWMutex
        prefs    fyne.Preferences
        items    map[string]*Group
        listener func()
}

var Groups = &groupStore{items: map[string]*Group{}}

func (s *groupStore) Bind(p fyne.Preferences) {
        s.prefs = p
        s.load()
        // Always provide a default broadcast group
        s.mu.Lock()
        if _, ok := s.items[GroupBroadcast]; !ok {
                s.items[GroupBroadcast] = &Group{Name: GroupBroadcast, Created: time.Now()}
        }
        s.mu.Unlock()
        s.save()
}

func (s *groupStore) SetListener(f func()) { s.listener = f }
func (s *groupStore) notify() {
        if s.listener != nil {
                s.listener()
        }
}

func (s *groupStore) load() {
        if s.prefs == nil {
                return
        }
        if str := s.prefs.String("groups.items"); str != "" {
                _ = json.Unmarshal([]byte(str), &s.items)
        }
}

func (s *groupStore) save() {
        s.mu.RLock()
        defer s.mu.RUnlock()
        if s.prefs == nil {
                return
        }
        if b, err := json.Marshal(s.items); err == nil {
                s.prefs.SetString("groups.items", string(b))
        }
}

func (s *groupStore) Create(name string) *Group {
        if name == "" {
                return nil
        }
        s.mu.Lock()
        g, ok := s.items[name]
        if !ok {
                g = &Group{Name: name, Created: time.Now()}
                s.items[name] = g
        }
        s.mu.Unlock()
        s.save()
        s.notify()
        return g
}

func (s *groupStore) Join(name string) {
        s.Create(name)
}

func (s *groupStore) Leave(name string) {
        if name == GroupBroadcast {
                return
        }
        s.mu.Lock()
        delete(s.items, name)
        s.mu.Unlock()
        s.save()
        s.notify()
}

func (s *groupStore) AddMember(group, addr string) {
        s.mu.Lock()
        g, ok := s.items[group]
        if !ok {
                s.mu.Unlock()
                return
        }
        for _, m := range g.Members {
                if m == addr {
                        s.mu.Unlock()
                        return
                }
        }
        g.Members = append(g.Members, addr)
        s.mu.Unlock()
        s.save()
        s.notify()
}

func (s *groupStore) Has(name string) bool {
        s.mu.RLock()
        defer s.mu.RUnlock()
        _, ok := s.items[name]
        return ok
}

func (s *groupStore) All() []Group {
        s.mu.RLock()
        defer s.mu.RUnlock()
        out := make([]Group, 0, len(s.items))
        for _, g := range s.items {
                out = append(out, *g)
        }
        return out
}

// ─── Group invite over BLE manufacturer data ─────────────────────────────────

func InviteToGroup(name string) {
        ShoutText("GINV:", name)
}

func HandleIncomingGroupInvite(name, fromAddr string) {
        if name == "" {
                return
        }
        if Groups.Has(name) {
                return
        }
        // Enqueue invite for the UI to surface
        pendingInvitesMu.Lock()
        pendingInvites = append(pendingInvites, GroupInvite{Group: name, From: fromAddr, At: time.Now()})
        pendingInvitesMu.Unlock()
        if onGroupInvite != nil {
                onGroupInvite(name, fromAddr)
        }
}

type GroupInvite struct {
        Group string
        From  string
        At    time.Time
}

var (
        pendingInvitesMu sync.Mutex
        pendingInvites   []GroupInvite
        onGroupInvite    func(group, from string)
)

func PendingInvites() []GroupInvite {
        pendingInvitesMu.Lock()
        defer pendingInvitesMu.Unlock()
        return append([]GroupInvite(nil), pendingInvites...)
}

func ConsumeInvite(group string) {
        pendingInvitesMu.Lock()
        defer pendingInvitesMu.Unlock()
        out := pendingInvites[:0]
        for _, p := range pendingInvites {
                if p.Group != group {
                        out = append(out, p)
                }
        }
        pendingInvites = out
}
