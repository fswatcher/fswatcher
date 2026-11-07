//go:build freebsd || openbsd || netbsd

package fswatcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// mkfifo creates a FIFO at path. Its cleanup opens the FIFO for writing
// without blocking so that a watcher stuck in a blocking open of it is
// released and the test fails instead of hanging in Close. Call it after
// newWatcher so this cleanup runs before the watcher's Close.
func mkfifo(t *testing.T, path string) {
	t.Helper()
	if err := unix.Mkfifo(path, 0o644); err != nil {
		t.Fatalf("Mkfifo: %v", err)
	}
	t.Cleanup(func() {
		if fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK, 0); err == nil {
			unix.Close(fd)
		}
	})
}

func TestKqueueFIFOChildDoesNotBlock(t *testing.T) {
	dir := tempDir(t)
	w := newWatcher(t)

	// A FIFO that exists when Add runs is opened while populating children.
	mkfifo(t, filepath.Join(dir, "existing-fifo"))
	addErr := make(chan error, 1)
	go func() { addErr <- w.Add(dir, Create) }()
	select {
	case err := <-addErr:
		if err != nil {
			t.Fatalf("Add: %v", err)
		}
	case <-time.After(eventTimeout):
		t.Fatalf("Add blocked on a FIFO in the watched directory")
	}

	// A FIFO created after Add is opened by the read loop; later entries
	// must still be reported.
	mkfifo(t, filepath.Join(dir, "new-fifo"))
	target := filepath.Join(dir, "after.txt")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	deadline := time.NewTimer(eventTimeout)
	defer deadline.Stop()
	for {
		select {
		case ev, ok := <-w.Events:
			if !ok {
				t.Fatalf("Events channel closed early")
			}
			if ev.Name == target && ev.Op.Has(Create) {
				return
			}
		case _, ok := <-w.Errors:
			// Attaching a vnode watch to a FIFO may be rejected by the
			// kernel; that is reported but must not stop the watcher.
			if !ok {
				t.Fatalf("Errors channel closed early")
			}
		case <-deadline.C:
			t.Fatalf("timeout waiting for Create of %s; watcher blocked on a FIFO", target)
		}
	}
}
