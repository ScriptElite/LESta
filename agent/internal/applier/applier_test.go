package applier_test

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mikho/LESta/agent/internal/applier"
	"github.com/mikho/LESta/agent/internal/capability/fake"
	"github.com/mikho/LESta/agent/internal/contract"
	"github.com/mikho/LESta/agent/internal/protocol"
)

// serve starts a server on a socket in a temp directory and returns a client for it.
func serve(t *testing.T, server *applier.Server) (socket string, remote applier.Remote) {
	t.Helper()

	// Unix socket paths are short; a long t.TempDir() can exceed the limit.
	dir, err := os.MkdirTemp("", "app")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socket = filepath.Join(dir, "a.sock")

	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = server.Serve(ctx, l)
	}()

	t.Cleanup(func() {
		cancel()
		<-done
	})

	return socket, applier.Remote{SocketPath: socket}
}

func operation(capability string) protocol.OperationEnvelope {
	return protocol.OperationEnvelope{ProtocolVersion: "1", Capability: capability, Operation: protocol.OperationObserve, ResourceID: "00000000-0000-0000-0000-000000000001", IdempotencyKey: "k"}
}

func echoHandler(_ context.Context, op protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
	return protocol.ResultEnvelope{ProtocolVersion: op.ProtocolVersion, Capability: op.Capability, ResourceID: op.ResourceID, Status: protocol.StatusApplied}, nil
}

func TestAFakeCapabilityBehindTheApplierSatisfiesTheSharedContract(t *testing.T) {
	contract.RunAgainst(t, func(t *testing.T) protocol.Capability {
		backing := fake.New()
		_, remote := serve(t, &applier.Server{
			Handler: func(ctx context.Context, op protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
				return backing.Apply(ctx, op)
			},
			Hosted:       map[string]bool{"web.nginx.v1": true},
			TrustAnyPeer: true,
		})

		return remote
	})
}

func TestARoundTripReturnsTheHandlersResult(t *testing.T) {
	_, remote := serve(t, &applier.Server{Handler: echoHandler, Hosted: map[string]bool{"dns.bind9.v1": true}, TrustAnyPeer: true})

	result, err := remote.Apply(context.Background(), operation("dns.bind9.v1"))
	if err != nil || result.Status != protocol.StatusApplied || result.Capability != "dns.bind9.v1" {
		t.Fatalf("Apply = %+v, %v", result, err)
	}
}

func TestACapabilityTheApplierDoesNotHostIsRefusedBeforeTheHandler(t *testing.T) {
	called := false

	_, remote := serve(t, &applier.Server{
		Handler: func(context.Context, protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
			called = true
			return protocol.ResultEnvelope{}, nil
		},
		Hosted:       map[string]bool{"dns.bind9.v1": true},
		TrustAnyPeer: true,
	})

	_, err := remote.Apply(context.Background(), operation("web.nginx.v1"))
	if err == nil || !strings.Contains(err.Error(), "not applied by the applier") || called {
		t.Fatalf("expected a refusal without calling the handler, got %v (called=%v)", err, called)
	}
}

func TestAHandlerErrorAndAPanicBecomeErrorsAndTheServerKeepsServing(t *testing.T) {
	calls := 0

	_, remote := serve(t, &applier.Server{
		Handler: func(ctx context.Context, op protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
			calls++

			switch calls {
			case 1:
				return protocol.ResultEnvelope{}, errors.New("no verdict")
			case 2:
				panic("boom")
			}

			return echoHandler(ctx, op)
		},
		Hosted:       map[string]bool{"dns.bind9.v1": true},
		TrustAnyPeer: true,
	})

	if _, err := remote.Apply(context.Background(), operation("dns.bind9.v1")); err == nil || err.Error() != "no verdict" {
		t.Errorf("a handler error must reach the caller unchanged, got %v", err)
	}

	if _, err := remote.Apply(context.Background(), operation("dns.bind9.v1")); err == nil || !strings.Contains(err.Error(), "panicked") {
		t.Errorf("a panic must become an error, got %v", err)
	}

	if _, err := remote.Apply(context.Background(), operation("dns.bind9.v1")); err != nil {
		t.Errorf("the server must keep serving after a panic: %v", err)
	}
}

func TestAnOversizedOrMalformedRequestIsRefusedWithoutReachingTheHandler(t *testing.T) {
	called := false

	socket, _ := serve(t, &applier.Server{
		Handler: func(context.Context, protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
			called = true
			return protocol.ResultEnvelope{}, nil
		},
		Hosted:       map[string]bool{"dns.bind9.v1": true},
		TrustAnyPeer: true,
	})

	send := func(raw []byte) {
		conn, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}

		defer func() { _ = conn.Close() }()

		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = conn.Write(raw)

		buf := make([]byte, 4096)
		_, _ = conn.Read(buf)
	}

	huge := make([]byte, 4)
	binary.BigEndian.PutUint32(huge, applier.MaxMessageBytes+1)
	send(huge)

	zero := make([]byte, 4)
	send(zero)

	notJSON := append([]byte{0, 0, 0, 5}, []byte("hello")...)
	send(notJSON)

	unknownField := append([]byte{0, 0, 0, 14}, []byte(`{"extra":true}`)...)
	send(unknownField)

	if called {
		t.Errorf("a refused request must never reach the handler")
	}
}

func TestACancelledContextStopsAWaitingCall(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	_, remote := serve(t, &applier.Server{
		Handler: func(ctx context.Context, op protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
			select {
			case <-release:
			case <-ctx.Done():
			}

			return echoHandler(ctx, op)
		},
		Hosted:       map[string]bool{"dns.bind9.v1": true},
		TrustAnyPeer: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()

	if _, err := remote.Apply(ctx, operation("dns.bind9.v1")); err == nil {
		t.Fatalf("expected the call to end with the context")
	}

	if time.Since(start) > 3*time.Second {
		t.Errorf("the call did not stop when its context ended")
	}
}

func TestAnUnreachableApplierIsAnErrorNotAVerdict(t *testing.T) {
	_, err := applier.Remote{SocketPath: filepath.Join(t.TempDir(), "missing.sock")}.Apply(context.Background(), operation("dns.bind9.v1"))
	if err == nil || !strings.Contains(err.Error(), "could not be reached") {
		t.Fatalf("expected an unreachable error, got %v", err)
	}
}

func TestOnlyRootAndTheAllowedUsersAreServed(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("peer credentials are checked on Linux")
	}

	if os.Geteuid() == 0 {
		t.Skip("root is always served, so a refusal cannot be shown from a root test")
	}

	handled := false
	handler := func(ctx context.Context, op protocol.OperationEnvelope) (protocol.ResultEnvelope, error) {
		handled = true

		return echoHandler(ctx, op)
	}

	_, notAllowed := serve(t, &applier.Server{Handler: handler, Hosted: map[string]bool{"dns.bind9.v1": true}, AllowedUIDs: []uint32{uint32(os.Geteuid()) + 1}})
	if _, err := notAllowed.Apply(context.Background(), operation("dns.bind9.v1")); err == nil || handled {
		t.Errorf("a user that is not allowed must be refused (err=%v, handled=%v)", err, handled)
	}

	_, allowed := serve(t, &applier.Server{Handler: handler, Hosted: map[string]bool{"dns.bind9.v1": true}, AllowedUIDs: []uint32{uint32(os.Geteuid())}})
	if _, err := allowed.Apply(context.Background(), operation("dns.bind9.v1")); err != nil {
		t.Errorf("an allowed user must be served: %v", err)
	}
}
