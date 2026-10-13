# tui-commander — TUI v1

## Goal

Turn the working v0 into the file manager you actually live in: stop opening a
graphical file explorer, and never lose a file doing it.

The v0 feature set is already the daily surface (two panels, selection, copy,
move, rename, delete, mkdir, pager, editor, drive switching, quick search).
v1 is about **correctness under real-world trees** first, then the handful of
features that remove the last reasons to open Explorer.

Exit criteria for v1:

- Paths longer than `MAX_PATH` work end to end on Windows: browse, descend,
  preview, copy, move, rename, delete, edit.
- A symlink or junction is never followed blindly: a loop cannot hang or
  duplicate an operation, and deleting a link never deletes its target.
- The listing, the delete confirmation and every dialog are honest about
  hidden/readonly entries.
- Sorting and per-panel filters are user-selectable and persist per panel.
- The reachable surface is discoverable from a top menu, not only from a wall
  of hotkeys at the bottom.

## Inherited constraints (v0)

Go 1.26. UI is tview/tcell (pure Go, no cgo). Clean-room: behavior patterns
only, never assets or code from Total Commander or Midnight Commander.
Windows-first. The sibling GUI app implements this same spec, so behaviors stay
expressed as testable assertions. No configuration system was introduced in v0;
the delete policy stays fixed to the platform and is NOT configurable in v1
either (UI preferences and destructive semantics are different things).

New for v1:

