//go:build windows

package main

import (
	"os"

	"pdf-watcher/internal/app"
	"pdf-watcher/internal/config"
	"pdf-watcher/internal/win"
)

func main() {
	release, ok := win.SingleInstance(config.AppName)
	if !ok {
		os.Exit(0)
	}
	defer release()
	if err := app.Run(); err != nil {
		os.Exit(1)
	}
}
