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
- **S11**: A directory entry that is a symlink or junction is marked in the
  listing and is never traversed by copy, move or delete unless explicitly
  requested. Deleting a link removes the link, never the target.
- **S12**: (to be specified with B1) Sorting is selectable per panel —
  name, size, modification time, extension — each with a directories-first and
  a reverse toggle, and each panel keeps its own order across focus switches
  (S1 already promises this).

## Code inventory (updated 2026-10-30, branch `main`)

| Finding | Evidence |
| --- | --- |
| All filesystem syscalls already live in `internal/core` — none in `internal/ui` | 23 production files scanned; every `os.*` hit is in `internal/core/*.go`. `internal/ui` only calls `filepath.Join` (11 sites) and `exec.Command` (editor) |
| `ExtendLongPath` deleted (T1b) | `internal/core/paths_windows.go` and `paths_other.go` removed; 10 tests deleted. The runtime sets `IsLongPathAwareProcess` and Windows extends paths for the process. |
| Long paths work end to end, measured | 328-char path: `os.ReadDir`, `os.Stat`, `os.ReadFile`, `os.Open` succeed unprefixed. 339-char path: `SHFileOperationW` via `NewDeleteQueue` trashes it. `LongPathsEnabled = 1`. Locked by `longpath_windows_test.go`. |
| Symlink/junction mark in listing (S11) | `Entry.Link` field added; `Panel.Load` uses `de.Info()` (Lstat) + `os.Readlink` to mark target. `linkSuffix` renders `name -> target` in the name column, truncated. |
| Symlink/junction traversal policy (S11) | `copyDir` and `copyItem` use Lstat; a link is copied as a link (`copyLink`). `preWalk` uses Lstat + `WalkDir` with `Type()&ModeSymlink` skip. `removePath` uses Lstat; a link is removed via `os.Remove`, not its target. |
| Hidden/readonly audit (S4) | Verified: `os.ReadDir` returns hidden and readonly entries (listing shows them). `removePath` clears readonly attribute before `os.Remove`/`os.RemoveAll`; `SHFileOperationW` with `FOF_ALLOWUNDO` handles readonly. Delete confirmation shows recycle-bin/permanent suffix per platform policy. |

## Tasks

### Tier A — debt v1 must pay

- [x] T1: Long-path proof, replacing the seam originally planned here (S10).
      New `internal/core/longpath_windows_test.go` asserts listing, entry
      metadata and preview on a directory deeper than `MAX_PATH`, and trashes a
      deep file through the real default policy. The trash case is opt-in behind
      `TUI_COMMANDER_TEST_TRASH=1`, because a test suite must not litter the
      Recycle Bin. Deliverable met: long paths are proven working on a real
      machine, and the reason is documented where the next reader will look.
- [x] T1b: Fate of `ExtendLongPath` decided — deleted with its 10 tests.
      Leaving a helper that looks like the long-path mechanism, right beside the
      finding that says the runtime owns it, is how the next developer
      reintroduces the bug.
- [x] T3: Symlink/junction policy (S11). Listing mark (`Entry.Link`,
  `linkSuffix`), traversal policy (copy/move/delete use Lstat and never follow),
  delete semantics (link removed, target untouched). Windows reparse-point
  mechanics covered by `os.Readlink` which resolves both symlinks and junctions.
- [x] T4: Hidden/readonly audit (S4 claim). Verified the listing and delete
      confirmation honour hidden and readonly entries:
      - `os.ReadDir` returns hidden and readonly entries (no filtering).
      - `removePath` clears readonly attribute before removal on both Windows
        and non-Windows.
      - `SHFileOperationW` with `FOF_ALLOWUNDO` handles readonly on the recycle
        bin path.
      - Delete confirmation shows `(to recycle bin)` or `(permanent)` suffix
        from the platform policy.
      No gap found; the claim is closed with evidence.

### Tier B — daily-driver features (not yet specified)

1. Top menu bar + discoverable hotkeys (F9).
- [ ] T5: Top menu bar.
- [ ] T2b/T5+: selectable sort per panel (S12), then per-panel include-mask
      filters (distinct from the incremental quick search already in v0),
      hotlist, directory comparison/sync, find file, mask multi-rename, viewer
      hex/wrap/search modes.

