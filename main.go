package main

import (
	"os"
	"os/signal"
	"syscall"

	"tui-commander/internal/core"
	"tui-commander/internal/ui"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	leftPanel := core.NewPanel()
	if err := leftPanel.Load(cwd); err != nil {
		panic(err)
	}

	rightPanel := core.NewPanel()
	if err := rightPanel.Load(cwd); err != nil {
		panic(err)
	}

	app := ui.NewApp(leftPanel, rightPanel)

	// Graceful shutdown on Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT)
	go func() {
		<-sigCh
		app.Stop()
	}()

	if err := app.Run(); err != nil {
		panic(err)
	}
}
