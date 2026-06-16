# sosget

Desktop tool for Scality support engineers. Given a customer email, it connects to the Scality SFTP server, lists the files in their folder sorted by date, and lets you select and download them — all from a native GUI window.

---

## Prerequisites

### Go

Install Go 1.22 or later from https://go.dev/dl/

Verify:
```sh
go version
```

### Platform dependencies (required by the GUI framework — Fyne)

Fyne renders natively using OpenGL and requires a C compiler and system graphics libraries.

#### macOS

Install Xcode Command Line Tools:
```sh
xcode-select --install
```

That's it — no extra libraries needed.

#### Linux

Install GCC and the OpenGL/X11 development libraries. Pick the command for your distro:

```sh
# Debian / Ubuntu
sudo apt install gcc libgl1-mesa-dev xorg-dev

# Fedora / RHEL
sudo dnf install gcc mesa-libGL-devel libXcursor-devel libXrandr-devel \
     libXinerama-devel libXi-devel

# Arch
sudo pacman -S gcc mesa libxcursor libxrandr libxinerama libxi
```

#### Windows

1. Install [Go for Windows](https://go.dev/dl/)
2. Install [TDM-GCC](https://jmeubank.github.io/tdm-gcc/) (provides the C compiler `gcc.exe`)
3. Make sure both `go` and `gcc` are on your `PATH`

Verify:
```sh
go version
gcc --version
```

---

## Build

Clone the repo and build for your current platform:

```sh
git clone https://github.com/lucasmlousada-scality/sosget.git
cd sosget
make build
```

This produces a `sosget` binary (or `sosget.exe` on Windows) in the current directory.

### Manual build (if `make` is not available on Windows)

```sh
go build -o sosget ./cmd/sosget
```

---

## First run — configuration

Run the binary once to open the GUI:

```sh
./sosget          # macOS / Linux
sosget.exe        # Windows
```

On first launch the **Settings** dialog opens automatically. Fill in:

| Field | Value |
|---|---|
| SFTP Username | Your Scality SSO username (e.g. `lucas.mlousada`) |
| SFTP Password | Your Scality SSO password — stored in the OS keychain, never on disk |
| Download folder | Where downloaded files will be saved (defaults to `~/Downloads`) |

Click **Save**. Settings are written to:

- **macOS / Linux**: `~/.config/sosget/config.json` (username + folder only — password goes to the OS keychain)
- **Windows**: `%AppData%\sosget\config.json`

To change settings later, click **⚙ Settings** in the main window.

---

## Usage

1. Open a terminal and start the app:
   ```sh
   ./sosget
   ```

2. In the **Customer email** field enter the customer's SFTP login, e.g.:
   ```
   username@customer.com
   ```

3. Open **Google Authenticator** on your phone and enter the current 6-digit code in the **OTP code** field.

4. Click **Connect**.
   - The app connects to `ftp.scality.com` using your credentials.
   - It navigates to `/customers/chroot-{username}/home/{username}/`.
   - Files are listed sorted newest first.

5. Tick the files you want to download.

6. Click **Download Selected (N)**.
   - Files are saved to your configured download folder.
   - The status bar shows progress and confirms when done.

---

## Headless / scripted setup

If you need to configure the tool on a server without a display, use the CLI mode:

```sh
./sosget configure
```

This prompts for username and password in the terminal and writes the same config file that the GUI uses.

---

## Notes

- The SFTP host (`ftp.scality.com`), port (`22`), and base path (`/customers`) are hardcoded — they never change for Scality support use.
- Credentials are stored in the OS keychain (macOS Keychain, Windows Credential Manager, Linux Secret Service). The config file on disk contains only non-sensitive settings.
- Downloaded sosreports are excluded from git by `.gitignore` — they will never be accidentally committed if you run `sosget` from inside the repo directory.
