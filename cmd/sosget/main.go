package main

import (
	"fmt"
	"os"

	"github.com/lucasmlousada-scality/sosget/internal/config"
	"github.com/lucasmlousada-scality/sosget/internal/ui"
)

func main() {
	// `sosget configure` stays available for headless/scripted setup
	if len(os.Args) > 1 && os.Args[1] == "configure" {
		if err := config.Configure(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	ui.Run()
}
