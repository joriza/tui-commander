# tui-commander

A behavior-first, Midnight Commander-inspired dual-panel terminal file manager.
Written in **Go** — pure Go, no cgo, no external dependencies at run time:
a single static binary. Copy and run, nothing to install.

**Status:** v1.0.0 released.

Spec and task log: [odd/tasks/tui-v0.md](odd/tasks/tui-v0.md) (v0),
[odd/tasks/tui-v1.md](odd/tasks/tui-v1.md) (v1).

Lessons and fixes for the future GUI sibling:
[odd/lessons-for-sibling-app.md](odd/lessons-for-sibling-app.md) — updated with every fix.

## Build

Requires Go 1.26+.

```bash
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o tui-commander.exe .
```

`CGO_ENABLED=0` guarantees a static binary (`tcell` is pure Go, so there is no
cgo to lose). `-trimpath` keeps local paths out of the binary and `-ldflags`
strips symbols, which roughly halves the size.

Cross-compile for another OS or architecture from the same tree:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o tui-commander .
```

In PowerShell, set the variables first:

```powershell
$env:CGO_ENABLED='0'; $env:GOOS='linux'; $env:GOARCH='amd64'
go build -trimpath -ldflags="-s -w" -o tui-commander .
```

## Run

```bash
./tui-commander.exe
```

## Portable checklist

1. Build with `CGO_ENABLED=0` (see above).
2. Copy the single executable to the target machine or a USB stick.
3. Run it. There is no installer, no runtime, no registry entry, and nothing to
   uninstall.
5. Use a terminal that handles ANSI and UTF-8: Windows Terminal, or a Windows 10+
   console with a UTF-8 code page. Legacy OEM code pages may render non-ASCII
   names incorrectly.
5. Configuration is stored in `%APPDATA%\tui-commander\config.toml` (Windows)
   or `~/.config/tui-commander/config.toml` (Linux). Run once to generate it,
   then copy the config alongside the binary for full portability.

## Keys

| Key | Action |
| --- | --- |
| `Tab` | Switch the active panel |
| Arrows, `Home`, `End` | Move the cursor |
| `PgUp` / `PgDn`, `Ctrl+PgUp` / `Ctrl+PgDn` | Move the cursor by 10 lines |
| `Enter` | Descend into the directory under the cursor |
| `Backspace` | Go to the parent directory |
| Letter keys | Quick search: a window narrows the listing to names containing what you type (case-insensitive). `Esc` cancels, `Enter` jumps to the first match. Arrow keys navigate results. | |
| `Insert` / `Space` | Select the cursor entry; `Insert` also advances |
| `+` / `-` / `*` | Select by mask / deselect by mask / invert the selection |
| `Alt+F1` / `Alt+F2` | Choose a drive or volume for the left / right panel (focus moves to that panel) |
| `F2` | Reread the panel |
| `F3` | View the cursor file (syntax highlighting for code, metadata for images/video/PDF) |
| `F4` | Open the cursor file in the external editor (`EDITOR` / `VISUAL`) |
| `F5` / `Shift+F5` | Copy / duplicate |
| `F6` / `Shift+F6` | Move or rename / rename in place |
| `F7` | Create a directory |
| `F8` or `Delete` | Delete |
| `Esc` | Close the topmost dialog, or clear the selection on the main view |
| `F9` | Open the menu bar |
| `F10` | Quit — asks for confirmation first; the default answer is No. Arrow keys or Space change selection. |
| `Ctrl+M` | Multi-rename (Total Commander style pattern syntax) |
| `Shift+F2` | Directory compare/sync (Real/Mirror modes) |
| `Ctrl+Q` | Quick View — preview file in the other panel |
| `Ctrl+Q` (in search) | Quick View while searching |

Copy, move and delete act on the selection set when one exists, and on the
cursor entry otherwise. A multi-selection is always reported as a count, never
as a list of names. `Esc` clears selection on the main view.

## Viewer (F3)

The built-in viewer supports:

| Type | Behavior |
| --- | --- |
| **Code** (140+ languages) | Syntax highlighting (Monokai theme), line wrap toggle (`w`), hex mode (`h`), search (`/`), next/prev match (`n`/`N`) |
| **Images** (PNG, JPEG, GIF, BMP, WebP) | Metadata: dimensions, frames (GIF), MIME type |
| **Video** (MP4, MKV, WEBM, MOV, AVI, FLV, WMV) | Format, MIME type |
| **Audio** (MP3, WAV, FLAC, OGG, M4A, AAC) | Format, MIME type |
| **PDF** | Extracted text content |
| **Other** | Plain text with line wrapping, binary detection |

Keys in viewer: `h`=hex, `w`=wrap, `/`=search, `n`/`N`=next/prev, `q`/`Esc`/`F3`/`F10`=close.

## Quick View (Ctrl+Q)

Press `Ctrl+Q` to show a preview of the focused file in the **other panel**.
Supports syntax highlighting, media metadata, and PDF text extraction.

## Multi-Rename (Ctrl+M)

Total Commander-style pattern syntax:

| Placeholder | Meaning |
| --- | --- |
| `[N]` | Name without extension |
| `[E]` | Extension (with dot) |
| `[C]` | Counter (auto-width) |
| `[C:N]` | Counter with N digits (e.g., `[C:3]` → 001, 002) |
| `[Y][M][D][H][I][S]` | Date/time from file mtime |
| `[P]` | Parent directory name |
| `[Z]` | File size in bytes |

Also supports search/replace (plain or regex), counter start/step/digits,
uppercase/lowercase, and extension auto-preservation. Preview shows 20 items
before executing.

## Directory Compare/Sync (Shift+F2)

Compares two directory trees (left/right panels).

| Mode | Behavior |
| --- | --- |
| **Real (bidirectional)** | Copy newer/missing both ways; conflicts require confirmation |
| **Mirror (left→right)** | Target becomes exact replica of source; target-only files deleted |

Shows action summary before execution. Conflicts require explicit confirmation.

## Quick Search

Type any letter in a panel to open the quick search window.
- Arrow keys (`↑`/`↓`/`PgUp`/`PgDn`/`Home`/`End`) navigate filtered results.
- `Esc` cancels and restores full listing.
- `Enter` selects the match and closes the search.

## Selection

| Key | Action |
| --- | --- |
| `Insert` / `Space` | Select entry (`Insert` advances) |
| `+` / `-` / `*` | Mask select / deselect / invert |
| `Esc` | Clear selection (on main view) |
| `Enter` on selection | Operates on all selected items |

Operations (copy/move/delete) act on the selection set when non-empty,
otherwise on the cursor entry. Multi-selection is always shown as a count.

## Configuration

Config file: `%APPDATA%\tui-commander\config.toml` (Windows) /
`~/.config/tui-commander/config.toml` (Linux/macOS).

Generated on first run. Editable fields:

| Section | Options |
| --- | --- |
| `panel` | `sort_by` (name/size/time/ext), `dirs_first` (bool), `reverse` (bool), `include_mask` (glob), `hotlist` (string array) |
| `editor` | `command` (string), `args` (array) |
| `ui` | `show_hidden`, `show_extensions`, `wrap_names`, `human_readable_sizes`, `show_mod_time`, `theme`, `language` |
| `keybindings` | All keys customizable (empty = default) |

Config menu (`Alt+O`): shows current settings, hotlist management, links to sort/mask dialogs.
"Save Config" button persists current panel state.

## Behavior notes

**Deleting follows the operating system.** On Windows, a path on a fixed volume
goes to the Recycle Bin; a path on a removable, network or unknown volume is
deleted permanently. That mirrors what Explorer does, and there is no setting to
change it. On Linux the delete is always permanent — the platform exposes no
recycler API that the binary can call reliably.

**The editor is external.** `F4` runs whatever `EDITOR` (or `VISUAL`) points to,
then rereads the panel. The value may include arguments and a quoted path with
spaces, for example:

```
set EDITOR="C:\Program Files\Microsoft VS Code\Code.exe" --wait
```

The panel is reread when the editor exits. If `EDITOR` is unset, `F4` explains
that instead of guessing an editor.

**F3 is a pager, not a hex viewer.** It samples at most 1 MiB, detects binary
content and caps very long lines, so large or binary files never hang the
terminal. `Esc`, `F3` and `F10` close it.

**Window starts maximized.** The application launches maximized by default.

**Progress dialogs** show current file name and file count (simplified to avoid
rendering issues).

## License

MIT — see [LICENSE](LICENSE).