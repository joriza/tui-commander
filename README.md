# tui-commander

A behavior-first, Midnight Commander-inspired dual-panel terminal file manager.
Written in **Go** — pure Go, no cgo, no external dependencies at run time:
a single static binary. Copy and run, nothing to install.

**Status:** v0 feature-complete; v1 in progress on branch `tui-v1`.
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
4. Use a terminal that handles ANSI and UTF-8: Windows Terminal, or a Windows 10+
   console with a UTF-8 code page. Legacy OEM code pages may render non-ASCII
   names incorrectly.
5. There is no configuration file. Behavior is fixed to the platform, so nothing
   has to be carried alongside the binary.

## Keys

| Key | Action |
| --- | --- |
| `Tab` | Switch the active panel |
| Arrows, `Home`, `End` | Move the cursor |
| `PgUp` / `PgDn`, `Ctrl+PgUp` / `Ctrl+PgDn` | Move the cursor by 10 lines |
| `Enter` | Descend into the directory under the cursor |
| `Backspace` | Go to the parent directory |
| Letter keys | Quick search: a window narrows the listing to names containing what you type (case-insensitive). `Esc` cancels, `Enter` jumps to the first match |
| `Insert` / `Space` | Select the cursor entry; `Insert` also advances |
| `+` / `-` / `*` | Select by mask / deselect by mask / invert the selection |
| `Alt+F1` / `Alt+F2` | Choose a drive or volume for the left / right panel |
| `F2` | Reread the panel |
| `F3` | View the cursor file in the built-in pager |
| `F4` | Open the cursor file in the external editor |
| `F5` / `Shift+F5` | Copy / duplicate |
| `F6` / `Shift+F6` | Move or rename / rename in place |
| `F7` | Create a directory |
| `F8` or `Delete` | Delete |
| `Esc` | Close the topmost dialog, or clear the selection on the main view |
| `F10` | Quit |

Copy, move and delete act on the selection set when one exists, and on the
cursor entry otherwise. A multi-selection is always reported as a count, never
as a list of names.

## Behavior notes

**Deleting follows the operating system.** On Windows, a path on a fixed volume
goes to the Recycle Bin; a path on a removable, network or unknown volume is
deleted permanently. That mirrors what Explorer does, and there is no setting to
change it. On Linux and other platforms the delete is always permanent — the
platform exposes no recycler API that the binary can call reliably (see the
decisions log for the full reasoning). This is the one intentional difference
for a future GUI sibling.

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

## License

MIT.
