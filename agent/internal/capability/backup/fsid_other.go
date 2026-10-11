//go:build !linux

package backup

import "github.com/mikho/LESta/agent/internal/fsid"

// runAsAccount has no filesystem-identity switch off Linux (see fsid.RunAs).
func runAsAccount(uid, gid int, fn func() error) error {
	return fsid.RunAs(uid, gid, fn)
}
