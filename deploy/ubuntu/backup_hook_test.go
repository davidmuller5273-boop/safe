// Package ubuntu only exists so `go test ./...` also runs the shell tests of the
// backup hook (deploy/ubuntu/test-backup-hook.sh, uses a fake mysqldump).
package ubuntu

import (
	"os/exec"
	"testing"
)

func TestBackupHookShellSuite(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	out, err := exec.Command(bash, "test-backup-hook.sh").CombinedOutput()
	if err != nil {
		t.Fatalf("backup hook tests failed: %v\n%s", err, out)
	}
}

func TestRestoreFromBackupShellSuite(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not available")
	}
	out, err := exec.Command(bash, "test-restore-from-backup.sh").CombinedOutput()
	if err != nil {
		t.Fatalf("restore script tests failed: %v\n%s", err, out)
	}
}
