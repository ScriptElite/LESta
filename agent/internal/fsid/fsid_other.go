//go:build !linux

// Package fsid runs a function with the Linux filesystem identity of an account.
package fsid

// RunAs has no filesystem-identity switch off Linux (the agent only runs on Linux nodes); it
// exists so the package builds and tests on a developer machine.
func RunAs(_, _ int, fn func() error) error {
	return fn()
}
