//go:build openbsd

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
	events    chan<- Event
	errors    chan<- error
	kq        int    // kqueue fd
	closePipe [2]int // pipe for signaling Close on platforms without EVFILT_USER
	done      chan struct{}
	exited    chan struct{}

	// mu protects the fields below.
	mu     sync.Mutex
	roots  map[string]*kqWatch
	byFd   map[int]*kqWatch
	closed bool
}

// NewWatcher returns a Watcher backed by kqueue.
func NewWatcher() (*Watcher, error) {
	kq, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}

	// Create a pipe for signaling Close
	var closePipe [2]int
	if err := unix.Pipe(closePipe[:]); err != nil {
		unix.Close(kq)
		return nil, err
	}

	// Register the read end of the pipe with kqueue
	var ev unix.Kevent_t
	unix.SetKevent(&ev, closePipe[0], unix.EVFILT_READ, unix.EV_ADD|unix.EV_CLEAR)
	if _, err := ignoringEINTR2(func() (int, error) {
		return unix.Kevent(kq, []unix.Kevent_t{ev}, nil, nil)
	}); err != nil {
		unix.Close(closePipe[0])
		unix.Close(closePipe[1])
		unix.Close(kq)
		return nil, err
	}

	events := make(chan Event, 64)
	errors := make(chan error, 8)
	w := &Watcher{
		Events:    events,
		Errors:    errors,
		events:    events,
		errors:    errors,
		kq:        kq,
		closePipe: closePipe,
		done:      make(chan struct{}),
		exited:    make(chan struct{}),
		roots:     make(map[string]*kqWatch),
		byFd:      make(map[int]*kqWatch),
	}
	go w.readLoop()
	return w, nil
}

// handleEvents processes a batch of kqueue events.
// It returns true if the read loop should exit (e.g. on Close signal), false otherwise.
func (w *Watcher) handleEvents(events []unix.Kevent_t) bool {
	for i := range events {
		if int(events[i].Ident) == w.closePipe[0] {
			// Close signal via pipe
			return true
		}
		w.handleEvent(&events[i])
	}
	return false
}

func (w *Watcher) stopReadLoop() {
	// Signal readLoop by writing to the pipe
	ignoringEINTR2(func() (int, error) {
		return unix.Write(w.closePipe[1], []byte{0})
	})
	unix.Close(w.closePipe[1])
}

func (w *Watcher) closeKq() error {
	unix.Close(w.closePipe[0])
	return suppressEINTR(func() error {
		return unix.Close(w.kq)
	})
}
