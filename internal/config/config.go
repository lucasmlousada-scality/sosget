package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"
	"golang.org/x/term"
)

const (
	keyringService = "sosget"
	keySFTPPass    = "sftp-pass"

	SFTPHost     = "ftp.scality.com"
	SFTPPort     = 22
	SFTPBasePath = "/customers"
)

type Config struct {
	SFTPUser string `json:"sftp_user"`
	SFTPPass string `json:"-"`
}

type fileConfig struct {
	SFTPUser string `json:"sftp_user"`
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sosget", "config.json"), nil
}

func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config not found: %w", err)
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if fc.SFTPUser == "" {
		return nil, fmt.Errorf("sftp_user not set — run 'sosget configure'")
	}

	cfg := &Config{SFTPUser: fc.SFTPUser}
	cfg.SFTPPass, _ = keyring.Get(keyringService, keySFTPPass)
	return cfg, nil
}

func Configure() error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	var fc fileConfig
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &fc)
	}

	fmt.Println("=== sosget configuration ===")
	fmt.Printf("SFTP host : %s (fixed)\n", SFTPHost)
	fmt.Printf("Base path : %s (fixed)\n\n", SFTPBasePath)

	fc.SFTPUser = prompt("Your SFTP username", fc.SFTPUser)

	sftpPass := promptSecret("Your SFTP password (stored in OS keyring, leave blank to always prompt)")
	if sftpPass != "" {
		if err := keyring.Set(keyringService, keySFTPPass, sftpPass); err != nil {
			return fmt.Errorf("save sftp password: %w", err)
		}
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	fmt.Printf("\nConfiguration saved to %s\n", path)
	return nil
}

// SaveUser writes username to the config file and optionally saves password to keyring.
// Called by the GUI settings dialog.
func SaveUser(user, pass string) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	fc := fileConfig{SFTPUser: user}
	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	if pass != "" {
		return keyring.Set(keyringService, keySFTPPass, pass)
	}
	return nil
}

func prompt(label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	var val string
	fmt.Scanln(&val)
	val = strings.TrimSpace(val)
	if val == "" {
		return defaultVal
	}
	return val
}

func promptSecret(label string) string {
	fmt.Printf("%s: ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
