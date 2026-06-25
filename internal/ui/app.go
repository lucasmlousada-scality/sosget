package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lucasmlousada-scality/sosget/internal/config"
	"github.com/lucasmlousada-scality/sosget/internal/sftp"
)

type sosApp struct {
	win           fyne.Window
	cfg           *config.Config
	sftpClient    *sftp.Client
	selectedFiles []sftp.FileEntry

	emailEntry     *widget.Entry
	otpEntry       *widget.Entry
	status         *widget.Label
	downloadDirLbl *widget.Label
	fileList       *fyne.Container
	downloadBtn    *widget.Button
	connectBtn     *widget.Button
}

func Run() {
	a := app.New()
	sa := &sosApp{}
	sa.win = a.NewWindow("sosget — SOS Report Fetcher")
	sa.win.Resize(fyne.NewSize(740, 580))
	sa.win.SetMaster()

	sa.cfg, _ = config.Load()
	sa.win.SetContent(sa.buildUI())

	if sa.cfg == nil {
		// First run: open settings right away
		sa.openSettings()
	}

	sa.win.ShowAndRun()
}

func (sa *sosApp) buildUI() fyne.CanvasObject {
	sa.emailEntry = widget.NewEntry()
	sa.emailEntry.SetPlaceHolder("username@company.com")

	sa.otpEntry = widget.NewPasswordEntry()
	sa.otpEntry.SetPlaceHolder("6-digit code from Google Authenticator")

	sa.status = widget.NewLabel("Enter customer email and OTP, then click Connect.")
	sa.status.Wrapping = fyne.TextWrapWord

	sa.downloadDirLbl = widget.NewLabel(sa.downloadDirText())
	sa.downloadDirLbl.Wrapping = fyne.TextTruncate

	sa.fileList = container.NewVBox()
	scroll := container.NewVScroll(sa.fileList)
	scroll.SetMinSize(fyne.NewSize(0, 300))

	sa.connectBtn = widget.NewButton("Connect", sa.onConnect)
	settingsBtn := widget.NewButton("⚙ Settings", sa.openSettings)

	sa.downloadBtn = widget.NewButton("Download Selected", sa.onDownload)
	sa.downloadBtn.Disable()

	header := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Customer email", sa.emailEntry),
			widget.NewFormItem("OTP code", sa.otpEntry),
		),
		container.NewHBox(sa.connectBtn, settingsBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Files (newest first):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		sa.status,
		container.NewHBox(sa.downloadBtn, sa.downloadDirLbl),
	)

	return container.NewBorder(header, footer, nil, nil, scroll)
}

func (sa *sosApp) onConnect() {
	email := sa.emailEntry.Text
	otp := sa.otpEntry.Text

	if email == "" {
		sa.setStatus("Please enter a customer email.")
		return
	}
	if otp == "" {
		sa.setStatus("Please enter your OTP code.")
		return
	}
	if sa.cfg == nil {
		sa.setStatus("Not configured — click ⚙ Settings first.")
		return
	}

	sa.connectBtn.Disable()
	sa.setStatus("Connecting to " + config.SFTPHost + "...")
	sa.fileList.Objects = nil
	sa.fileList.Refresh()
	sa.selectedFiles = nil
	sa.downloadBtn.Disable()
	sa.downloadBtn.SetText("Download Selected")

	go func() {
		defer sa.connectBtn.Enable()

		if sa.sftpClient != nil {
			sa.sftpClient.Close()
			sa.sftpClient = nil
		}

		client, err := sftp.Connect(sftp.Config{
			Host:        config.SFTPHost,
			Port:        config.SFTPPort,
			Username:    sa.cfg.SFTPUser,
			Password:    sa.cfg.SFTPPass,
			OTPCode:     otp,
			TwoFADevice: sa.cfg.TwoFADevice,
		})
		if err != nil {
			sa.setStatus("Connection failed: " + err.Error())
			return
		}
		sa.sftpClient = client

		sa.setStatus("Searching for customer folder...")
		candidates, err := client.FindCustomerFolders(config.SFTPBasePath, email)
		if err != nil {
			sa.setStatus("Error scanning folders: " + err.Error())
			return
		}
		if len(candidates) == 0 {
			sa.setStatus("No folder found matching " + email)
			return
		}

		username := candidates[0]
		if len(candidates) > 1 {
			ch := make(chan string, 1)
			fyne.Do(func() {
				radio := widget.NewRadioGroup(candidates, nil)
				radio.SetSelected(candidates[0])
				d := dialog.NewCustomConfirm(
					"Multiple folders found — pick one",
					"Select", "Cancel",
					radio,
					func(ok bool) {
						if ok && radio.Selected != "" {
							ch <- radio.Selected
						} else {
							ch <- ""
						}
					}, sa.win)
				d.Show()
			})
			username = <-ch
			if username == "" {
				sa.setStatus("Cancelled.")
				return
			}
		}

		remotePath := sftp.CustomerPathForUser(config.SFTPBasePath, username)
		sa.setStatus("Listing files at " + remotePath + "...")

		files, err := client.ListFiles(remotePath)
		if err != nil {
			sa.setStatus("Error listing files: " + err.Error())
			return
		}
		if len(files) == 0 {
			sa.setStatus("No files found at " + remotePath)
			return
		}

		sa.fileList.Objects = nil
		for _, f := range files {
			label := fmt.Sprintf("%-60s  %8s  %s", f.Name, formatSize(f.Size), formatAge(f.ModTime))
			check := widget.NewCheck(label, func(checked bool) {
				if checked {
					sa.selectedFiles = append(sa.selectedFiles, f)
				} else {
					n := sa.selectedFiles[:0]
					for _, s := range sa.selectedFiles {
						if s.Path != f.Path {
							n = append(n, s)
						}
					}
					sa.selectedFiles = n
				}
				count := len(sa.selectedFiles)
				if count > 0 {
					sa.downloadBtn.SetText(fmt.Sprintf("Download Selected (%d)", count))
					sa.downloadBtn.Enable()
				} else {
					sa.downloadBtn.SetText("Download Selected")
					sa.downloadBtn.Disable()
				}
			})
			sa.fileList.Add(check)
		}
		sa.fileList.Refresh()
		sa.setStatus(fmt.Sprintf("%d file(s) found — select and click Download.", len(files)))
	}()
}

