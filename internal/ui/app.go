package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/lucasmlousada-scality/sosget/internal/config"
	"github.com/lucasmlousada-scality/sosget/internal/sftp"
)

// Status colors for visual feedback.
var (
	colorInfo    = color.NRGBA{R: 0x60, G: 0x60, B: 0x60, A: 0xff} // neutral grey
	colorLoading = color.NRGBA{R: 0x1e, G: 0x88, B: 0xe5, A: 0xff} // blue
	colorSuccess = color.NRGBA{R: 0x2e, G: 0x7d, B: 0x32, A: 0xff} // green
	colorError   = color.NRGBA{R: 0xc6, G: 0x28, B: 0x28, A: 0xff} // red
)

type sosApp struct {
	win           fyne.Window
	cfg           *config.Config
	sftpClient    *sftp.Client
	selectedFiles []sftp.FileEntry

	lookupEntry    *widget.Entry
	otpEntry       *widget.Entry
	status         *canvas.Text
	progress       *widget.ProgressBar
	downloadDirLbl *widget.Label
	fileList       *fyne.Container
	downloadBtn    *widget.Button
	cancelBtn      *widget.Button
	connectBtn     *widget.Button

	cancelDownload context.CancelFunc
}

func Run(version string) {
	a := app.New()
	sa := &sosApp{}
	sa.win = a.NewWindow("sosget " + version + " — SOS Report Fetcher")
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
	sa.lookupEntry = widget.NewEntry()
	sa.lookupEntry.SetPlaceHolder("jane.doe or jane@company.com")

	sa.otpEntry = widget.NewPasswordEntry()
	sa.otpEntry.SetPlaceHolder("6-digit code from your authenticator app")

	sa.status = canvas.NewText("Enter customer email or username and OTP, then click Connect.", colorInfo)
	sa.status.TextSize = 13

	sa.downloadDirLbl = widget.NewLabel(sa.downloadDirText())
	sa.downloadDirLbl.Wrapping = fyne.TextTruncate

	sa.fileList = container.NewVBox()
	scroll := container.NewVScroll(sa.fileList)
	scroll.SetMinSize(fyne.NewSize(0, 300))

	sa.connectBtn = widget.NewButton("Connect", sa.onConnect)
	settingsBtn := widget.NewButton("⚙ Settings", sa.openSettings)

	sa.downloadBtn = widget.NewButton("Download Selected", sa.onDownload)
	sa.downloadBtn.Disable()

	sa.cancelBtn = widget.NewButton("Cancel", sa.onCancel)
	sa.cancelBtn.Hide()

	sa.progress = widget.NewProgressBar()
	sa.progress.Hide()

	form := widget.NewForm(
		widget.NewFormItem("Email / Username", sa.lookupEntry),
		widget.NewFormItem("OTP code", sa.otpEntry),
	)

	header := container.NewVBox(
		form,
		container.NewHBox(sa.connectBtn, settingsBtn),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Files (newest first):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
	)

	footer := container.NewVBox(
		widget.NewSeparator(),
		sa.status,
		sa.progress,
		container.NewHBox(sa.downloadBtn, sa.cancelBtn, sa.downloadDirLbl),
	)

	return container.NewBorder(header, footer, nil, nil, scroll)
}

func (sa *sosApp) onConnect() {
	input := strings.TrimSpace(sa.lookupEntry.Text)
	otp := sa.otpEntry.Text

	if input == "" {
		sa.setError("Please enter a customer email or username.")
		return
	}
	if otp == "" {
		sa.setError("Please enter your OTP code.")
		return
	}
	if sa.cfg == nil {
		sa.setError("Not configured — click ⚙ Settings first.")
		return
	}

	sa.connectBtn.Disable()
	sa.setLoading("Connecting to " + config.SFTPHost + "...")
	// OTP codes are single-use; clear the field so a stale code is not reused.
	sa.otpEntry.SetText("")
	sa.fileList.Objects = nil
	sa.fileList.Refresh()
	sa.selectedFiles = nil
	sa.downloadBtn.Disable()
	sa.downloadBtn.SetText("Download Selected")

	go func() {
		defer fyne.Do(func() { sa.connectBtn.Enable() })

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
			sa.setError("Connection failed: " + err.Error())
			return
		}
		sa.sftpClient = client

		var username string

		// Auto-detect: an "@" means it's an email (fuzzy search),
		// otherwise treat the input directly as a username.
		if !strings.Contains(input, "@") {
			// Direct mode: use exactly what the user typed as the folder name.
			// CustomerPathForUser will build /customers/chroot-<username>/home/<username>
			username = input
		} else {
			// Email mode: fuzzy-search chroot-* folders
			sa.setLoading("Searching for customer folder...")
			candidates, err := client.FindCustomerFolders(config.SFTPBasePath, input)
			if err != nil {
				sa.setError("Error scanning folders: " + err.Error())
				return
			}
			if len(candidates) == 0 {
				sa.setError("No folder found matching " + input)
				return
			}

			username = candidates[0]
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
					sa.setError("Cancelled.")
					return
				}
			}
		}

		remotePath := sftp.CustomerPathForUser(config.SFTPBasePath, username)
		sa.setLoading("Listing files at " + remotePath + "...")

		files, err := client.ListFiles(remotePath)
		if err != nil {
			sa.setError("Error listing files: " + err.Error())
			return
		}
		if len(files) == 0 {
			sa.setError("No files found at " + remotePath)
			return
		}

		fyne.Do(func() {
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
		})
		sa.setSuccess(fmt.Sprintf("%d file(s) found — select and click Download.", len(files)))
	}()
}

