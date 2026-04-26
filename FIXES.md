# BitLink — Fix Pass

Applied as a follow-up to the initial code review. The wire protocol stays
backwards compatible with v5 *except* for one additive change: file chunks
now carry an explicit index (see "File transfer" below). Old peers will be
unable to receive files from new peers, but every other packet type still
interoperates.

## Critical correctness

1. **Self-message duplication** — `BroadcastPacket` no longer loops back to
   `onPacket(... "self" ...)`. Each caller now updates its own local UI
   record before broadcasting. This eliminated:
   - Outgoing chats appearing as "incoming from yourself".
   - SOS modal popping on the sender.
   - Duplicate group messages.

2. **Direct-chat recipient gate** — `ReceiveDirect` is now invoked only
   when `p.Recipient == ""` (legacy) or `p.Recipient == SelfName()`. Direct
   chats addressed to other peers are no longer recorded locally. The same
   gate is applied to `PktFileMeta` / `PktFileChunk` / `PktFileEnd`.

3. **Race-free identity** — `selfName` and `selfColor` are now guarded by
   `selfMu sync.RWMutex` and accessed exclusively through `SelfName()` /
   `SelfColor()` / `setSelf()`. The previous code had concurrent reads
   from the BLE goroutines while the UI thread wrote them.

4. **`currentTabIdx` race** — replaced with `atomic.Int32` plus a
   read-mutex-guarded `currentTabName`.

5. **`pendingInvites` mutex / `safeGo`** — broadcast loop and shouts run
   inside `safeGo`, so panics print a stack trace instead of crashing
   the whole BLE engine.

6. **Notify-after-unlock** — `contactStore.Touch` and `groupStore.AddMember`
   no longer hold the store mutex when calling `save()` + `notify()`,
   removing a deadlock potential when listeners try to read the store.

## Mesh / BLE

7. **Reassembly buffer DoS cap** — `rxBuffers` is capped at `maxRxBuffers`
   (64); when full, the oldest in-flight buffer is evicted.

8. **`io.ReadFull` in `DecodePacket`** — replaces the manual byte-by-byte
   `binary.Read` loop, so partial reads no longer truncate fields silently.
   Also enforces `MaxPacketDataSize = 3500`.

9. **`chunkSeq` seeded from `crypto/rand`** — fresh restarts no longer
   collide with in-flight reassembly buffers on neighbouring peers.

10. **`chunkPacket` rejects oversized packets** — returns an error when a
    Packet would need more than 255 air-chunks instead of silently
    truncating the message.

11. **`relayIfNeeded` skip for direct-to-us packets** — the broadcast mesh
    already delivered the message to every neighbour, so re-broadcasting a
    direct packet that was already addressed to us just amplifies noise.

12. **`updateTopology` uses `strings.HasPrefix`** — same with all the
    `name[:3] == "BL-"` checks scattered across the file. New helper
    `stripBLPrefix` and `topoNode(addr)` for snapshot-style topology reads.

13. **Threat-report identity beacon quiet window** — `SendThreatReport` now
    calls `SetBeaconQuietWindow(5*time.Second)` so the BLE LocalName isn't
    advertised at the same time the binary `PktThreat` packet is on air.
    The quiet window is enforced by the advertiser loop using
    `setSilentBeaconAdvertisement()`.

## Pair-code rework (security)

14. The original `pairCodeFor(name)` derived a 4-digit code from a hash of
    the username — anyone who knew your callsign could compute your code,
    so the "lock" was security theatre. The fix:
    - Generate a random 4-digit code per device on first launch via
      `crypto/rand`, persist as `pair.selfCode`.
    - Embed the code in the BLE beacon's manufacturer data
      (`'B','L', codeHi, codeLo`) so peers learn each other's code without
      a handshake.
    - `IsPeerNameVisible` now matches against `topology[addr].PairCode`.
    - `MyPairCode()` / `MyPairCodeBytes()` / `PairCodeFromBytes()` are the
      new accessors.

## Encryption (new)

15. **Optional shared-key transport encryption** — new `crypto.go`. When a
    32-byte mesh key is configured (Settings → "Mesh Encryption"), every
    `Packet.Data` field is wrapped in **AES-256-GCM** before broadcast and
    unwrapped on receive. Wrapped payloads are tagged with a magic byte
    (`0xCE`) so legacy plaintext peers and encrypted peers can coexist
    during rollout.
    - `SetMeshKeyHex(hex)` / `MeshKey()` / `GenerateMeshKey()`.
    - **Limitation**: Sender / Recipient / Group fields stay plaintext.
      This is transport encryption, not full E2E privacy.

## File transfer (wire-format change)

16. **Chunk index in `PktFileChunk`** — body is now
    `id\x1fIDX\x1fbody` instead of `id\x1fbody`. The receiver
    (`HandleChunk` / `HandleEnd`) now:
    - Stores chunks in a map keyed by index, not a flat slice.
    - Detects out-of-order delivery and drops dups.
    - Verifies completeness against the advertised chunk count from META.
17. **META protocol upgrade** — body is now `id\x1fname\x1fsize\x1ftotal`
    (legacy format had no id-first field; we now require it for symmetry
    with chunks). Receiver tolerates a 3-field meta but expects a 4-field
    one for completeness checking.
18. **`FileSizeLimit` reduced to 64 KB** — 500 KB over advertisement-only
    BLE took ~10+ minutes per file; 64 KB is closer to the realistic
    airtime budget. `FileChunkSize` raised to 1024 to match.
19. **Stalled receiver reaper** — `Files.reapStalled(30s)` is called from
    the mesh reaper to drop in-flight uploads that stop making progress.
    `HandleMeta` also caps the live receiver count at 16.
20. **`sanitizeFilename` hardened**: uses `filepath.Base`, strips
    control chars, replaces Windows-forbidden punctuation, prefixes
    reserved device names (CON / LPT1 / …), and caps length at 200 bytes.

## Code quality / housekeeping

21. **stdlib hex** — removed the in-tree `hexFromBytes` / `hexToBytes` /
    `hexNib` and `splitN` reimplementations; now using `encoding/hex` and
    `strings.SplitN`.
22. Removed the verbose `[BitLink] INCOMING …` and `ReceiveDirect …` debug
    `fmt.Printf` lines.
23. Suppressed unused-variable warnings for palette colors that are
    referenced from other files (`colCyanDim`, `colVioletGlow`, etc.) via
    a single `var _ = []color.NRGBA{…}`.
24. `performHardReset` now also wipes `pair.selfCode`, `pair.lockCode` and
    `crypto.meshKey`, and resets the in-memory `pairLockCode` /
    `myPairCode` / `selfMu` state.

## Known follow-ups (not part of this pass)

- `IsPeerNameVisible` is wired up but the chat / contact tabs don't yet
  call it to filter their lists. That requires touching every UI file and
  is best done as a separate UX pass.
- The `currentTabIdx() != 0` check in main.go assumes index 0 == Home —
  hard-code lookup by name instead once UI lookup helpers are added.
- `ui_*.go` still reach for `fyne.CurrentApp().Driver().AllWindows()[0]`
  in a handful of places; safe today (single-window app) but should be
  threaded through as a parameter for cleanliness.
- File transfers are not yet authenticated — a malicious peer can still
  inject chunks into someone else's reassembly by stealing the file ID.
  Adding an HMAC keyed off the mesh shared key would close that gap.
