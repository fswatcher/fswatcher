// Package fsnotify provides cross-platform file system change notifications.
package fsnotify

import (
	"errors"
	"fmt"
	"strings"
)

// Op describes a set of file system event types as a bitmask.
type Op uint32

const (
	// Create indicates a file or directory was created.
	Create Op = 1 << iota
	// Write indicates a file's contents were modified.
	Write
	// Remove indicates a file or directory was removed.
	Remove
	// Rename indicates a file or directory was renamed or moved.
	Rename
	// Chmod indicates permissions or attributes changed.
	Chmod
)

// All is the union of every supported Op bit.
const All = Create | Write | Remove | Rename | Chmod

// Has reports whether op contains any bit set in target.
func (op Op) Has(target Op) bool {
	return op&target != 0
}

// String returns a human-readable representation such as "CREATE|WRITE".
func (op Op) String() string {
	if op == 0 {
		return ""
	}
	var parts []string
	if op.Has(Create) {
		parts = append(parts, "CREATE")
	}
	if op.Has(Write) {
		parts = append(parts, "WRITE")
	}
	if op.Has(Remove) {
		parts = append(parts, "REMOVE")
	}
	if op.Has(Rename) {
		parts = append(parts, "RENAME")
	}
	if op.Has(Chmod) {
		parts = append(parts, "CHMOD")
	}
	return strings.Join(parts, "|")
}

// Event represents a single file system change.
type Event struct {
	// Name is the absolute or watcher-relative path of the affected entry.
	Name string
	// Op is the set of changes that occurred. A single notification may
	// carry more than one bit when the underlying OS coalesces events.
	Op Op
}

// String returns a human-readable representation such as "CREATE: /tmp/x".
func (e Event) String() string {
	return fmt.Sprintf("%s: %q", e.Op, e.Name)
}

// Sentinel errors returned by Watcher methods.
var (
	// ErrAlreadyAdded is returned by Add when path is already registered.
	ErrAlreadyAdded = errors.New("fsnotify: path already added")
	// ErrNotAdded is returned by Remove when path is not registered.
	ErrNotAdded = errors.New("fsnotify: path not added")
	// ErrClosed is returned by methods called on a closed Watcher.
	ErrClosed = errors.New("fsnotify: watcher closed")
	// ErrUnsupported is returned by NewWatcher on platforms without a backend.
	ErrUnsupported = errors.New("fsnotify: platform not supported")
)
