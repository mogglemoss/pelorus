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

// IconStyle selects the glyph set used by Icon.
type IconStyle int

const (
	// IconStyleUnicode uses pure Unicode geometric glyphs — works in any
	// terminal, no font dependency. This is the default.
	IconStyleUnicode IconStyle = iota
	// IconStyleNerd uses Nerd Font devicons. Requires a Nerd Font installed
	// in the user's terminal (see nerdfonts.com). Unknown glyphs render as
	// tofu if the font isn't present.
	IconStyleNerd
)

var currentIconStyle IconStyle = IconStyleUnicode

// SetIconStyle configures which glyph set Icon returns. Call once at startup.
func SetIconStyle(s IconStyle) { currentIconStyle = s }

// iconKey identifies a semantic file kind. The unicode/nerd tables both key
// on this, so adding a new extension updates both mappings in one place.
type iconKey int

const (
	kDefault iconKey = iota
	kDir
	kSymlink
	kSymlinkBroken
	kExec
	kMarkdown
	kCodeCompiled
	kCodeScripting
	kCodeWeb
	kShell
	kConfig
	kArchive
	kImage
	kVideo
	kAudio
	kFont
	kPDF
	kDocument
	kSpreadsheet
	kDatabase
	kLog
	kPatch
	kDiskImage
	kLock
	kGit
	kDocker
	kBuildManifest
	kText
)

// unicodeIcons — single-width BMP glyphs, no font dependency.
var unicodeIcons = map[iconKey]string{
	kDefault:       "·",
	kDir:           "▸",
	kSymlink:       "↪",
	kSymlinkBroken: "⚠",
	kExec:          "◈",
	kMarkdown:      "≡",
	kCodeCompiled:  "◆",
	kCodeScripting: "◇",
	kCodeWeb:       "◊",
	kShell:         "❯",
	kConfig:        "≈",
	kArchive:       "◉",
	kImage:         "▣",
	kVideo:         "▶",
	kAudio:         "♪",
	kFont:          "ℱ",
	kPDF:           "◨",
	kDocument:      "▤",
	kSpreadsheet:   "▦",
	kDatabase:      "⛁",
	kLog:           "⋯",
	kPatch:         "±",
	kDiskImage:     "◍",
	kLock:          "⊘",
	kGit:           "⎇",
	kDocker:        "⎈",
	kBuildManifest: "⚒",
	kText:          "⍌",
}

// nerdIcons — codepoints from ryanoasis/nerd-fonts devicons.
var nerdIcons = map[iconKey]string{
	kDefault:       "\uf016", // nf-fa-file_o
	kDir:           "\uf07b", // nf-fa-folder
	kSymlink:       "\uf481", // nf-oct-file_symlink_file
	kSymlinkBroken: "\uf05e", // nf-fa-ban
	kExec:          "\uf489", // nf-oct-terminal
	kMarkdown:      "\ue73e", // nf-dev-markdown
	kCodeCompiled:  "\ue624", // nf-seti-c (generic compiled)
	kCodeScripting: "\ue606", // nf-seti-python (generic scripting)
	kCodeWeb:       "\ue736", // nf-dev-html5
	kShell:         "\uf489", // nf-oct-terminal
	kConfig:        "\ue615", // nf-seti-config
	kArchive:       "\uf1c6", // nf-fa-file_archive_o
	kImage:         "\uf1c5", // nf-fa-file_image_o
	kVideo:         "\uf1c8", // nf-fa-file_video_o
	kAudio:         "\uf1c7", // nf-fa-file_audio_o
	kFont:          "\uf031", // nf-fa-font
	kPDF:           "\uf1c1", // nf-fa-file_pdf_o
	kDocument:      "\uf1c2", // nf-fa-file_word_o
	kSpreadsheet:   "\uf1c3", // nf-fa-file_excel_o
	kDatabase:      "\ue706", // nf-dev-database
	kLog:           "\uf18d", // nf-fa-file_text
	kPatch:         "\uf440", // nf-oct-diff
	kDiskImage:     "\uf0a0", // nf-fa-hdd_o
	kLock:          "\uf023", // nf-fa-lock
	kGit:           "\ue702", // nf-dev-git
	kDocker:        "\ue7b0", // nf-dev-docker
	kBuildManifest: "\ue68a", // nf-custom-toml
	kText:          "\uf15c", // nf-fa-file_text_o
}