- No dialog, error message or listing may ever show a `\\?\` prefix. That holds
  by construction: the program never builds one, because Windows extends long
  paths for the process, so the paths the app stores, compares and displays are
  always the natural ones.
- `internal/ui` performs no filesystem access of its own: it builds paths and
  renders what `internal/core` returns.

## Behavior spec v1 (testable assertions)

- **S9**: Every filesystem syscall in the program stays inside `internal/core`.
  No other production file calls `os.Open`, `os.Stat`, `os.ReadDir`,
  `os.Create`, `os.Remove`, `os.Rename`, `os.Mkdir*`, `os.Chmod`, `os.Chtimes`,
  `filepath.Walk` or their siblings on a path derived from user navigation, and
  `internal/ui` touches no filesystem at all. Already true today (audited); the
  assertion exists so it stays true. Long paths are explicitly **not** a reason
  to introduce a seam — see S10.
- **S10**: A path longer than `MAX_PATH` can be listed, descended into,
  previewed, copied, moved, renamed and deleted. The program adds no
  extended-length prefix of its own: the Go runtime marks the process long-path
  aware and Windows extends the path for it, so the paths the app stores,
  compares, sorts and displays keep their natural form and a failed syscall
  reports the path the user knows. The single call that does not go through the
  `os` package — `SHFileOperationW` in `delete_windows.go` — is covered by a
  build-tagged test instead of by a prefix.
- **S11**: (to be specified with A2) A directory entry that is a symlink or
  junction is marked in the listing and is never traversed by copy, move or
  delete unless explicitly requested. Deleting a link removes the link, never
  the target.
- **S12**: (to be specified with B1) Sorting is selectable per panel —
  name, size, modification time, extension — each with a directories-first and
  a reverse toggle, and each panel keeps its own order across focus switches
  (S1 already promises this).

## Code inventory (audited 2026-10-13, branch `tui-v1`)

| Finding | Evidence |
| --- | --- |
| All filesystem syscalls already live in `internal/core` — none in `internal/ui` | 23 production files scanned; every `os.*` hit is in `internal/core/*.go`. `internal/ui` only calls `filepath.Join` (11 sites) and `exec.Command` (editor) |
| `ExtendLongPath` is defined and tested, never called, and not needed | `internal/core/paths_windows.go:14`; 10 uses, all in `*_test.go`. Checked against the Go 1.26 sources: `runtime/os_windows.go` sets the PEB `IsLongPathAwareProcess` bit and then `internal/syscall/windows.CanUseLongPaths = true`, which turns `os.fixLongPath` (`os/path_windows.go`) into a no-op — the `os` package deliberately does not prefix, Windows extends for the process. Wiring it would duplicate that and leak the prefix into `*os.PathError`, dialogs and the editor path |
| Long paths work end to end here, measured | 328-character path: `os.ReadDir`, `os.Stat`, `os.ReadFile` and `os.Open` all succeed unprefixed. 339-character path: `SHFileOperationW` via `NewDeleteQueue` trashes it. `HKLM\SYSTEM\CurrentControlSet\Control\FileSystem\LongPathsEnabled = 1`. Locked down by `internal/core/longpath_windows_test.go` |
| Symlink/junction handling does not exist | zero hits for `Lstat`, `Readlink`, `Symlink`, `ModeSymlink`, `Reparse`, `junction` across all `*.go` |
- Production syscall sites that can receive an arbitrary user path: `copy.go`
  (15), `move.go` (8), `delete.go`/`delete_windows.go`/`delete_other.go` (4),
  `mkdir.go` (3), `panel.go` (2), `preview.go` (1), `rename.go` (2),
  `move_samevolume.go` (2). Sites in `volumes.go` (drive roots) and `dirs.go`
  (lexical comparison) are always shallow.
- Pre-existing off-by-one, not introduced here: `ExtendLongPath` prefixes only
  when `len(path) > 260`, so a path of exactly 260 characters is passed through
  unprefixed even though `MAX_PATH` includes the terminating NUL. Recorded as a
  follow-up; a test asserts the current behaviour, so changing it is its own
  decision.

## Tasks

### Tier A — debt v1 must pay

- [x] T1: Long-path proof, replacing the seam originally planned here (S10).
      New `internal/core/longpath_windows_test.go` asserts listing, entry
      metadata and preview on a directory deeper than `MAX_PATH`, and trashes a
      deep file through the real default policy. The trash case is opt-in behind
      `TUI_COMMANDER_TEST_TRASH=1`, because a test suite must not litter the
      Recycle Bin. Deliverable met: long paths are proven working on a real
      machine, and the reason is documented where the next reader will look.
- [ ] T1b: Decide the fate of the now-provably-dead `ExtendLongPath` and its 10
      tests. Leaving a helper that looks like the long-path mechanism, right
      beside the finding that says the runtime owns it, is how the next
      developer reintroduces the bug.
- [ ] T3: Symlink/junction policy (S11). Needs its own spec first: listing mark,
      traversal policy, delete semantics, and the Windows reparse-point
      mechanics.
- [ ] T4: Hidden/readonly audit (S4 claim). Verify the listing and the delete
      confirmation actually honour hidden and readonly entries, then either
      close the spec claim with evidence or fix the gap.

### Tier B — daily-driver features (not yet specified)

1. Top menu bar + discoverable hotkeys (F9).
- [ ] T5: Top menu bar.
- [ ] T2b/T5+: selectable sort per panel (S12), then per-panel include-mask
      filters (distinct from the incremental quick search already in v0),
      hotlist, directory comparison/sync, find file, mask multi-rename, viewer
      hex/wrap/search modes.

### Tier C — platform and delivery

- [ ] T6: Recycle bin on Linux via the XDG trash spec, removing the documented
      divergence.
- [ ] T7: Configuration file for UI preferences (panel order, sort, filters,
      keybindings, editor). Destructive semantics stay out of it.
- [ ] T6b: CI + release automation (windows/linux/darwin × amd64/arm64,
      checksums).

### Explicitly out of v1

FTP/SFTP, browsing archives as directories, mouse support, i18n, and the
sibling GUI application. Each is a project, not a task.

## Evidence

| Task | Commit | What was observed |
| --- | --- | --- |
| (audit) | _none — read-only_ | Full production inventory of filesystem syscalls, path construction, `ExtendLongPath` usage and symlink handling. Established that all syscalls are already confined to `internal/core`, that `ExtendLongPath` has zero production call sites, and that symlinks are not handled at all. Done inline by the parent: the subagent runtime refused every launch (`OpenCode's free tier can only be used from within OpenCode`), so delegation was unavailable and the degradation path was used instead |
| T1 | `00593b9` | Long paths proven end to end on Windows: `TestLongPath_Listing` and `TestLongPath_Preview` pass against a directory deeper than `MAX_PATH`, and `TestLongPath_Trash` (opt-in) trashes a deep file through `DefaultDeleteMode` and the raw `SHFileOperationW` call. `go test ./... -count=1` and `go vet ./...` green. The Go sources were read to explain the result: the runtime marks the process long-path aware, so no prefix is applied anywhere |

## Decisions log

- 2026-10-13 (architecture, parent): **corrected — the program builds no
  long-path prefix, and no seam is created.** The first version of this entry
  decided the opposite: one `internal/core/fsops.go` seam applying the prefix at
  the syscall boundary and de-extending the path Windows writes into a returned
  `*os.PathError`. That decision was taken before the evidence existed, and the
  evidence refutes it. The Go runtime sets the process's long-path-aware flag
  and Windows extends the path, so neither `os` nor this program needs a prefix.
  Measuring settled it: a probe showed every `os.*` call working on a
  328-character path, and the raw `SHFileOperationW` call working on a
  339-character path. The seam would have cost roughly fifteen call-site
  rewrites, leaked the prefix into error text and the editor path, and bought
  nothing. Stored paths keep their natural form because nothing ever rewrites
  them.
- 2026-10-13 (architecture, parent): **the symlink policy lives in
  `internal/core`, where the filesystem is read.** Replaces the earlier claim
  that the long-path seam would also carry it: no seam is being built, and none
  is needed, because the audit confirmed every syscall is already confined to
  `internal/core`. A2's question — "is this entry a link, and may I traverse
  it" — belongs beside the reads that produce the listing.
- 2026-10-13 (ordering, parent, accepted by user): **Tier A before Tier B.** A
  long path that cannot be opened and a junction that can duplicate a tree are
  defects in shipped behavior, not missing features, and paying them while the
  repository is small is much cheaper than paying them under a menu bar.
- 2026-10-13 (delivery): **one long-lived feature branch (`tui-v1`) with
  work-unit commits, PRs decided later.** Derived from the established v0
  convention (the whole feature landed as work-unit commits on `tui-v0`, no
  PRs), not asserted by the parent.
- 2026-10-13 (environment): **subagent delegation is unavailable in this
  runtime** — every `task` launch fails with `OpenCode's free tier can only be
  used from within OpenCode`, the same transport failure that turned RDD off for
  this clone. Workers are replaced by inline execution, and each work unit is
  therefore verified by the parent running the full check suite. Recorded so the
  missing delegated verification is visible rather than silent.

## Delivery

Forecast for v1 as scoped above: **roughly 2,500–3,500 authored changed lines**
(Tier A is small; Tier B dominates). Work units are kept reviewable on their
own, the branch stays shippable at every commit, and the per-task heuristic of
about 400 authored lines is planning guidance, not a cap.

## Checks

Every work unit must leave all of these green:

- `go build ./...`
- `go vet ./...`
- `go test ./... -count=1`
- `$env:GOOS='linux'; go build ./...; $env:GOOS=''`
- formatting clean on every file the work unit touches

`gofmt -l internal` used to be the check here and it is **not usable**: the
working tree mixes line endings (`core.autocrlf=true` materialises CRLF for the
files git has rewritten, while files written by the editor stay LF), and
`gofmt -l` reports every CRLF file as unformatted. Measured 2026-10-13: it flags
23 files that no one touched, while the LF files in the same package come back
clean. Until that is settled — `.gitattributes` with `* text=auto eol=lf` is the
likely fix — check formatting only on the files a change actually touches.

Windows-specific behaviour (long paths, the recycle bin, drive types) is
exercised by build-tagged tests that run on the Windows CI runner and are
skipped elsewhere.
