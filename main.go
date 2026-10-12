package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func main() {
	app := tview.NewApplication()

	leftList := tview.NewList().
		AddItem("Left: <path placeholder>", "", 0, nil)

	rightList := tview.NewList().
		AddItem("Right: <path placeholder>", "", 0, nil)

	flex := tview.NewFlex().
		AddItem(leftList, 0, 1, false).
		AddItem(rightList, 0, 1, false)

	keyBar := tview.NewTextView().
		SetText("F5 Copy  F6 Move  F7 MkDir  F8 Del  F10 Quit")

	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(flex, 0, 1, false).
		AddItem(keyBar, 1, 0, false)

	app.SetRoot(root, true)

	// Quit bindings: F10, q, Ctrl+C
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyF10:
			app.Stop()
			return nil
		case event.Rune() == 'q' || event.Rune() == 'Q':
			app.Stop()
			return nil
		}
		return event
	})

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