// filenameIcons maps exact (case-insensitive) filenames to a semantic key.
// Checked before extension rules, so Dockerfile never falls through to the
// default glyph.
var filenameIcons = map[string]iconKey{
	"dockerfile":         kDocker,
	"containerfile":      kDocker,
	".dockerignore":      kDocker,
	"makefile":           kBuildManifest,
	"cmakelists.txt":     kBuildManifest,
	"cargo.toml":         kBuildManifest,
	"cargo.lock":         kLock,
	"go.mod":             kBuildManifest,
	"go.sum":             kLock,
	"package.json":       kBuildManifest,
	"package-lock.json":  kLock,
	"yarn.lock":          kLock,
	"pnpm-lock.yaml":     kLock,
	"bun.lockb":          kLock,
	"composer.json":      kBuildManifest,
	"composer.lock":      kLock,
	"gemfile":            kBuildManifest,
	"gemfile.lock":       kLock,
	"pipfile":            kBuildManifest,
	"pipfile.lock":       kLock,
	"pyproject.toml":     kBuildManifest,
	"requirements.txt":   kBuildManifest,
	"poetry.lock":        kLock,
	".gitignore":         kGit,
	".gitattributes":     kGit,
	".gitmodules":        kGit,
	".editorconfig":      kConfig,
	"license":            kText,
	"licence":            kText,
	"readme":             kMarkdown,
	".env":               kConfig,
}

