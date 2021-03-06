package lock

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	defaultRetryInterval = 250 * time.Millisecond

	// Open File Description Locks
	//
	// Usually record locks held by a process are released on *any* close and are
	// not inherited across a fork().
	// These cmd values will set locks that conflict with process-associated
	// record  locks, but are "owned" by the open file description, not the
	// process. This means that they are inherited across fork() like BSD (flock)
	// locks, and they are only released automatically when the last reference to
	// the the open file against which they were acquired is put.
	//
	// source /usr/include/bits/fcntl-linux.h
	F_OFD_GETLK  = 37
	F_OFD_SETLK  = 37
	F_OFD_SETLKW = 38
)

var (
	ErrLockLocked = fmt.Errorf("lock: lock is locked")
)

// New returns a new Locker
func New(path string, retryInterval time.Duration) *Locker {
	if retryInterval == time.Duration(0) {
		retryInterval = defaultRetryInterval
	}
	return &Locker{
		path:          path,
		retryInterval: retryInterval,
	}
}

type Locker struct {
	path          string
	file          *os.File
	retryInterval time.Duration
}

// todo:
// Lock locks ...
func (l *Locker) Lock() error {
	abs, err := filepath.Abs(l.path)
	if err != nil {
		return errors.Wrap(err, "absolute represenation of path failed")
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(err, "path doesn't exist")
		}
		return errors.Wrap(err, "stat failed")
	}
	if fi.IsDir() {
		return errors.New("directory not allowed")
	}
	file, err := os.OpenFile(abs, os.O_RDWR, 0660)
	if err != nil {
		return errors.Wrap(err, "open failed")
	}
	for {
		err = unix.FcntlFlock(file.Fd(), F_OFD_SETLK, &unix.Flock_t{
			Type:   unix.F_WRLCK,
			Whence: int16(io.SeekStart),
		})
		if err == nil {
			break
		}
		if err != unix.EWOULDBLOCK {
			file.Close()
			return errors.Wrap(err, "lock failed")
		}
		time.Sleep(l.retryInterval)
	}
	l.path = abs
	l.file = file

	return nil
}

// todo:
// TryLock ...
func (l *Locker) TryLock() error {
	abs, err := filepath.Abs(l.path)
	if err != nil {
		return errors.Wrap(err, "abs failed")
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.Wrap(err, "path doesn't exist")
		}
		return errors.Wrap(err, "stat failed")
	}
	if fi.IsDir() {
		return errors.New("directories are not allowed")
	}
	file, err := os.OpenFile(abs, os.O_RDWR, 0660)
	if err != nil {
		return errors.Wrap(err, "open failed")
	}
	err = unix.FcntlFlock(file.Fd(), F_OFD_SETLK, &unix.Flock_t{
		Type:   unix.F_WRLCK,
		Whence: int16(io.SeekStart),
	})
	if err != nil {
		if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
			err = ErrLockLocked
		}
		return err
	}
	l.path = abs
	l.file = file

	return nil
}

// todo:
// Unlock ...
func (l *Locker) Unlock() error {
	// it's sufficient to simply close the file descriptor
	if err := l.file.Close(); err != nil {
		return errors.Wrap(err, "close failed")
	}
	return nil
}
