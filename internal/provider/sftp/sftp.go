package sftp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	gossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	gosftp "github.com/pkg/sftp"

	"github.com/mogglemoss/pelorus/internal/provider"
	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// Provider implements provider.Provider for remote filesystems over SSH/SFTP.
type Provider struct {
	label      string // human-readable, e.g. "nautilus (SSH)"
	host       string // SSH hostname or IP
	port       int    // SSH port, default 22
	user       string // SSH username
	sshClient  *gossh.Client
	sftpClient *gosftp.Client
}

// Connect establishes an SSH/SFTP connection to the given host.
// Auth is attempted in order: SSH agent, then each identity file.
func Connect(host string, port int, user string, identityFiles []string) (*Provider, error) {
	if port == 0 {
		port = 22
	}

	var authMethods []gossh.AuthMethod

	// 1. Try SSH agent.
	if agentSock := os.Getenv("SSH_AUTH_SOCK"); agentSock != "" {
		conn, err := net.Dial("unix", agentSock)
		if err == nil {
			agentClient := agent.NewClient(conn)
			authMethods = append(authMethods, gossh.PublicKeysCallback(agentClient.Signers))
		}
	}

	// 2. Try each identity file.
	for _, idFile := range identityFiles {
		data, err := os.ReadFile(idFile)
		if err != nil {
			continue
		}
		signer, err := gossh.ParsePrivateKey(data)
		if err != nil {
			// Skip encrypted keys (no passphrase prompt in v0.6).
			continue
		}
		authMethods = append(authMethods, gossh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("sftp: no usable authentication methods for %s@%s:%d", user, host, port)
	}

	cfg := &gossh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), //nolint:gosec // v0.6 known limitation
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	sshConn, err := gossh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("sftp: SSH dial %s: %w", addr, err)
	}

	sftpConn, err := gosftp.NewClient(sshConn)
	if err != nil {
		sshConn.Close()
		return nil, fmt.Errorf("sftp: create SFTP client: %w", err)
	}

	label := fmt.Sprintf("%s@%s (SSH)", user, host)

	return &Provider{
		label:      label,
		host:       host,
		port:       port,
		user:       user,
		sshClient:  sshConn,
		sftpClient: sftpConn,
	}, nil
}

// Close shuts down SFTP and SSH connections.
func (p *Provider) Close() error {
	var errs []string
	if p.sftpClient != nil {
		if err := p.sftpClient.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if p.sshClient != nil {
		if err := p.sshClient.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("sftp close: %s", strings.Join(errs, "; "))
	}
	return nil
}

// String returns the human-readable label for this provider.
func (p *Provider) String() string {
	return p.label
}

// Capabilities reports SFTP provider capabilities.
func (p *Provider) Capabilities() provider.Caps {
	return provider.Caps{
		CanSetPermissions: true,
		CanSymlink:        true,
		CanPreview:        true,
		CanTrash:          false,
		IsRemote:          true,
		SupportsArchive:   false,
	}
}

// List returns the directory contents at path.
func (p *Provider) List(path string) ([]fileinfo.FileInfo, error) {
	entries, err := p.sftpClient.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("sftp list %q: %w", path, err)
	}

	result := make([]fileinfo.FileInfo, 0, len(entries))
	for _, entry := range entries {
		fullPath := path + "/" + entry.Name()
		fi := osFileInfoToFileInfo(fullPath, entry)

		if entry.Mode()&os.ModeSymlink != 0 {
			fi.IsSymlink = true
			target, err := p.sftpClient.ReadLink(fullPath)
			if err == nil {
				fi.SymlinkTarget = target
				// Check if target exists (detect broken symlinks).
				if !filepath.IsAbs(target) {
					target = filepath.Dir(fullPath) + "/" + target
				}
				if _, err := p.sftpClient.Lstat(target); err != nil {
					fi.SymlinkBroken = true
				}
			}
		}

		result = append(result, fi)
	}
	return result, nil
}

