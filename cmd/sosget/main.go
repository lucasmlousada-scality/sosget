package main

import (
	"fmt"
	"os"

	"github.com/lucasmlousada-scality/sosget/internal/config"
	"github.com/lucasmlousada-scality/sosget/internal/sftp"
	"github.com/lucasmlousada-scality/sosget/internal/tui"
	"github.com/lucasmlousada-scality/sosget/internal/zendesk"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: sosget <ticket-id>")
		fmt.Fprintln(os.Stderr, "       sosget configure")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "configure":
		if err := config.Configure(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		if err := run(os.Args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}

func run(ticketID string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("%w\n\nRun 'sosget configure' to set up credentials", err)
	}

	fmt.Printf("Fetching ticket #%s from Zendesk...\n", ticketID)
	zd := zendesk.New(cfg.ZendeskDomain, cfg.ZendeskEmail, cfg.ZendeskToken)
	ticket, err := zd.GetTicket(ticketID)
	if err != nil {
		return fmt.Errorf("zendesk: %w", err)
	}
	fmt.Printf("Customer : %s\n", ticket.RequesterName)
	fmt.Printf("Org      : %s\n", ticket.OrgName)
	fmt.Println()

	fmt.Println("Connecting to SFTP (you will be prompted for OTP)...")
	client, err := sftp.Connect(sftp.Config{
		Host:     cfg.SFTPHost,
		Port:     cfg.SFTPPort,
		Username: cfg.SFTPUser,
		Password: cfg.SFTPPass,
		BasePath: cfg.SFTPBasePath,
	})
	if err != nil {
		return fmt.Errorf("sftp: %w", err)
	}
	defer client.Close()

	folder, err := client.FindCustomerFolder(ticket.OrgName)
	if err != nil {
		return fmt.Errorf("find folder: %w", err)
	}
	fmt.Printf("Found folder: %s\n\n", folder)

	files, err := client.ListFiles(folder)
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}
	if len(files) == 0 {
		fmt.Println("No files found in customer folder.")
		return nil
	}

	selected, err := tui.FilePicker(files)
	if err != nil {
		return fmt.Errorf("picker: %w", err)
	}
	if len(selected) == 0 {
		fmt.Println("No files selected.")
		return nil
	}

	destDir, _ := os.Getwd()
	fmt.Println()
	for _, f := range selected {
		fmt.Printf("Downloading %s ...", f.Name)
		if err := client.Download(f, destDir); err != nil {
			fmt.Printf(" FAILED: %v\n", err)
		} else {
			fmt.Printf(" done -> %s/%s\n", destDir, f.Name)
		}
	}
	return nil
}