func (sa *sosApp) onDownload() {
	if sa.sftpClient == nil || len(sa.selectedFiles) == 0 {
		return
	}
	files := make([]sftp.FileEntry, len(sa.selectedFiles))
	copy(files, sa.selectedFiles)

	doDownload := func(destDir string) {
		sa.downloadBtn.Disable()
		go func() {
			defer sa.downloadBtn.Enable()
			for i, f := range files {
				sa.setStatus(fmt.Sprintf("Downloading %d/%d: %s", i+1, len(files), f.Name))
				if err := sa.sftpClient.Download(f, destDir); err != nil {
					sa.setStatus("Error: " + err.Error())
					return
				}
			}
			sa.setStatus(fmt.Sprintf("Done — %d file(s) saved to %s", len(files), destDir))
		}()
	}

	if sa.cfg != nil && sa.cfg.DownloadDir != "" {
		doDownload(sa.cfg.DownloadDir)
	} else {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			doDownload(uri.Path())
		}, sa.win)
	}
}

func (sa *sosApp) openSettings() {
	userEntry := widget.NewEntry()
	userEntry.SetPlaceHolder("your.username")

	passEntry := widget.NewPasswordEntry()
	passEntry.SetPlaceHolder("leave blank to keep existing")

	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder("/Users/you/Downloads")

	if sa.cfg != nil {
		userEntry.SetText(sa.cfg.SFTPUser)
		dirEntry.SetText(sa.cfg.DownloadDir)
	}

	browseBtn := widget.NewButton("Browse…", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err == nil && uri != nil {
				dirEntry.SetText(uri.Path())
			}
		}, sa.win)
	})

	dirRow := container.NewBorder(nil, nil, nil, browseBtn, dirEntry)

	twoFASelect := widget.NewSelect(config.TwoFADevices, nil)
	if sa.cfg != nil && sa.cfg.TwoFADevice != "" {
		twoFASelect.SetSelected(sa.cfg.TwoFADevice)
	} else {
		twoFASelect.SetSelected(config.TwoFADevices[0])
	}

	items := []*widget.FormItem{
		{Text: "SFTP Username", Widget: userEntry, HintText: "Your Scality SSO username"},
		{Text: "SFTP Password", Widget: passEntry, HintText: "Stored in OS keychain"},
		{Text: "Download folder", Widget: dirRow, HintText: "Where files are saved"},
		{Text: "2FA method", Widget: twoFASelect, HintText: "Device used for authentication"},
	}

	dialog.ShowForm("Settings", "Save", "Cancel", items, func(ok bool) {
		if !ok {
			return
		}
		if userEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("username cannot be empty"), sa.win)
			return
		}
		if err := config.SaveAll(userEntry.Text, passEntry.Text, dirEntry.Text, twoFASelect.Selected); err != nil {
			dialog.ShowError(err, sa.win)
			return
		}
		sa.cfg, _ = config.Load()
		sa.downloadDirLbl.SetText(sa.downloadDirText())
		sa.setStatus("Settings saved.")
	}, sa.win)
}

func (sa *sosApp) setStatus(msg string) {
	sa.status.SetText(msg)
}

func (sa *sosApp) downloadDirText() string {
	if sa.cfg != nil && sa.cfg.DownloadDir != "" {
		return "→ " + sa.cfg.DownloadDir
	}
	return "→ (no folder set)"
}

func formatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatAge(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02 15:04")
	}
}
