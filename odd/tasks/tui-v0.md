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
- [ ] T2: Core path platform adapter — Windows long-path handling, name
      validation (reserved names, trailing dots/spaces), cross-platform join
      and display formatting; unit tests (test-first).
- [ ] T3: Core copy engine — selection set → sequential queue → per-item +
      total progress → cancellation; error collection per item; unit tests
      with temp filesystem (test-first).
- [ ] T4: Core move/delete/rename/mkdir — move reuses copy engine (rename
      fast path when same volume), delete with explicit attribute handling;
      unit tests (test-first).
- [ ] T5: Dual panel UI — real listings, per-panel state, cursor, sorting,
      focus switching (S1, S2).
- [ ] T6: F-key wiring — F5/F6/F7/F8/Shift+F6 bound to core operations with
      confirmation dialogs and progress view (S3–S5).
- [ ] T7: Incremental filter + drive switching (S6, S2).
- [ ] T8: View/Edit + polish — F3 pager, F4 editor handoff, README final,
      optimized static build (`-ldflags="-s -w"`), smoke test on a real
      folder tree, portable-run checklist.

## Evidence

| Task | Commit | Checks observed |
| --- | --- | --- |
| T1 | `f290617` (branch `tui-v0`) | `go build ./...`, `go vet ./...`, `go test ./...` all green; observed directly by parent session (build, vet, test rerun) |

## Decisions log

- 2026-10-12: Go + tview chosen over Rust+ratatui (learning-first, Go 1.26.4
  already installed; Rust absent). Sibling GUI project (Flutter) will consume
  this spec; code is not shared.
- 2026-10-12: Repository created outside the REA repository, per REA's
  repository guidelines.
- 2026-10-12 (T1 review note): the global `q`/`Q` quit binding is temporary
  scaffold convenience; it must be removed or scoped once the incremental
  filter input exists (S6, T7), or typing "q" in the filter would quit.
