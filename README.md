# tui-commander

A behavior-first, Midnight Commander-inspired dual-panel terminal file manager.
Single static binary — copy and run, nothing to install.

**Status:** work in progress (v0). See [odd/tasks/tui-v0.md](odd/tasks/tui-v0.md).

Lessons and fixes for the future GUI sibling:
[odd/lessons-for-sibling-app.md](odd/lessons-for-sibling-app.md) — updated with every fix.

## Build

Requires Go 1.26+.

```bash
go build -ldflags="-s -w" -o tui-commander.exe .
```

The result is a portable executable: copy it anywhere and run it. No runtime,
no dependencies, no installer.

## Run

```bash
./tui-commander.exe
```

## License

MIT.
