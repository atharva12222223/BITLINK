package main

import (
        "encoding/json"
        "sync"
        "time"

        "fyne.io/fyne/v2"
)

// Contact is a saved peer with a custom nickname.
type Contact struct {
        Address  string    `json:"addr"`
        Nickname string    `json:"nick"`
        LastSeen time.Time `json:"last"`
        RSSI     int16     `json:"rssi"`
}

type contactStore struct {
        mu       sync.RWMutex
        prefs    fyne.Preferences
        items    map[string]*Contact // keyed by address
        listener func()
}

var Contacts = &contactStore{items: map[string]*Contact{}}

func (s *contactStore) Bind(p fyne.Preferences) {
        s.prefs = p
        s.load()
}

func (s *contactStore) SetListener(f func()) { s.listener = f }
func (s *contactStore) notify() {
        if s.listener != nil {
                s.listener()
        }
}

func (s *contactStore) load() {
        if s.prefs == nil {
                return
        }
        if str := s.prefs.String("contacts.items"); str != "" {
                _ = json.Unmarshal([]byte(str), &s.items)
        }
}

func (s *contactStore) save() {
        s.mu.RLock()
        defer s.mu.RUnlock()
        if s.prefs == nil {
                return
        }
        if b, err := json.Marshal(s.items); err == nil {
                s.prefs.SetString("contacts.items", string(b))
        }
}

// Save (or update) a contact with nickname.
func (s *contactStore) Save(addr, nickname string, rssi int16) {
        s.mu.Lock()
        c, ok := s.items[addr]
        if !ok {
                c = &Contact{Address: addr}
                s.items[addr] = c
        }
        c.Nickname = nickname
        c.LastSeen = time.Now()
        c.RSSI = rssi
        s.mu.Unlock()
        s.save()
        s.notify()
}

// Touch updates last-seen and RSSI without changing nickname.
func (s *contactStore) Touch(addr string, rssi int16) {
        s.mu.Lock()
        c, ok := s.items[addr]
        if ok {
                c.LastSeen = time.Now()
                c.RSSI = rssi
        }
        s.mu.Unlock()
        if ok {
                s.save()
                s.notify()
        }
}

func (s *contactStore) Delete(addr string) {
        s.mu.Lock()
        delete(s.items, addr)
        s.mu.Unlock()
        s.save()
        s.notify()
}

func (s *contactStore) Get(addr string) (Contact, bool) {
        s.mu.RLock()
        defer s.mu.RUnlock()
        c, ok := s.items[addr]
        if !ok {
                return Contact{}, false
        }
        return *c, true
}

func (s *contactStore) NicknameFor(addr string) string {
        c, ok := s.Get(addr)
        if !ok {
                return ""
        }
        return c.Nickname
}

func (s *contactStore) All() []Contact {
        s.mu.RLock()
        defer s.mu.RUnlock()
        out := make([]Contact, 0, len(s.items))
        for _, c := range s.items {
                out = append(out, *c)
        }
        return out
}
