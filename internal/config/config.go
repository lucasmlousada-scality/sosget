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
	keyZendeskToken = "zendesk-token"
	keySFTPPass     = "sftp-pass"
)

type Config struct {
	ZendeskDomain string `json:"zendesk_domain"`
	ZendeskEmail  string `json:"zendesk_email"`
	ZendeskToken  string `json:"-"`
	SFTPHost      string `json:"sftp_host"`
	SFTPPort      int    `json:"sftp_port"`
	SFTPUser      string `json:"sftp_user"`
	SFTPPass      string `json:"-"`
	SFTPBasePath  string `json:"sftp_base_path"`
}

type fileConfig struct {
	ZendeskDomain string `json:"zendesk_domain"`
	ZendeskEmail  string `json:"zendesk_email"`
	SFTPHost      string `json:"sftp_host"`
	SFTPPort      int    `json:"sftp_port"`
	SFTPUser      string `json:"sftp_user"`
	SFTPBasePath  string `json:"sftp_base_path"`
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

	cfg := &Config{
		ZendeskDomain: fc.ZendeskDomain,
		ZendeskEmail:  fc.ZendeskEmail,
		SFTPHost:      fc.SFTPHost,
		SFTPPort:      fc.SFTPPort,
		SFTPUser:      fc.SFTPUser,
		SFTPBasePath:  fc.SFTPBasePath,
	}

	if cfg.SFTPPort == 0 {
		cfg.SFTPPort = 22
	}

	cfg.ZendeskToken, _ = keyring.Get(keyringService, keyZendeskToken)
	cfg.SFTPPass, _ = keyring.Get(keyringService, keySFTPPass)

	// Zendesk is optional — missing token just means org name is prompted at runtime.
	if cfg.SFTPHost == "" || cfg.SFTPUser == "" {
		return nil, fmt.Errorf("incomplete SFTP credentials")
	}

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

	// Load existing config if present
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &fc)
	}

	fmt.Println("=== sosget configuration ===")
	fmt.Println("Press Enter to keep existing values.")
	fmt.Println()
	fmt.Println("--- Zendesk (optional) ---")
	fc.ZendeskDomain = prompt("Zendesk domain (e.g. scality.zendesk.com, leave blank to skip)", fc.ZendeskDomain)
	if fc.ZendeskDomain != "" {
		fc.ZendeskEmail = prompt("Zendesk email", fc.ZendeskEmail)
		zdToken := promptSecret("Zendesk API token (hidden)")
		if zdToken != "" {
			if err := keyring.Set(keyringService, keyZendeskToken, zdToken); err != nil {
				return fmt.Errorf("save zendesk token: %w", err)
			}
		}
	}
	fmt.Println()
	fmt.Println("--- SFTP ---")
	fc.SFTPHost = prompt("SFTP host", orDefault(fc.SFTPHost, "ftp.scality.com"))
	fc.SFTPUser = prompt("SFTP username", fc.SFTPUser)
	fc.SFTPBasePath = prompt("SFTP base path (customer folders root)", orDefault(fc.SFTPBasePath, "/"))

	sftpPass := promptSecret("SFTP password (hidden, leave empty to always prompt)")
	if sftpPass != "" {
		if err := keyring.Set(keyringService, keySFTPPass, sftpPass); err != nil {
			return fmt.Errorf("save sftp password: %w", err)
		}
	}

	if fc.SFTPPort == 0 {
		fc.SFTPPort = 22
	}

	data, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	fmt.Printf("\nConfiguration saved to %s\n", path)
	fmt.Println("Secrets stored in OS keyring.")
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

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
