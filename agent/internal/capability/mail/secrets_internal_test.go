package mail

import (
	"os"
	"os/user"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOnlyTheCredentialFileIsKeptFromOthers(t *testing.T) {
	dir := t.TempDir()
	c := &MailCapability{cfg: Config{}}

	files := []aggregateFile{
		{filepath.Join(dir, "domains.list"), "example.com\n", false},
		{filepath.Join(dir, "passwd"), "alice@example.com:{BLF-CRYPT}x\n", true},
	}

	if err := c.writeStaging(files); err != nil {
		t.Fatal(err)
	}

	modes := map[string]os.FileMode{}

	for _, f := range files {
		info, err := os.Stat(f.finalPath + stagingSuffix)
		if err != nil {
			t.Fatal(err)
		}

		modes[filepath.Base(f.finalPath)] = info.Mode().Perm()
	}

	if modes["domains.list"] != 0o644 {
		t.Errorf("a lookup list must stay readable, got %v", modes["domains.list"])
	}

	if modes["passwd"] != 0o640 {
		t.Errorf("the credential file must not be world-readable, got %v", modes["passwd"])
	}
}

func TestAStagingFileLeftByAnEarlierRunIsBroughtToTheRightMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "passwd")
	if err := os.WriteFile(path+stagingSuffix, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &MailCapability{cfg: Config{}}
	if err := c.writeStaging([]aggregateFile{{path, "new", true}}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path + stagingSuffix)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Errorf("expected 0640, got %v (%v)", info.Mode().Perm(), err)
	}
}

func TestTheSecretGroupIsAppliedOnlyWhenConfigured(t *testing.T) {
	current, err := user.Current()
	if err != nil {
		t.Skip("no current user")
	}

	group, err := user.LookupGroupId(current.Gid)
	if err != nil {
		t.Skip("no primary group name")
	}

	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, nil, 0o640); err != nil {
		t.Fatal(err)
	}

	if err := (&MailCapability{cfg: Config{}}).shareWithSecretGroup(path); err != nil {
		t.Errorf("no group configured must be a no-op: %v", err)
	}

	if err := (&MailCapability{cfg: Config{SecretGroup: group.Name}}).shareWithSecretGroup(path); err != nil {
		t.Errorf("a group the process belongs to must be applied: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || int(stat.Gid) != mustAtoi(t, group.Gid) {
		t.Errorf("the group was not applied: %+v", info.Sys())
	}

	if err := (&MailCapability{cfg: Config{SecretGroup: "no-such-group-lesta-test"}}).shareWithSecretGroup(path); err == nil {
		t.Errorf("an unknown group must be an error")
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()

	n := 0

	for _, r := range s {
		n = n*10 + int(r-'0')
	}

	return n
}