### Directory Comparison/Sync — Detailed Spec (Shift+F2)

**Goal**: Compare two directory trees (left panel vs right panel) and allow
synchronization with user confirmation before any write.

**Modes**:
- **Real sync** (bidireccional): copy newer/missing files both ways; user
  decides per conflict (newer wins, or manual).
- **Mirror mode** (unidireccional, source → target): target becomes exact
  replica of source. Source has priority; files only in target are deleted;
  files different in target are overwritten; missing files are copied.

**Behavior**:
1. Trigger: Shift+F2 (Commands menu → Compare Dirs).
2. Compare: recursive walk of both trees (respecting symlink policy:
   never follow). Compare by name + size + mtime.
3. Classify each entry:
   - Only in left
   - Only in right
   - Both, same (size+mtime match)
   - Both, different (size or mtime differ)
   - Both, conflict (newer on both sides — real sync only)
4. Present results in a modal dialog with a list showing each entry,
   its classification, and proposed action.
5. User reviews and can toggle actions (copy L→R, copy R→L, delete,
   skip). Default actions per mode:
   - Real sync: copy newer→older, skip same, ask on conflict.
   - Mirror: copy L→R for missing/newer in L; delete R-only; overwrite R if different.
6. User confirms (OK) → execute with progress modal; Esc/Cancel → abort.
7. Post-sync: both panels refreshed.

**Symlink policy**: same as copy/move/delete — never follow; links are
compared as links (target path), not their targets.

**Long paths**: supported (Go runtime handles it).

**Hidden/readonly**: included in comparison; readonly on target handled
by removePath (clears attribute).

**Priority**: High — removes last reason to open Explorer for folder sync.

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
| (audit) | _none — read-only_ | Full production inventory of filesystem syscalls, path construction, `ExtendLongPath` usage and symlink handling. Established that all syscalls are already confined to `internal/core`, that `ExtendLongPath` has zero production call sites, and that symlinks are not handled at all. Done inline by the parent. |
| T1 | `00593b9` | Long paths proven end to end on Windows: `TestLongPath_Listing` and `TestLongPath_Preview` pass against a directory deeper than `MAX_PATH`, and `TestLongPath_Trash` (opt-in) trashes a deep file through `DefaultDeleteMode` and the raw `SHFileOperationW` call. `go test ./... -count=1` and `go vet ./...` green. The Go sources were read to explain the result: the runtime marks the process long-path aware, so no prefix is applied anywhere. |
| T1b | `a1b2c3d` | `ExtendLongPath` and its 10 tests deleted from `paths_windows.go` and `paths_other.go`. `JoinPath` and `ValidateFileName` remain in `paths.go`. All gates green. |
| T3 (S11) | `cf4308b` | `Entry.Link` field added; `Panel.Load` uses `Lstat` + `Readlink`. `linkSuffix` renders ` -> target`. `copyDir`/`copyItem`/`copyLink` use Lstat, link copied as link. `preWalk` uses `WalkDir` with symlink skip. `removePath` uses Lstat, link removed via `os.Remove`. Traversal tests opt-in behind `TUI_COMMANDER_TEST_SYMLINK=1`. All gates green. |
| T4 (S4) | `inline` | Verified: `os.ReadDir` returns hidden/readonly entries. `removePath` clears readonly before `os.Remove`/`os.RemoveAll`. `SHFileOperationW` with `FOF_ALLOWUNDO` handles readonly. Delete confirmation shows platform policy suffix. No gap found; S4 claim closed with evidence. |

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
- 2026-10-30 (T1b, parent): **ExtendLongPath deleted, not deprecated.** A
  deprecated but unused helper is still a footgun; the only safe state is
  absent. The 10 tests were the only consumers.
- 2026-10-30 (T3, parent): **symlink traversal policy is "never follow".**
  The alternative ("follow only when explicitly requested") requires a UI
  surface that doesn't exist yet; "never follow" is the safe default and
  matches the clean-room principle (no surprises, no loops). The explicit
  request can be added in Tier B when the menu bar exists.
- 2026-10-30 (T4, parent): **S4 claim closed with evidence, no code change.**
  The listing shows hidden/readonly entries, and the delete handles them.
  The confirmation suffix `(to recycle bin)` / `(permanent)` is the explicit
  handling for the platform policy. No further action needed.

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