// extensionIcons maps lowercased extensions (with leading dot) to a key.
var extensionIcons = map[string]iconKey{
	// Markdown
	".md": kMarkdown, ".markdown": kMarkdown, ".mdx": kMarkdown,
	// Compiled code
	".go": kCodeCompiled, ".rs": kCodeCompiled, ".c": kCodeCompiled,
	".cpp": kCodeCompiled, ".cc": kCodeCompiled, ".cxx": kCodeCompiled,
	".h": kCodeCompiled, ".hpp": kCodeCompiled, ".java": kCodeCompiled,
	".swift": kCodeCompiled, ".kt": kCodeCompiled, ".kts": kCodeCompiled,
	".zig": kCodeCompiled, ".nim": kCodeCompiled, ".hs": kCodeCompiled,
	".scala": kCodeCompiled, ".ml": kCodeCompiled, ".cs": kCodeCompiled,
	".fs": kCodeCompiled, ".erl": kCodeCompiled, ".ex": kCodeCompiled,
	".exs": kCodeCompiled,
	// Scripting
	".py": kCodeScripting, ".rb": kCodeScripting, ".lua": kCodeScripting,
	".pl": kCodeScripting, ".php": kCodeScripting, ".r": kCodeScripting,
	".jl": kCodeScripting, ".clj": kCodeScripting, ".cljs": kCodeScripting,
	// Web
	".js": kCodeWeb, ".mjs": kCodeWeb, ".cjs": kCodeWeb,
	".ts": kCodeWeb, ".jsx": kCodeWeb, ".tsx": kCodeWeb,
	".html": kCodeWeb, ".htm": kCodeWeb,
	".css": kCodeWeb, ".scss": kCodeWeb, ".sass": kCodeWeb, ".less": kCodeWeb,
	".vue": kCodeWeb, ".svelte": kCodeWeb, ".elm": kCodeWeb, ".dart": kCodeWeb,
	// Shell
	".sh": kShell, ".bash": kShell, ".zsh": kShell, ".fish": kShell,
	".ksh": kShell,
	// Config / data
	".json": kConfig, ".yaml": kConfig, ".yml": kConfig, ".toml": kConfig,
	".xml": kConfig, ".ini": kConfig, ".cfg": kConfig, ".conf": kConfig,
	".env": kConfig, ".properties": kConfig,
	// Archive
	".zip": kArchive, ".tar": kArchive, ".gz": kArchive, ".bz2": kArchive,
	".xz": kArchive, ".7z": kArchive, ".rar": kArchive, ".tgz": kArchive,
	".zst": kArchive, ".lz": kArchive, ".lzma": kArchive,
	// Image
	".png": kImage, ".jpg": kImage, ".jpeg": kImage, ".gif": kImage,
	".webp": kImage, ".bmp": kImage, ".svg": kImage, ".ico": kImage,
	".tiff": kImage, ".tif": kImage, ".heic": kImage, ".avif": kImage,
	// Video
	".mp4": kVideo, ".mov": kVideo, ".mkv": kVideo, ".avi": kVideo,
	".webm": kVideo, ".flv": kVideo, ".m4v": kVideo, ".wmv": kVideo,
	// Audio
	".mp3": kAudio, ".flac": kAudio, ".wav": kAudio, ".ogg": kAudio,
	".m4a": kAudio, ".aac": kAudio, ".opus": kAudio,
	// Font
	".ttf": kFont, ".otf": kFont, ".woff": kFont, ".woff2": kFont,
	".eot": kFont,
	// PDF
	".pdf": kPDF,
	// Document
	".doc": kDocument, ".docx": kDocument, ".odt": kDocument, ".rtf": kDocument,
	".pages": kDocument,
	// Spreadsheet
	".xls": kSpreadsheet, ".xlsx": kSpreadsheet, ".ods": kSpreadsheet,
	".csv": kSpreadsheet, ".tsv": kSpreadsheet, ".numbers": kSpreadsheet,
	// Database / SQL
	".sql": kDatabase, ".db": kDatabase, ".sqlite": kDatabase, ".sqlite3": kDatabase,
	// Log
	".log": kLog,
	// Patch / diff
	".patch": kPatch, ".diff": kPatch,
	// Disk image
	".iso": kDiskImage, ".dmg": kDiskImage, ".img": kDiskImage,
	// Lock files (generic extension)
	".lock": kLock,
	// Text
	".txt": kText, ".text": kText,
	// Windows executables — Unix mode-bit check handles the rest.
	".exe": kExec, ".bin": kExec, ".out": kExec,
}

// Icon returns an icon string for the file type using the active icon style.
//
// Resolution order:
//  1. Symlink (broken or not)
//  2. Directory
//  3. Exact filename match (Dockerfile, Makefile, go.mod, .gitignore, …)
//  4. Extension match
//  5. Unix executable (mode & 0o111 != 0 on a regular file)
//  6. Default
func Icon(fi FileInfo) string {
	return iconFor(fi, currentIconStyle)
}

func iconFor(fi FileInfo, style IconStyle) string {
	key := resolveIconKey(fi)
	table := unicodeIcons
	if style == IconStyleNerd {
		table = nerdIcons
	}
	if g, ok := table[key]; ok {
		return g
	}
	return table[kDefault]
}

func resolveIconKey(fi FileInfo) iconKey {
	if fi.IsSymlink {
		if fi.SymlinkBroken {
			return kSymlinkBroken
		}
		return kSymlink
	}
	if fi.IsDir {
		return kDir
	}
	name := strings.ToLower(fi.Name)
	if k, ok := filenameIcons[name]; ok {
		return k
	}
	// README.* variants (README.md already hits extension rule; this catches
	// README, README.txt, README.rst, etc. that the exact table doesn't).
	if strings.HasPrefix(name, "readme") {
		return kMarkdown
	}
	if k, ok := extensionIcons[strings.ToLower(filepath.Ext(fi.Name))]; ok {
		return k
	}
	// Unix executable fallback: any regular file with an exec bit set.
	if fi.Mode&0o111 != 0 {
		return kExec
	}
	return kDefault
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
