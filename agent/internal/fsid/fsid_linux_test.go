//go:build linux

package fsid_test

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/mikho/LESta/agent/internal/fsid"
)

func TestRunAsRunsTheFunctionAndReportsItsError(t *testing.T) {
	uid, gid := os.Geteuid(), os.Getegid()
	ran := false

	// Switching to the identity the process already has is allowed for any user.
	if err := fsid.RunAs(uid, gid, func() error { ran = true; return nil }); err != nil || !ran {
		t.Fatalf("RunAs(%d, %d) = %v, ran = %v", uid, gid, err, ran)
	}

	want := os.ErrInvalid
	if err := fsid.RunAs(uid, gid, func() error { return want }); err != want {
		t.Errorf("the function's error must be returned unchanged, got %v", err)
	}
}

func TestRunAsRefusesAnIdentityItCannotTake(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can take any identity")
	}

	if err := fsid.RunAs(os.Geteuid()+1, os.Getegid(), func() error { t.Error("fn must not run under an identity that was not taken"); return nil }); err == nil {
		t.Error("an unprivileged process must not be able to switch to another user, and RunAs must say so")
	}
}

func TestRunAsDropsRootsAccessToFilesTheAccountCannotTouch(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("needs root to switch to another account")
	}

	const account = 65534

	// Not t.TempDir(): its parent is 0700, which the account could not enter.
	dir, err := os.MkdirTemp("", "fsid")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}

	private := filepath.Join(dir, "root-only")
	if err := os.WriteFile(private, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(private, link); err != nil {
		t.Fatal(err)
	}

	err = fsid.RunAs(account, account, func() error {
		// What a swapped-in link would lead root to: reading or overwriting a file the account
		// has no right to. Both must now fail.
		if _, readErr := os.ReadFile(link); !errors.Is(readErr, os.ErrPermission) {
			t.Errorf("reading through the link must be denied, got %v", readErr)
		}

		if writeErr := os.WriteFile(link, []byte("overwritten"), 0o644); !errors.Is(writeErr, os.ErrPermission) {
			t.Errorf("writing through the link must be denied, got %v", writeErr)
		}

		// A file the account creates is owned by the account, with no chown needed.
		return os.WriteFile(filepath.Join(dir, "mine"), []byte("x"), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}

	if content, _ := os.ReadFile(private); string(content) != "secret" {
		t.Errorf("the protected file was changed: %q", content)
	}

	info, err := os.Stat(filepath.Join(dir, "mine"))
	if err != nil {
		t.Fatal(err)
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Uid) != account || int(stat.Gid) != account {
		t.Errorf("a file created as the account must be owned by it, got %+v", info.Sys())
	}
}
