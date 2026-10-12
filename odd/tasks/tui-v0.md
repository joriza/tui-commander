# tui-commander — TUI v0

## Goal

A behavior-first, Midnight Commander-inspired dual-panel terminal file manager
for Windows (Linux as fast follow). Single static binary, zero install:
copy-and-run, portable from a USB stick.

## Constraints

- Language: Go 1.26 (installed locally). UI: tview/tcell (pure Go, no cgo, no
  ncurses).
- Behavior over aesthetics: default tview styling is acceptable for v0.
- Clean-room: replicate behavior patterns, never assets or code from Total
  Commander or Midnight Commander.
- Sibling-project model: a future GUI sibling implements this same spec;
  keep behaviors expressed as testable assertions here.
- Windows-first quirks budget: long paths (`\\?\` prefix), legacy console
  codepages, readonly/hidden attributes, console resize events.

## Behavior spec v0 (testable assertions)

- S1: Two independent panels; each keeps its own directory, cursor, filter and
  sort state when focus switches.
- S2: Navigation: Enter on directory descends; `..` ascends; drive/volume
  switching on Windows; directory reread on external change (F-reread is
  acceptable for v0).
- S3: Copy (F5) and move (F6) operate on the selection set, run through a
  sequential queue, report per-item and total progress, and can be cancelled;
  errors are collected and reported without aborting the queue.
- S4: Delete (F8) requires confirmation and deletes permanently; readonly and
  hidden files are handled explicitly.
- S5: Rename (Shift+F6) and make directory (F7) validate names against the
  platform rules (reserved names, trailing dots/spaces on Windows).
- S6: Incremental filter narrows the visible list as the user types; clearing
  restores the full listing.
- S7: View (F3) pages text files in place; Edit (F4) opens `$EDITOR`/`%EDITOR%`
  and rereads the panel on return.
- S8: All core operations are pure logic, independent of the UI, covered by
  unit tests with a real temporary filesystem.

## Tasks

- [x] T1: Scaffold — Go module `tui-commander`, main entry with tview
      dual-panel placeholder, `.gitignore`, README with build/run/portable
      notes; `go build`, `go vet`, `go test ./...` green.
- [x] T2: Core path platform adapter — Windows long-path handling, name
      validation (reserved names, trailing dots/spaces), cross-platform join
      and display formatting; unit tests (test-first).
- [x] T3: Core copy engine — selection set → sequential queue → per-item +
      total progress → cancellation; error collection per item; unit tests
      with temp filesystem (test-first).
- [x] T4: Core move/delete/rename/mkdir — move reuses copy engine (rename
      fast path when same volume), delete with explicit attribute handling;
      unit tests (test-first).
- [x] T5: Dual panel UI — real listings, per-panel state, cursor, sorting,
      focus switching (S1, S2).
- [x] T6: F-key wiring — TC-standard bindings: F5 copy, F6 shared
      move-or-rename dialog, Shift+F6 rename in place, Shift+F5 duplicate,
      F7 mkdir, F8 and Delete delete, all with confirmation dialogs and
      progress view (S3–S5); Esc closes topmost dialog only.
      Delivered as T6a (47ee481 + fixes 0123a9d/c2add46) and T6b (ae22ba4:
      copy/duplicate/move/rename with byte progress and overwrite
      confirmation; silent-overwrite move risk caught and fixed in review).
- [ ] T7: Incremental filter (Ctrl+Alt+letter, TC standard) + drive
switching with Alt+F1/Alt+F2 (S6, S2); selection keys Insert,
Space, +, -, * on main-row keys.
- [x] T6c (commit `9099946`): ItemStarted.Size in core; OperationRunner
      byte tracking; progress modal with two text bars (file% + total%) +
      ETA/speed after 2% (EMA, TC-style); ShowFormDialog extraRows (delete
      confirm +2); F2 = reread focused panel (TC standard per installed
      KEYBOARD.TXT); MoveQueue QueueTotals propagation verified.
- [x] T6d (commit `9980525`): progress dialog shows the CURRENT FILE NAME and
      an incrementing FILE COUNT, including for directory-tree copies.
      Root cause (confirmed empirically 2026-10-12): CopyQueue/MoveQueue are
      item-granular — a directory is ONE queue item, so only the folder name
      was ever reported and the count stuck at 0/1; also OperationRunner
      double-counted totals (`total++` on every ItemStarted after QueueTotals
      already set TotalItems → "1/2" for a one-item copy). Fix: core emits a
      new `ItemFile` event per individual file (name + size) during tree
      copies; `QueueTotals` gains `TotalFiles`; runner drives the count from
      file-level events; UI keeps the simplified "current file name + N/M
      files" display the user asked for (bytes/ETA bars deferred).
      RDD review: preflight ran; candidate is high-risk and wide (35 files /
      5137 lines from the un-advanced branch-point boundary). Consent could
      NOT be presented — this runtime exposes no native `question` consent UI,
      so the review start is blocked pending a compatible runtime.
- [ ] T8: View/Edit + polish — F3 pager, F4 editor handoff, README final,
      optimized static build (`-ldflags="-s -w"`), smoke test on a real
      folder tree, portable-run checklist.

## Evidence

| Task | Commit | Checks observed |
| --- | --- | --- |
| T1 | `f290617` (branch `tui-v0`) | `go build ./...`, `go vet ./...`, `go test ./...` all green; observed directly by parent session (build, vet, test rerun). Visual smoke confirmed by user: dual panels render with bottom key bar |
| T2 | `36cdd2e` (branch `tui-v0`) | RED observed (undefined API + 2 failing + 1 test-setup bug caught) → GREEN: 23 tests on windows/amd64 (`-count=1` rerun), `GOOS=linux` cross-compile build+test, `go vet` clean; all observed directly by parent session |
| T3 | `99d3d9a` (branch `tui-v0`) | 40 tests green (`-count=1`), vet clean, `GOOS=linux` build+test-compile; race detector skipped — CGO unavailable on this machine (confirmed by parent). Caveat: worker sessions ended twice without final reports; RED observation was not independently captured; parent finished the last step inline (removed debug scratch tests, folded a clean pre-cancelled-context test into copy_test.go) and verified everything directly |
| T4 | `babf371` (branch `tui-v0`) | RED observed (RemoveAll nil for nonexistent sources → Stat check added) → GREEN: 66 tests (`-count=1`), vet clean, `GOOS=linux` build+test compile; all observed directly by parent session |
| T5 | `9a90bf1` (branch `tui-v0`) | Panel RED→GREEN per worker (RED tail not independently captured — worker report truncated, third occurrence); parent verified directly: 81 tests green (`-count=1`), vet clean, linux build+test compile, keyboard bindings reviewed line-by-line (Tab/Enter/Ctrl+PgUp/arrows/Home/End/F10/Esc). Visual smoke: pending user partial test |
| T6a | `47ee481` (branch `tui-v0`) | SameDirectory RED→GREEN (4 tests); parent verified directly: 85 tests green (`-count=1`), vet clean, linux build+test compile; parent improved ItemError handling to keep per-file paths in error modal; UI visual smoke: pending user test of F7/F8 |
| T6a fix | `0123a9d` (branch `tui-v0`) | User-reported deadlock: F7/F8 froze keyboard with no visible dialog, Esc unrecoverable. Root cause: QueueUpdateDraw called from UI-goroutine dialog paths (page adds, button callbacks, closeDialog). Fixed by making dialog helpers mutate pages directly; only updateProgress (operation goroutine) keeps QueueUpdateDraw. Verified: build, vet, 85 tests green; user retest pending |
| T6a fix 2 | `c2add46` (branch `tui-v0`) | User-reported F8 hang: progress modal stuck while delete completed. Root cause: NESTED QueueUpdateDraw — runner marshals OnProgress onto the UI thread and updateProgress queued again (unbuffered updates channel self-deadlock). Fixed: updateProgress mutates directly (runner is the single marshalling point). Also: dialogs now have borders+titles; Esc never quits (F10 only) confirmed as policy. Diagnosed with headless simulation-screen harness + tracing (removed after). Lessons recorded in odd/lessons-for-sibling-app.md |
| T6a fix 3 | `11c7cfa` (branch `tui-v0`) | User feedback: modals stretched full-screen height. wrapDialog takes explicit content height; error modal capped at 20 rows with scroll. Lesson C5 added to the sibling registry |
| T6b | `ae22ba4` (branch `tui-v0`) | F5/Shift+F5/F6/Shift+F6 wired TC-style; byte progress via QueueTotals+ItemProgress; overwrite confirmation on copy AND cross-directory move (silent Overwrite:true caught in parent review — data-loss risk). Build, vet, 85 tests, linux build observed directly. Visual smoke: pending user test |
| T6d | `9980525` (branch `tui-v0`) | Root cause confirmed empirically (dumped dir-copy event stream: one folder-level ItemStarted, items=1, no file names). RED observed by worker (undefined `ItemFile`/`TotalFiles` build failure) → GREEN: new tests `TestCopyQueue_DirEmitsItemFile` (3 files, per-file Source/Size, TotalFiles=3) and `TestMoveQueue_FallbackForwardsItemFile` (2 files forwarded). Parent re-verified directly: `go build ./...`, `go vet ./...`, `go test ./... -count=1` all green; reviewed copy.go/operations.go diff line-by-line. Visual smoke: pending user retest |

## Decisions log

- 2026-10-12: Go + tview chosen over Rust+ratatui (learning-first, Go 1.26.4
  already installed; Rust absent). Sibling GUI project (Flutter) will consume
  this spec; code is not shared.
- 2026-10-12: Repository created outside the REA repository, per REA's
  repository guidelines.
- 2026-10-12 (T1 review note): the global `q`/`Q` quit binding is temporary
  scaffold convenience; it must be removed or scoped once the incremental
  filter input exists (S6, T7), or typing "q" in the filter would quit.
- 2026-10-12 (T3 design): copy is chunked buffered I/O (`io.CopyBuffer`,
  1–4 MB configurable buffer, cancellation checked between chunks, throttled
  progress callbacks) — never byte-by-byte. The file queue stays sequential
  in v0 by spec (S3) matching MC/TC default behavior; the queue consumes a
  `Copier` interface so bounded parallelism can be added in v1 without
  changing the queue contract. Copies preserve mtime (`os.Chtimes`).
- 2026-10-12 (partial-test feedback): Backspace is the primary parent-dir
  key (user request); Ctrl+PgUp also kept bound to parent (stock TC binds it
  so — user may ask to remove it). Cursor row replaced with a full-width
  solid color bar (black-on-green focused / black-on-grey unfocused) after
   the user reported the reverse-video cue was too subtle.
 - 2026-10-12 (keyboard fidelity, user requirement): respect Total Commander
   standard keyboard behavior as far as a terminal allows. v0 map: Tab focus;
  Enter descend; Ctrl+PgUp parent; Insert select (cursor advances); Space
  select (dir sizes v1); +/-/* group select/deselect/invert on main-row keys
  (terminal adaptation — numpad codes unreliable); F3/F4/F5/F7/F8 per TC;
  F6 shared move-or-rename dialog like TC; Shift+F6 rename in place;
  Shift+F5 duplicate in same dir; F8 and Delete both delete; Esc closes the
  topmost dialog and only quits from the main view (documented TUI
  adaptation); F10 quits (no menu bar in v0); Ctrl+Alt+letter quick search;
   Alt+F1/Alt+F2 drive switch per panel. v1 candidates recorded: Shift+F2
   compare directories, Ctrl+M multi-rename, Ctrl+D dir hotlist, folder
   tabs (Ctrl+T/Ctrl+W), Space dir-size calculation, Enter executing files.
 - 2026-10-12 (partial-test feedback, round 2): Ctrl+PgUp/Ctrl+PgDn scroll
   the cursor by a page within the folder (user request); parent-dir keys
   are Backspace only. Cursor bar: muted darkgreen (focused) computed from
   the view's real inner width each frame — a fixed 100-col padding wrapped
   lines and bled the background into extra rows (looked like everything
   selected).
 - 2026-10-12 (partial-test feedback, round 3): tview `[-]` resets only the
   foreground — cursor-row background bled into all following rows (shrunk
   from the top as the cursor moved). Fixed with `[-:-]` (fg+bg reset).
   Follow-up recorded: Ctrl+PgUp/Ctrl+PgDn page step should equal the
   visible row count of the pane (TC behavior), not the current fixed ±10.
