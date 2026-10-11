package identity_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mikho/LESta/agent/internal/capability/identity"
)

// fakeUserTool writes a stand-in for useradd/userdel that records its arguments.
func fakeUserTool(t *testing.T, dir, name string) (path, log string) {
	t.Helper()

	log = filepath.Join(dir, name+".log")
	path = filepath.Join(dir, name)

	if err := os.WriteFile(path, []byte("#!/bin/sh\necho \"$@\" >> "+log+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	return path, log
}

func TestRootUserModesAcceptOnlyPerAccountNames(t *testing.T) {
	dir := t.TempDir()
	useradd, addLog := fakeUserTool(t, dir, "useradd")
	userdel, delLog := fakeUserTool(t, dir, "userdel")
	cfg := identity.Config{UseraddBinary: useradd, UserdelBinary: userdel}

	for _, name := range []string{"root", "lesta-agent", "www-data", "lesta-t", "lesta-t1x", "lesta-t1 -o -u 0", "../lesta-t1", "", "lesta-t1234567890123"} {
		if code := identity.SystemUserAdd(cfg, name); code == 0 {
			t.Errorf("useradd mode accepted %q", name)
		}

		if code := identity.SystemUserDel(cfg, name); code == 0 {
			t.Errorf("userdel mode accepted %q", name)
		}
	}

	for _, log := range []string{addLog, delLog} {
		if _, err := os.Stat(log); err == nil {
			t.Errorf("a refused name must never reach the real tool (%s was written)", log)
		}
	}

	if code := identity.SystemUserAdd(cfg, "lesta-t42"); code != 0 {
		t.Fatalf("useradd mode refused a per-account name: %d", code)
	}

	if code := identity.SystemUserDel(cfg, "lesta-t42"); code != 0 {
		t.Fatalf("userdel mode refused a per-account name: %d", code)
	}

	added, _ := os.ReadFile(addLog)
	if strings.TrimSpace(string(added)) != "--system --no-create-home --shell /usr/sbin/nologin --home-dir /public lesta-t42" {
		t.Errorf("useradd was run with unexpected arguments: %q", added)
	}

	deleted, _ := os.ReadFile(delLog)
	if strings.TrimSpace(string(deleted)) != "lesta-t42" {
		t.Errorf("userdel was run with unexpected arguments: %q", deleted)
	}
}
