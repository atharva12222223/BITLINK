package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
)

// Maximum file size accepted for transfer. Reduced from the original 500 KB
// to keep total airtime within the realistic budget of advertisement-only
// mesh: at ~16 bytes per chunk × 220 ms airtime × 3 repeats, even 64 KB
// already takes several minutes to transmit reliably.
const FileSizeLimit = 64 * 1024

// FileChunkSize keeps each PktFileChunk Data field comfortably below
// MaxPacketDataSize — 1 KiB pieces give 64 chunks for a 64 KB max file.
const FileChunkSize = 1024

// FileEntry — record of a sent or received file.
type FileEntry struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	Peer     string    `json:"peer"`
	Outgoing bool      `json:"out"`
	Time     time.Time `json:"ts"`
}

type fileStore struct {
	mu        sync.RWMutex
	prefs     fyne.Preferences
	items     []FileEntry
	listener  func()
	docDir    string
	receivers map[string]*fileReceiver // id -> partial
}

type fileReceiver struct {
	Name      string
	Size      int64
	From      string
	Total     int               // expected chunk count, 0 if unknown
	Chunks    map[int][]byte    // chunk idx -> bytes
	Started   time.Time
	LastChunk time.Time
}

var Files = &fileStore{receivers: map[string]*fileReceiver{}}

func (s *fileStore) Bind(p fyne.Preferences, docDir string) {
	s.prefs = p
	s.docDir = docDir
	_ = os.MkdirAll(docDir, 0o755)
	s.load()
}

func (s *fileStore) SetListener(f func()) { s.listener = f }
func (s *fileStore) notify() {
	if s.listener != nil {
		s.listener()
	}
}

func (s *fileStore) load() {
	if s.prefs == nil {
		return
	}
	if str := s.prefs.String("files.items"); str != "" {
		_ = json.Unmarshal([]byte(str), &s.items)
	}
}

func (s *fileStore) save() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.prefs == nil {
		return
	}
	if b, err := json.Marshal(s.items); err == nil {
		s.prefs.SetString("files.items", string(b))
	}
}

func (s *fileStore) Items() []FileEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]FileEntry, len(s.items))
	copy(out, s.items)
	return out
}

func (s *fileStore) Add(e FileEntry) {
	s.mu.Lock()
	s.items = append(s.items, e)
	s.mu.Unlock()
	s.save()
	s.notify()
}

// reapStalled drops file receivers that haven't received a chunk for `idle`.
// Called from the mesh reaper to cap memory usage.
func (s *fileStore) reapStalled(idle time.Duration) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, r := range s.receivers {
		ref := r.LastChunk
		if ref.IsZero() {
			ref = r.Started
		}
		if now.Sub(ref) > idle {
			delete(s.receivers, id)
		}
	}
}

// ─── Sending ─────────────────────────────────────────────────────────────────

// SendFile reads a local file and streams it as PktFileMeta + PktFileChunk*N + PktFileEnd.
// If peerAddr == "" the file is broadcast to all connected peers.
// Each chunk now carries an explicit index so out-of-order delivery and dropped
// chunks can be detected and rejected on the receiving side.
func (s *fileStore) SendFile(peerAddr, path string, onProgress func(float64)) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() > FileSizeLimit {
		return fmt.Errorf("file exceeds %d byte limit", FileSizeLimit)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	idRaw := make([]byte, 8)
	if _, err := rand.Read(idRaw); err != nil {
		return err
	}
	idHex := hex.EncodeToString(idRaw)
	name := filepath.Base(path)

	total := len(data)
	totalChunks := (total + FileChunkSize - 1) / FileChunkSize
	if totalChunks == 0 {
		totalChunks = 1
	}

	self := SelfName()

	// META: id\x1fname\x1fsize\x1ftotalChunks
	metaBody := fmt.Sprintf("%s\x1f%s\x1f%d\x1f%d", idHex, name, len(data), totalChunks)
	meta := newPacket(PktFileMeta, []byte(metaBody))
	meta.Sender = self
	meta.Recipient = peerAddr
	copy(meta.ID[:], idRaw)
	if peerAddr == "" {
		BroadcastPacket(meta)
	} else {
		_ = SendToPeer(peerAddr, meta)
	}

	// CHUNKS: id\x1fIDX\x1fbody (binary-safe — IDX is decimal ASCII)
	sent := 0
	for i := 0; i < totalChunks; i++ {
		off := i * FileChunkSize
		end := off + FileChunkSize
		if end > total {
			end = total
		}
		hdr := []byte(idHex + "\x1f" + strconv.Itoa(i) + "\x1f")
		body := append(hdr, data[off:end]...)
		chunk := newPacket(PktFileChunk, body)
		chunk.Sender = self
		chunk.Recipient = peerAddr
		if peerAddr == "" {
			BroadcastPacket(chunk)
		} else {
			_ = SendToPeer(peerAddr, chunk)
		}
		sent = end
		if onProgress != nil {
			onProgress(float64(sent) / float64(total))
		}
		// Give the BLE stack room. Larger sleep with bigger chunks.
		time.Sleep(40 * time.Millisecond)
	}

	// END: id only
	endP := newPacket(PktFileEnd, []byte(idHex))
	endP.Sender = self
	endP.Recipient = peerAddr
	if peerAddr == "" {
		BroadcastPacket(endP)
	} else {
		_ = SendToPeer(peerAddr, endP)
	}

	s.Add(FileEntry{
		ID: idHex, Name: name, Path: path, Size: info.Size(),
		Peer: peerAddr, Outgoing: true, Time: time.Now(),
	})
	return nil
}

