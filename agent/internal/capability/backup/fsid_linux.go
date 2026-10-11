//go:build linux

package backup

import "github.com/mikho/LESta/agent/internal/fsid"

// runAsAccount runs fn with the filesystem identity of the account (see fsid.RunAs).
func runAsAccount(uid, gid int, fn func() error) error {
	return fsid.RunAs(uid, gid, fn)
}
