//go:build !linux

package applier

import (
	"errors"
	"net"
)

// checkPeer has no peer credentials off Linux (the applier only runs on Linux nodes): it refuses
// everything, so a build for another platform cannot serve by accident. Tests use TrustAnyPeer.
func (s *Server) checkPeer(_ net.Conn) error {
	return errors.New("peer credentials are not available on this platform")
}