// ─── Receiving ───────────────────────────────────────────────────────────────

func (s *fileStore) HandleMeta(p Packet) {
	parts := strings.SplitN(string(p.Data), "\x1f", 4)
	if len(parts) < 3 {
		return
	}
	id := parts[0]
	name := sanitizeFilename(parts[1])
	size, _ := strconv.ParseInt(parts[2], 10, 64)
	if size < 0 || size > FileSizeLimit {
		return
	}
	totalChunks := 0
	if len(parts) >= 4 {
		totalChunks, _ = strconv.Atoi(parts[3])
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Cap concurrent in-flight receivers.
	if len(s.receivers) >= 16 {
		var oldestID string
		var oldestT time.Time
		first := true
		for k, v := range s.receivers {
			ref := v.LastChunk
			if ref.IsZero() {
				ref = v.Started
			}
			if first || ref.Before(oldestT) {
				oldestID = k
				oldestT = ref
				first = false
			}
		}
		delete(s.receivers, oldestID)
	}
	s.receivers[id] = &fileReceiver{
		Name:    name,
		Size:    size,
		From:    p.Sender,
		Total:   totalChunks,
		Chunks:  map[int][]byte{},
		Started: time.Now(),
	}
}

func (s *fileStore) HandleChunk(p Packet) {
	parts := strings.SplitN(string(p.Data), "\x1f", 3)
	if len(parts) < 3 {
		return
	}
	id := parts[0]
	idx, err := strconv.Atoi(parts[1])
	if err != nil || idx < 0 {
		return
	}
	body := []byte(parts[2])
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.receivers[id]
	if !ok {
		return
	}
	if r.Total > 0 && idx >= r.Total {
		return
	}
	if _, dup := r.Chunks[idx]; dup {
		return
	}
	r.Chunks[idx] = body
	r.LastChunk = time.Now()
}

func (s *fileStore) HandleEnd(p Packet) (FileEntry, bool) {
	id := string(p.Data)
	s.mu.Lock()
	r, ok := s.receivers[id]
	delete(s.receivers, id)
	s.mu.Unlock()
	if !ok {
		return FileEntry{}, false
	}
	if s.docDir == "" {
		return FileEntry{}, false
	}
	// Verify completeness when total was advertised.
	if r.Total > 0 && len(r.Chunks) != r.Total {
		fmt.Printf("[BitLink] file %s: incomplete (%d/%d chunks)\n", id, len(r.Chunks), r.Total)
		return FileEntry{}, false
	}
	// Reassemble chunks in index order.
	maxIdx := -1
	for k := range r.Chunks {
		if k > maxIdx {
			maxIdx = k
		}
	}
	full := make([]byte, 0, r.Size)
	for i := 0; i <= maxIdx; i++ {
		piece, ok := r.Chunks[i]
		if !ok {
			fmt.Printf("[BitLink] file %s: missing chunk %d\n", id, i)
			return FileEntry{}, false
		}
		full = append(full, piece...)
	}
	if r.Size > 0 && int64(len(full)) > r.Size+int64(FileChunkSize) {
		// Sanity check — refuse runaway sizes.
		return FileEntry{}, false
	}
	if len(id) < 6 {
		return FileEntry{}, false
	}
	out := filepath.Join(s.docDir, fmt.Sprintf("%s_%s", id[:6], r.Name))
	if err := os.WriteFile(out, full, 0o600); err != nil {
		fmt.Println("[BitLink] save file:", err)
		return FileEntry{}, false
	}
	e := FileEntry{
		ID: id, Name: r.Name, Path: out, Size: int64(len(full)),
		Peer: r.From, Outgoing: false, Time: time.Now(),
	}
	s.Add(e)
	return e, true
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// sanitizeFilename strips path traversal, control chars, reserved Windows
// names, and caps the resulting length so inbound files can never escape the
// document directory or trip the OS.
func sanitizeFilename(s string) string {
	// Take only the basename — defeats "../" payloads.
	s = filepath.Base(s)
	if s == "." || s == ".." || s == "/" || s == "\\" {
		return "file.bin"
	}
	// Strip control chars and forbidden ASCII punctuation.
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == 0x7F {
			continue
		}
		switch c {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			c = '_'
		}
		out = append(out, c)
	}
	clean := strings.TrimSpace(string(out))
	clean = strings.TrimLeft(clean, ".")
	if clean == "" {
		return "file.bin"
	}
	// Reserved Windows device names (case-insensitive, with or without ext).
	upper := strings.ToUpper(clean)
	base := upper
	if dot := strings.Index(upper, "."); dot >= 0 {
		base = upper[:dot]
	}
	switch base {
	case "CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
		clean = "_" + clean
	}
	// Cap length (most filesystems allow 255 bytes).
	const maxNameLen = 200
	if len(clean) > maxNameLen {
		ext := filepath.Ext(clean)
		if len(ext) > 16 {
			ext = ""
		}
		clean = clean[:maxNameLen-len(ext)] + ext
	}
	return clean
}
