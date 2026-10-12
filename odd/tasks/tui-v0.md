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
- [ ] T7: selection + incremental filter (S6) + drive switching (S2).
      Split into three work units:
      - [x] T7a: multi-selection model in `core.Panel` + keys Insert
            (toggle + advance), Space (toggle), `+`/`-` (mask select/deselect),
            `*` (invert); selected rows highlighted and count shown in the
            panel title. Selection keyed by entry name, cleared on Load.
      - [x] T7b: incremental filter — typing a letter in the focused panel
            opens a small search window; the listing narrows live to entries
            whose name contains the typed text (substring, case-insensitive);
            Backspace edits; Esc cancels the filter and restores the full
            listing; Enter moves the cursor to the first match and closes the
            window. TC mode "Letter with search window" (Config / Quick search,
            4th option of the first group — confirmed by user 2026-10-12).
            Trigger is typing a letter, NOT Ctrl+Alt+letter as first drafted.
      - [x] T7c: drive/volume switching — Alt+F1 (left panel) / Alt+F2 (right
            panel) opens a volume list; choosing one loads that root into the
            panel. Windows: enumerate existing `A:\`..`Z:\` roots; Unix: `/`.
      - [ ] T7a2 (deferred): wire copy/move/delete to operate on the selection
            set (S3). Today they act on the cursor entry only; `selectedEntries`
            in app.go is dead code.
      - [x] T7d (T7 smoke-test polish, requested by user 2026-10-12):
            - [x] T7d1: choosing a drive with Alt+F1/Alt+F2 makes that panel the
                  ACTIVE panel (Total Commander behaviour); today focus stays
                  where it was after the dialog closes.
            - [x] T7d2: the name column fills the space left after the size and
                  date columns (proportional to the panel inner width) instead of
                  a fixed 30 chars; keeps a readable minimum so narrow
                  (unmaximized) windows do not lose the name.
            - [x] T7d3: very long names are truncated in the MIDDLE with an
                  ellipsis, preserving the start and the extension (e.g.
                  `very-long-fi….tar.gz`), instead of a plain end cut.
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
      (Superseded 2026-10-12: the consent prompt now renders after adding
      `"question": "allow"`; the blocked RDD lineage was abandoned — see the
      decisions log and evidence for details.)
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
| T6d | `9980525` (branch `tui-v0`) | Root cause confirmed empirically (dumped dir-copy event stream: one folder-level ItemStarted, items=1, no file names). RED observed by worker (undefined `ItemFile`/`TotalFiles` build failure) → GREEN: new tests `TestCopyQueue_DirEmitsItemFile` (3 files, per-file Source/Size, TotalFiles=3) and `TestMoveQueue_FallbackForwardsItemFile` (2 files forwarded). Parent re-verified directly: `go build ./...`, `go vet ./...`, `go test ./... -count=1` all green;       reviewed copy.go/operations.go diff line-by-line. Visual smoke: pending user retest |
| T7a | `d597e9c` (branch `tui-v0`) | Name-keyed selection model in `core.Panel` (cleared on Load), keys Insert/Space/`+`/`-`/`*`, yellow selected rows + `[N]` count in the panel title. Worker RED (undefined `ToggleSelectAtCursor`) → GREEN: 11 new core tests. Parent re-verified directly: `go build ./...`, `go vet ./...`, `go test ./... -count=1` all green; reviewed the panel.go/app.go diff line-by-line. `gofmt -l` flags `internal/core` repo-wide (CRLF, pre-existing — untouched `dirs.go` is also flagged). Visual smoke: pending user test |
| T7c | `02c0bfb` (branch `tui-v0`) | `core.Volumes()` enumerates roots (Windows `A:\`..`Z:\` filtered by `os.Stat`, single `/` elsewhere); `ShowListDialog` added to dialogs.go; Alt+F1 (left) / Alt+F2 (right) open a drive list and load the chosen root; Alt cases precede plain F2 (reread). Worker RED (undefined `Volumes`) → GREEN: 3 platform-aware tests. Parent re-verified directly: build, vet, `go test ./... -count=1`, linux cross-build+vet all green; reviewed diff line-by-line. Visual smoke: pending user test |
| T7b | `fe8f4c5` (branch `tui-v0`) | `core.Panel` gains `filter`/`visible` (visible = single source of truth for Entries/cursor/selection; never aliases `entries`), SetFilter/ClearFilter/Filter/FirstMatchIndex. UI: hidden bottom-pinned "Quick search" window; typing a printable letter (excluding space/+/-/*) opens it and narrows live; Backspace edits by rune; Esc cancels (filter cleared, cursor restored to search origin); Enter puts the cursor on the first match in the full listing and closes; any other key cancels and acts on the restored listing. Selection count is visible-scoped while filtering (hidden selections retained). Worker RED (undefined SetFilter) → GREEN: 10 new core tests. Parent re-verified directly: build, vet, tests, linux cross all green; reviewed diff line-by-line. Visual smoke: pending user test |
| RDD T7 slice | abandoned `review-4a4ed81f7b510552` (2026-10-12, `operator_disposition`, actor `orchestrator`) | Review of the T7 slice (13 paths / 1867 lines vs boundary `e9f1c32`) started on user consent (granted via native `question`); the `review-reliability` lens capture failed a 7th time with `opencode_task_output_empty` (reviewer Task completes without producing a result — this runtime's reviewer transport returns nothing; identical failure across 7 lineages since T6d). Relaunched once after the fresh STATUS re-offered the same bound slot; failed identically. Per contract, no indefinite retries: user chose to abandon the lineage at minimal cost. Quarantine: `.git/gentle-ai/review-transactions/quarantine/review-4a4ed81f7b510552-4132857914`. Verification stands on ordinary evidence: worker test-first (RED→GREEN), parent line-by-line diff review, `go build ./...`, `go vet ./...`, `go test ./... -count=1` and linux cross-build/vet all green per task. RDD remains enabled; a later review would need a compatible runtime (Claude Code/Codex) or a working reviewer transport. |
| T7b fix | `8258ac3` (branch `tui-v0`) | User smoke test (2026-10-12): the quick search filter works with plain letters, but the search window overlapped the bottom key bar. Fixed in `buildUI`: `searchWrap` gains a fixed 2-row bottom spacer so the window renders above the key bar (which occupies the last row of the main page). Parent re-verified directly: `go build ./...`, `go vet ./...`, `go test ./... -count=1` all green (no deterministic UI-layout test exists — structural check only). Manual retest: user |
| T7d | `cba96bc` (branch `tui-v0`) | Worker (gentle-ai-worker) RED (undefined `nameColumnWidth`/`truncateMiddle`) → GREEN: new `internal/ui/render_test.go` (TestTruncateMiddle 4 subtests, TestTruncateMiddleLongASCII, TestTruncateMiddleMultiByteCountsRunes, TestNameColumnWidth 4 subtests, TestNameColumnWidthNonDecreasing all PASS). Changes: `renderPanel` name column now `nameColumnWidth(width)` (fills the space left after the fixed size 8 + date 16 + separators 4 = 28 columns, minimum 20) plus `truncateMiddle` (middle `…`, keeps the extension); `driveDialog` sets `focusIndex` so the chosen drive's panel becomes active. Parent re-verified directly: `go build ./...`, `go vet ./...`, `go test ./... -count=1` all green (`internal/ui` now has tests); reviewed app.go + render_test.go line-by-line. T7d1 (drive focus) has no deterministic test — interactive dialog callback; structural check only. Visual smoke: pending user test. RDD clone-local OFF (opencode reviewer transport broken) — ordinary checks only |

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
- 2026-10-12 (T7 filter design, user-confirmed): quick search uses the TC mode
  "Letter with search window" (Configuración → Búsqueda rápida, 4th option of
  the first group). The trigger is typing a plain letter in the focused panel
  (NOT Ctrl+Alt+letter as first drafted). Live behavior: the listing narrows to
  entries whose name contains the typed text (substring, case-insensitive);
  Backspace edits; Esc cancels the filter and restores the full listing; Enter
  puts the cursor on the first match and closes the window.
- 2026-10-12 (T7a scope): the selection model lives in `core.Panel`, keyed by
  entry name, cleared on `Load`. Keys: Insert (toggle + advance), Space
  (toggle), `+`/`-` (mask select/deselect), `*` (invert). Copy/move/delete
  still act on the cursor entry only — feeding the selection set into those
  operations (S3) is deferred to a separate work unit T7a2 to keep this one
  bounded.
- 2026-10-12 (T7b smoke test feedback, user): quick search works with plain
  letters, but the search window overlapped the bottom key bar. Fixed by
  adding a fixed 2-row bottom spacer to `searchWrap` in `buildUI`, so the
  window renders above the key bar instead of on top of it.
- 2026-10-12 (T7d design, user): the panel name column is now proportional to
  the panel inner width — it fills the space left after the fixed size (8) and
  date (16) columns plus separators (4), with a 20-column minimum — replacing
  the former fixed 30 chars, so unmaximized windows keep names readable. Long
  names are truncated in the MIDDLE with `…` (U+2026), preserving the start and
  the extension. Choosing a drive (Alt+F1/Alt+F2) makes that panel the active
  panel (Total Commander behaviour). "Start maximized" was left out: it is an
  OS/terminal-host concern, not the application's.
