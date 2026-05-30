//go:build freebsd

package fswatcher

import (
	"sync"

	"golang.org/x/sys/unix"
)

// Watcher monitors registered paths via kqueue. Directories are watched
// non-recursively: child entries are tracked so Create and Remove fire
// for files inside the directory, matching the Linux/Windows backends.
type Watcher struct {
	// Events delivers change notifications. Closed when Close returns.
	Events <-chan Event
	// Errors delivers non-fatal errors from the read loop. Closed when Close returns.
	Errors <-chan error

	// Immutable after initialization.
	events chan<- Event
	errors chan<- error
	kq     int // kqueue fd
	done   chan struct{}
	exited chan struct{}

	// mu protects the fields below.
	mu     sync.Mutex
	roots  map[string]*kqWatch
	byFd   map[int]*kqWatch
	closed bool
}

const closeEvId = 9999

var evCloseRegister = unix.Kevent_t{
	Ident:  closeEvId,
	Filter: unix.EVFILT_USER,
	Flags:  unix.EV_ADD | unix.EV_CLEAR,
}

var evCloseTrigger = unix.Kevent_t{
	Ident:  closeEvId,
	Filter: unix.EVFILT_USER,
	Fflags: unix.NOTE_TRIGGER,
}

// NewWatcher returns a Watcher backed by kqueue.
func NewWatcher() (*Watcher, error) {
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	_, err = unix.Kevent(kq, []unix.Kevent_t{evCloseRegister}, nil, nil)
	if err != nil {
		unix.Close(kq)
		return nil, err
	}

	events := make(chan Event, 64)
	errors := make(chan error, 8)
	w := &Watcher{
		Events: events,
		Errors: errors,
		events: events,
		errors: errors,
		kq:     kq,
		done:   make(chan struct{}),
		exited: make(chan struct{}),
		roots:  make(map[string]*kqWatch),
		byFd:   make(map[int]*kqWatch),
	}
	go w.readLoop()
	return w, nil
}

// handleEvents processes a batch of kqueue events.
// It returns true if the read loop should exit (e.g. on Close signal), false otherwise.
func (w *Watcher) handleEvents(events []unix.Kevent_t) bool {
	for i := range events {
		w.handleEvent(&events[i])
	}
	return false
}

func (w *Watcher) stopReadLoop() error {
	_, err := unix.Kevent(w.kq, []unix.Kevent_t{evCloseTrigger}, nil, nil)
	return err
}

func (w *Watcher) closeKq() error {
	return unix.Close(w.kq)
}