// maxConcurrentDownloads caps how many files transfer in parallel.
const maxConcurrentDownloads = 3

func (sa *sosApp) onDownload() {
	if sa.sftpClient == nil || len(sa.selectedFiles) == 0 {
		return
	}
	files := make([]sftp.FileEntry, len(sa.selectedFiles))
	copy(files, sa.selectedFiles)

	doDownload := func(destDir string) {
		ctx, cancel := context.WithCancel(context.Background())
		sa.cancelDownload = cancel

		// Total bytes across all selected files, for the aggregate progress bar.
		var totalBytes int64
		for _, f := range files {
			totalBytes += f.Size
		}

		// Per-file byte counters, summed for overall progress.
		progressByFile := make([]int64, len(files))
		var mu sync.Mutex

		refreshProgress := func() {
			mu.Lock()
			var sum int64
			for _, v := range progressByFile {
				sum += v
			}
			mu.Unlock()
			if totalBytes > 0 {
				fyne.Do(func() { sa.progress.SetValue(float64(sum) / float64(totalBytes)) })
			}
		}

		fyne.Do(func() {
			sa.downloadBtn.Disable()
			sa.connectBtn.Disable()
			sa.cancelBtn.Show()
			sa.progress.SetValue(0)
			sa.progress.Show()
		})

		go func() {
			defer fyne.Do(func() {
				sa.downloadBtn.Enable()
				sa.connectBtn.Enable()
				sa.cancelBtn.Hide()
				sa.progress.Hide()
			})
			defer cancel()

			sem := make(chan struct{}, maxConcurrentDownloads)
			var wg sync.WaitGroup
			var firstErr error
			var errMu sync.Mutex

			for i := range files {
				// Stop launching new transfers if cancelled or one already failed.
				if ctx.Err() != nil {
					break
				}
				wg.Add(1)
				sem <- struct{}{}
				go func(idx int, f sftp.FileEntry) {
					defer wg.Done()
					defer func() { <-sem }()

					progress := func(written, total int64) {
						mu.Lock()
						progressByFile[idx] = written
						mu.Unlock()
						refreshProgress()
					}
					sa.setLoading(fmt.Sprintf("Downloading %s...", f.Name))
					if err := sa.sftpClient.Download(ctx, f, destDir, progress); err != nil {
						errMu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						errMu.Unlock()
						cancel() // stop the other transfers
					}
				}(i, files[i])
			}

			wg.Wait()

			switch {
			case ctx.Err() != nil && firstErr == nil:
				sa.setError("Download cancelled.")
			case firstErr != nil:
				if ctx.Err() != nil {
					sa.setError("Download cancelled.")
				} else {
					sa.setError("Error: " + firstErr.Error())
				}
			default:
				sa.setSuccess(fmt.Sprintf("Done — %d file(s) saved to %s", len(files), destDir))
			}
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

func (sa *sosApp) onCancel() {
	if sa.cancelDownload != nil {
		sa.setLoading("Cancelling...")
		sa.cancelDownload()
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
		sa.setSuccess("Settings saved.")
	}, sa.win)
}

// setStatusColor updates the status text and its color, safe from any goroutine.
func (sa *sosApp) setStatusColor(msg string, c color.Color) {
	fyne.Do(func() {
		sa.status.Text = msg
		sa.status.Color = c
		sa.status.Refresh()
	})
}

func (sa *sosApp) setStatus(msg string)  { sa.setStatusColor(msg, colorInfo) }
func (sa *sosApp) setLoading(msg string) { sa.setStatusColor(msg, colorLoading) }
func (sa *sosApp) setSuccess(msg string) { sa.setStatusColor(msg, colorSuccess) }
func (sa *sosApp) setError(msg string)   { sa.setStatusColor(msg, colorError) }

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