// Stat returns metadata for a single path.
func (p *Provider) Stat(path string) (fileinfo.FileInfo, error) {
	info, err := p.sftpClient.Lstat(path)
	if err != nil {
		return fileinfo.FileInfo{}, fmt.Errorf("sftp stat %q: %w", path, err)
	}
	fi := osFileInfoToFileInfo(path, info)
	if info.Mode()&os.ModeSymlink != 0 {
		fi.IsSymlink = true
		target, err := p.sftpClient.ReadLink(path)
		if err == nil {
			fi.SymlinkTarget = target
		}
	}
	return fi, nil
}

// Read returns a reader for the file at path.
func (p *Provider) Read(path string) (io.ReadCloser, error) {
	f, err := p.sftpClient.Open(path)
	if err != nil {
		return nil, fmt.Errorf("sftp read %q: %w", path, err)
	}
	return f, nil
}

// Copy copies a file from src to dst on the same SFTP server.
func (p *Provider) Copy(src, dst string) error {
	srcInfo, err := p.sftpClient.Lstat(src)
	if err != nil {
		return fmt.Errorf("sftp copy stat %q: %w", src, err)
	}
	if srcInfo.IsDir() {
		return p.copyDir(src, dst)
	}
	return p.copyFile(src, dst)
}

func (p *Provider) copyFile(src, dst string) error {
	in, err := p.sftpClient.Open(src)
	if err != nil {
		return fmt.Errorf("sftp copy open src %q: %w", src, err)
	}
	defer in.Close()

	out, err := p.sftpClient.Create(dst)
	if err != nil {
		return fmt.Errorf("sftp copy create dst %q: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("sftp copy %q -> %q: %w", src, dst, err)
	}
	return nil
}

func (p *Provider) copyDir(src, dst string) error {
	if err := p.sftpClient.MkdirAll(dst); err != nil {
		return fmt.Errorf("sftp copy mkdir %q: %w", dst, err)
	}

	entries, err := p.sftpClient.ReadDir(src)
	if err != nil {
		return fmt.Errorf("sftp copy readdir %q: %w", src, err)
	}

	for _, entry := range entries {
		s := src + "/" + entry.Name()
		d := dst + "/" + entry.Name()
		if entry.IsDir() {
			if err := p.copyDir(s, d); err != nil {
				return err
			}
		} else {
			if err := p.copyFile(s, d); err != nil {
				return err
			}
		}
	}
	return nil
}

// Move moves src to dst via SFTP Rename.
func (p *Provider) Move(src, dst string) error {
	if err := p.sftpClient.Rename(src, dst); err != nil {
		// Fall back to copy + delete.
		if err2 := p.Copy(src, dst); err2 != nil {
			return fmt.Errorf("sftp move copy phase: %w", err2)
		}
		if err2 := p.Delete(src); err2 != nil {
			return fmt.Errorf("sftp move delete phase: %w", err2)
		}
	}
	return nil
}

// Delete removes a file or directory (recursively) at path.
func (p *Provider) Delete(path string) error {
	info, err := p.sftpClient.Lstat(path)
	if err != nil {
		return fmt.Errorf("sftp delete stat %q: %w", path, err)
	}

	if !info.IsDir() {
		if err := p.sftpClient.Remove(path); err != nil {
			return fmt.Errorf("sftp delete %q: %w", path, err)
		}
		return nil
	}

	// Walk the directory tree bottom-up to delete recursively.
	walker := p.sftpClient.Walk(path)
	// Collect all paths first (walk is top-down), then delete bottom-up.
	var paths []string
	for walker.Step() {
		if err := walker.Err(); err != nil {
			continue
		}
		paths = append(paths, walker.Path())
	}

	// Delete in reverse order (deepest first).
	for i := len(paths) - 1; i >= 0; i-- {
		entry := paths[i]
		stat, err := p.sftpClient.Lstat(entry)
		if err != nil {
			continue
		}
		if stat.IsDir() {
			_ = p.sftpClient.RemoveDirectory(entry)
		} else {
			_ = p.sftpClient.Remove(entry)
		}
	}
	return nil
}

// MakeDir creates a directory (and all parents) at path.
func (p *Provider) MakeDir(path string) error {
	if err := p.sftpClient.MkdirAll(path); err != nil {
		return fmt.Errorf("sftp mkdir %q: %w", path, err)
	}
	return nil
}

// Rename renames (or moves) src to dst.
func (p *Provider) Rename(src, dst string) error {
	if err := p.sftpClient.Rename(src, dst); err != nil {
		return fmt.Errorf("sftp rename %q -> %q: %w", src, dst, err)
	}
	return nil
}

// --- helpers ---

func osFileInfoToFileInfo(path string, info os.FileInfo) fileinfo.FileInfo {
	return fileinfo.FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}
}

