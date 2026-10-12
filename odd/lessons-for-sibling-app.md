# Lessons for the Sibling Application

Hard-won fixes and decisions from tui-commander, recorded so the GUI sibling
(and any rewrite) does not repeat the same mistakes. **Maintenance rule: every
bug fix or behavior decision appends a row here in the same commit.**

## Concurrency & UI-thread marshalling

| # | Lesson | Rule for the sibling |
| --- | --- | --- |
| C1 | Calling the UI-thread marshalling primitive (`QueueUpdateDraw`) from code that already runs on the UI thread (key handlers, form callbacks) deadlocks the event loop: invisible dialog, dead keyboard, no recovery. | Every function documents which thread it runs on. Marshalling happens at exactly one boundary point; callbacks crossing it state their side. |
| C2 | Nesting the marshalling primitive inside a queued update self-deadlocks (unbuffered internal channel: the nested send waits for a loop iteration that cannot start until the outer update returns). Symptom: operation completes on disk, progress dialog never closes. | Callbacks delivered by a runner/queue are already on the UI thread. UI-facing helpers they call must mutate state directly. Never re-marshal inside marshalled code. |
| C3 | Diagnose UI deadlocks empirically: headless harness with a simulation screen + temporary event tracing found in minutes what pure reasoning could not. | Reproduce first, fix second. Remove harness/traces before commit. |
| C4 | Invisible dialogs look exactly like freezes. | Every modal gets a visible border and title from day one. |
| C5 | Modals stretched to full screen height: the layout container gave the content proportional size instead of its natural size. | Size dialogs to their content (rows of items/text + chrome); cap scrollable overflows (error list ≤ 20 rows). Flutter analogue: size dialogs to child, never expand. |

## Text rendering & styling

| # | Lesson | Rule for the sibling |
| --- | --- | --- |
| R1 | tview `[-]` resets only the foreground; the row background bled into every following row (looked like mass selection shrinking from the top as the cursor moved). | Style resets must cover all attributes (`[-:-]`). In Flutter terms: scope every style explicitly; never rely on "reset" semantics that are partial. |
| R2 | Fixed-width padding wider than the panel wraps lines and carries the highlight background into extra visual rows (looked like "everything selected at startup"). | Compute highlight width from the real viewport each frame (re-render on resize); truncate long lines instead of wrapping. |

## Platform quirks

| # | Lesson | Rule for the sibling |
| --- | --- | --- |
| P1 | `os.RemoveAll` returns `nil` for nonexistent paths — silent success where the user expects an error. | Stat-check before delete (or equivalent) so per-item errors are honest. |
| P2 | On Windows the readonly attribute blocks removal; `os.Chmod(0666)` clears it. Attribute clearing is best-effort; removal errors stay per-item. | Handle attribute clear before delete; never abort a queue on one locked item. |
| P3 | Numpad key codes are unreliable across terminals; Ctrl+Alt combos and F-keys mostly survive. | Keyboard adaptations must be documented per key, never silent. |

## Keyboard & UX decisions (carried to the sibling)

- TC-standard map: Tab focus; Enter descend; Backspace parent; Insert select
  (cursor advances); Space select; +/−/* group select/invert on main-row keys;
  F3 view, F4 edit, F5 copy, F6 shared move-or-rename dialog, Shift+F5
  duplicate, Shift+F6 rename in place, F7 mkdir, F8 and Delete delete;
  Ctrl+Alt+letter quick search; Alt+F1/Alt+F2 drive switch.
- Esc closes the topmost dialog and NEVER quits; F10 always quits.
- Ctrl+PgUp/Ctrl+PgDn scroll by page — step must equal the visible row count
  (TC behavior); current fixed ±10 is a recorded follow-up.
- F6 is ONE dialog: same directory → rename, different → move (decided by
  `core.SameDirectory`).
- Copy/move destination defaults to the other panel's path (TC behavior).

## Engine design decisions (carried to the sibling)

- Copy engine: chunked buffered I/O (`io.CopyBuffer`, 1 MiB default, clamped
  64 KiB–8 MiB), cancellation checked between chunks, throttled progress
  callbacks. Never byte-by-byte.
- Sequential file queue in v0 (matches TC/MC); the queue consumes a `Copier`
  interface so bounded parallelism can be added later without changing the
  contract.
- Copies preserve mtime; overwrite policy lives in core as a parameter;
  confirmation lives in the UI.
- Per-item errors never abort the queue; unprocessed items after cancellation
  are reported as skipped.
- Destination-inside-source guard (cleaned absolute paths) prevents
  infinite-recursion copies.
- Progress granularity: the copy/move queue is item-granular — a directory is
  ONE queue item, so only the folder name is ever reported. File-level UI
  progress (current file name + a file count) requires the engine to emit an
  explicit per-file event (here `ItemFile`), never inference from item events.
- Derived counters need a single source of truth: never set a running total
  from queue totals AND increment it again per started item — that double-count
  showed "1/2" for a one-item copy.

## Process lessons (applies to any AI-assisted project)

- Delegated workers may end sessions without a final report: always diagnose
  repository state directly before resuming, and verify checks yourself
  before recording evidence. Never invent RED/GREEN observations.
- Keep a per-task plan with spec assertions (S1–S8) and an evidence table;
  update both in the same commit as the fix they document.
