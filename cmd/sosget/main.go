package main

import (
	"fmt"
	"os"

	"github.com/lucasmlousada-scality/sosget/internal/config"
	"github.com/lucasmlousada-scality/sosget/internal/ui"
)

// version is injected at build time via -ldflags "-X main.version=...".
// Defaults to "dev" for local builds.
var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "configure":
			// `sosget configure` stays available for headless/scripted setup.
			if err := config.Configure(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "--version", "-v", "version":
			fmt.Printf("sosget %s\n", version)
			return
		case "--help", "-h", "help":
			printUsage()
			return
		}
	}
	ui.Run(version)
}

func printUsage() {
	fmt.Print(`sosget — SOS Report Fetcher

Usage:
  sosget              Launch the graphical interface
  sosget configure    Configure credentials (headless/scripted)
  sosget --version    Print version and exit
  sosget --help       Show this help
`)
}
