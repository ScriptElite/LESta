//go:build linux

// Package fsid runs a function with the Linux filesystem identity of an account.
package fsid

import (
	"fmt"
	"runtime"
	"syscall"
)

// current asks the kernel for the thread's filesystem user or group without changing it:
// setfsuid and setfsgid report the previous value, and an id of -1 never matches a real one.
// Their errors cannot be seen from the return value of the standard library wrappers, so a
// switch that did not happen would otherwise go unnoticed.
func current(syscallNumber uintptr) int {
	previous, _, _ := syscall.RawSyscall(syscallNumber, ^uintptr(0), 0, 0)

	return int(previous)
}

// RunAs runs fn with the filesystem user and group of the account: every file access inside fn
// is checked as that user, and files created are owned by it, while the process keeps its real and
// effective root identity so it can finish (clean up) afterwards. The filesystem identity is per
// thread, so fn runs on a thread locked to this goroutine. A root process that drops its
// filesystem user also drops the capabilities that bypass file permissions, so a link a tenant
// plants in their own tree cannot make a file outside their reach readable or writable.
func RunAs(uid, gid int, fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := syscall.Setfsgid(gid); err != nil {
		return fmt.Errorf("switching to the account's group: %w", err)
	}

	if err := syscall.Setfsuid(uid); err != nil {
		_ = syscall.Setfsgid(0)

		return fmt.Errorf("switching to the account's user: %w", err)
	}

	defer func() {
		_ = syscall.Setfsuid(0)
		_ = syscall.Setfsgid(0)
	}()

	if got := current(syscall.SYS_SETFSUID); got != uid {
		return fmt.Errorf("the filesystem user is %d after switching to %d", got, uid)
	}

	if got := current(syscall.SYS_SETFSGID); got != gid {
		return fmt.Errorf("the filesystem group is %d after switching to %d", got, gid)
	}

	return fn()
}
