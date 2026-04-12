package fileinfo

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo holds metadata about a filesystem entry.
type FileInfo struct {
	Name          string
	Path          string
	Size          int64
	Mode          os.FileMode
	ModTime       time.Time
	IsDir         bool
	IsSymlink     bool
	SymlinkTarget string
	SymlinkBroken bool
}

// Icon returns a Unicode icon character for the file type.
func Icon(fi FileInfo) string {
	if fi.IsSymlink {
		if fi.SymlinkBroken {
			return "⚠"
		}
		return "↪"
	}
	if fi.IsDir {
		return "▸"
	}
	ext := strings.ToLower(filepath.Ext(fi.Name))
	switch ext {
	case ".go", ".rs", ".py", ".js", ".ts", ".jsx", ".tsx", ".c", ".cpp", ".h", ".java", ".rb", ".swift", ".kt":
		return "◆"
	case ".md", ".markdown":
		return "≡"
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".ini", ".env":
		return "≈"
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar", ".tgz":
		return "◉"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".ico":
		return "▣"
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".flv":
		return "▶"
	case ".mp3", ".flac", ".wav", ".ogg", ".m4a", ".aac":
		return "♪"
	case ".pdf":
		return "≡"
	case ".sh", ".fish", ".zsh", ".bash":
		return "❯"
	case ".exe", ".bin", ".out":
		return "◈"
	default:
		return "·"
	}
}

// HumanSize returns a human-readable size string.
func HumanSize(size int64) string {
	const unit = 1024
	if size < unit {
		return formatInt(size) + " B"
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	prefixes := []string{"K", "M", "G", "T", "P", "E"}
	val := float64(size) / float64(div)
	return formatFloat(val) + " " + prefixes[exp] + "B"
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	negative := n < 0
	if negative {
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append([]byte{byte('0' + n%10)}, buf...)
		n /= 10
	}
	if negative {
		buf = append([]byte{'-'}, buf...)
	}
	return string(buf)
}

func formatFloat(f float64) string {
	// Simple one-decimal formatter without fmt dependency
	intPart := int64(f)
	frac := int64((f - float64(intPart)) * 10)
	if frac < 0 {
		frac = -frac
	}
	return formatInt(intPart) + "." + string(rune('0'+frac))
}
