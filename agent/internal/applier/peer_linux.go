//go:build linux

package applier

import (
	"errors"
	"fmt"
	"net"
	"syscall"
)

// checkPeer accepts a connection only from root or an allowed user, by the credentials the kernel
// recorded when the peer connected to the socket.
func (s *Server) checkPeer(conn net.Conn) error {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return errors.New("not a unix connection")
	}

	raw, err := unixConn.SyscallConn()
	if err != nil {
		return err
	}

	var (
		cred    *syscall.Ucred
		credErr error
	)

	if err := raw.Control(func(fd uintptr) {
		cred, credErr = syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	}); err != nil {
		return err
	}

	if credErr != nil {
		return credErr
	}

	if cred.Uid == 0 {
		return nil
	}

	for _, uid := range s.AllowedUIDs {
		if cred.Uid == uid {
			return nil
		}
	}

	return fmt.Errorf("user %d is not allowed", cred.Uid)
}