// --- SSH config parsing ---

// SSHConfigHost holds parsed values from a ~/.ssh/config Host block.
type SSHConfigHost struct {
	Alias         string
	HostName      string
	User          string
	Port          int
	IdentityFiles []string
}

// ParseSSHConfig reads and parses ~/.ssh/config, returning all non-wildcard Host entries.
// If the file does not exist, returns an empty slice without error.
func ParseSSHConfig() ([]SSHConfigHost, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("sftp: cannot determine home dir: %w", err)
	}

	cfgPath := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sftp: open ssh config: %w", err)
	}
	defer f.Close()

	currentUser, _ := osCurrentUsername()

	var hosts []SSHConfigHost
	var current *SSHConfigHost

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on first whitespace or '='.
		key, value, ok := splitConfigLine(line)
		if !ok {
			continue
		}

		switch strings.ToLower(key) {
		case "host":
			// Save previous host block.
			if current != nil {
				applyDefaults(current, home, currentUser)
				hosts = append(hosts, *current)
			}
			// Skip wildcard patterns.
			if value == "*" || strings.ContainsAny(value, "*?") {
				current = nil
			} else {
				current = &SSHConfigHost{
					Alias:    value,
					HostName: value, // default: same as alias
					User:     currentUser,
					Port:     22,
				}
			}

		case "hostname":
			if current != nil {
				current.HostName = value
			}

		case "user":
			if current != nil {
				current.User = value
			}

		case "port":
			if current != nil {
				if p, err := strconv.Atoi(value); err == nil {
					current.Port = p
				}
			}

		case "identityfile":
			if current != nil {
				expanded := expandSSHPath(value, home)
				current.IdentityFiles = append(current.IdentityFiles, expanded)
			}
		}
	}

	// Save last host block.
	if current != nil {
		applyDefaults(current, home, currentUser)
		hosts = append(hosts, *current)
	}

	return hosts, scanner.Err()
}

// splitConfigLine splits a config line into key and value.
// Handles both "Key Value" and "Key=Value" formats.
func splitConfigLine(line string) (key, value string, ok bool) {
	// Try '=' separator first.
	if idx := strings.IndexByte(line, '='); idx > 0 {
		key = strings.TrimSpace(line[:idx])
		value = strings.TrimSpace(line[idx+1:])
		return key, value, key != "" && value != ""
	}
	// Fall back to whitespace separator.
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return "", "", false
	}
	return parts[0], strings.Join(parts[1:], " "), true
}

// applyDefaults fills in default identity files if none were specified.
func applyDefaults(h *SSHConfigHost, home, _ string) {
	if len(h.IdentityFiles) == 0 {
		defaults := []string{
			filepath.Join(home, ".ssh", "id_rsa"),
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_ecdsa"),
		}
		for _, p := range defaults {
			if _, err := os.Stat(p); err == nil {
				h.IdentityFiles = append(h.IdentityFiles, p)
			}
		}
	}
}

// expandSSHPath expands ~ and %d (home directory) in SSH config paths.
func expandSSHPath(p, home string) string {
	p = strings.ReplaceAll(p, "%d", home)
	if p == "~" || strings.HasPrefix(p, "~/") {
		p = home + p[1:]
	}
	return p
}

// osCurrentUsername returns the current OS username, falling back to empty string.
func osCurrentUsername() (string, error) {
	// Use os/user if available; avoid import cycle by reading env var as fallback.
	if u := os.Getenv("USER"); u != "" {
		return u, nil
	}
	if u := os.Getenv("LOGNAME"); u != "" {
		return u, nil
	}
	return "", nil
}
