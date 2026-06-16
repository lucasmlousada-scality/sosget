package sftp

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	BasePath string
}

type FileEntry struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

type Client struct {
	sftpClient *sftp.Client
	sshClient  *ssh.Client
}

func Connect(cfg Config) (*Client, error) {
	storedPass := cfg.Password

	authMethods := []ssh.AuthMethod{
		ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			if instruction != "" {
				fmt.Println(instruction)
			}
			answers := make([]string, len(questions))
			for i, q := range questions {
				isPass := containsAny(strings.ToLower(q), "password", "passwd")
				if isPass && storedPass != "" {
					answers[i] = storedPass
					continue
				}
				fmt.Print(q)
				b, err := term.ReadPassword(int(os.Stdin.Fd()))
				fmt.Println()
				if err != nil {
					return nil, err
				}
				answers[i] = string(b)
			}
			return answers, nil
		}),
	}

	// Also try password auth as first method if we have a stored password.
	if storedPass != "" {
		authMethods = append([]ssh.AuthMethod{ssh.Password(storedPass)}, authMethods...)
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //nolint:gosec
		Timeout:         30 * time.Second,
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	sshConn, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}

	sftpConn, err := sftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return nil, fmt.Errorf("sftp init: %w", err)
	}

	return &Client{sftpClient: sftpConn, sshClient: sshConn}, nil
}

func (c *Client) Close() {
	c.sftpClient.Close()
	c.sshClient.Close()
}

// CustomerPath builds the deterministic SFTP path for a customer.
// Pattern: {basePath}/chroot-{localPart}/home/{localPart}
// where localPart is the portion of the email before '@'.
func CustomerPath(basePath, email string) string {
	localPart := email
	if i := strings.Index(email, "@"); i >= 0 {
		localPart = email[:i]
	}
	return path.Join(basePath, "chroot-"+localPart, "home", localPart)
}

// ListFiles returns all files (non-dirs) in dir, sorted newest first.
func (c *Client) ListFiles(dir string) ([]FileEntry, error) {
	entries, err := c.sftpClient.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []FileEntry
	for _, e := range entries {
		if e.IsDir() {
			// Also look one level deep for sosreport archives inside subdirs
			subEntries, err := c.sftpClient.ReadDir(path.Join(dir, e.Name()))
			if err == nil {
				for _, se := range subEntries {
					if !se.IsDir() {
						files = append(files, FileEntry{
							Name:    e.Name() + "/" + se.Name(),
							Path:    path.Join(dir, e.Name(), se.Name()),
							Size:    se.Size(),
							ModTime: se.ModTime(),
						})
					}
				}
			}
			continue
		}
		files = append(files, FileEntry{
			Name:    e.Name(),
			Path:    path.Join(dir, e.Name()),
			Size:    e.Size(),
			ModTime: e.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})
	return files, nil
}

// Download copies a remote file to destDir, showing a simple progress line.
func (c *Client) Download(f FileEntry, destDir string) error {
	remote, err := c.sftpClient.Open(f.Path)
	if err != nil {
		return err
	}
	defer remote.Close()

	localName := filepath.Join(destDir, filepath.Base(f.Name))
	local, err := os.Create(localName)
	if err != nil {
		return err
	}
	defer local.Close()

	_, err = io.Copy(local, remote)
	return err
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
