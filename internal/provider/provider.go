package provider

import (
	"io"

	"github.com/mogglemoss/pelorus/pkg/fileinfo"
)

// Caps describes the capabilities of a provider.
type Caps struct {
	CanSetPermissions bool
	CanSymlink        bool
	CanPreview        bool
	CanTrash          bool
	IsRemote          bool
	SupportsArchive   bool
	// RemoteLabel is non-empty for remote providers; contains "user@hostname".
	// Used by the pane to prefix the path display.
	RemoteLabel string
}

// Provider is the interface all filesystem backends must implement.
type Provider interface {
	List(path string) ([]fileinfo.FileInfo, error)
	Stat(path string) (fileinfo.FileInfo, error)
	Read(path string) (io.ReadCloser, error)
	Copy(src, dst string) error
	Move(src, dst string) error
	Delete(path string) error
	MakeDir(path string) error
	Rename(src, dst string) error
	Capabilities() Caps
	String() string
}
