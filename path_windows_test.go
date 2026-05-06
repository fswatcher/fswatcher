//go:build windows

package fsnotify

import (
	"strings"
	"syscall"
	"testing"
)

func TestCanonicalizeExpandsShortPath(t *testing.T) {
	// t.TempDir() may already mix 8.3 and long components on CI runners,
	// so fully canonicalize first to get the expected long form.
	long, err := canonicalize(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize(TempDir): %v", err)
	}

	longPtr, err := syscall.UTF16PtrFromString(long)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	var buf [syscall.MAX_PATH]uint16
	n, err := syscall.GetShortPathName(longPtr, &buf[0], uint32(len(buf)))
	if err != nil || n == 0 {
		t.Skipf("GetShortPathName unavailable: %v", err)
	}
	short := syscall.UTF16ToString(buf[:n])
	if strings.EqualFold(short, long) {
		t.Skip("temp dir has no distinct 8.3 short form")
	}

	got, err := canonicalize(short)
	if err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	if !strings.EqualFold(got, long) {
		t.Errorf("canonicalize(%q) = %q, want %q", short, got, long)
	}
}
