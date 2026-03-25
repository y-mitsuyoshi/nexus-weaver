package fs

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed in %s: %v\noutput: %s", args, dir, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func TestAutoCommit_SkipWhenNoChanges(t *testing.T) {
	// Prepare a temp repo with an initial commit
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	// create initial file and commit
	if err := os.WriteFile(dir+"/a.txt", []byte("initial"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-m", "init")

	oldHead := runGit(t, dir, "rev-parse", "HEAD")

	// Change working directory to the repo so AutoCommit's git calls operate there
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	base, err := AutoCommit("pre-test: skip-no-change")
	if err != nil {
		t.Fatalf("AutoCommit returned error: %v", err)
	}
	if base != oldHead {
		t.Fatalf("expected baseHash %s, got %s", oldHead, base)
	}

	// Ensure HEAD unchanged
	headNow := runGit(t, dir, "rev-parse", "HEAD")
	if headNow != oldHead {
		t.Fatalf("expected HEAD to remain %s, got %s", oldHead, headNow)
	}
}

func TestAutoCommit_CreatesCommitAndRollback(t *testing.T) {
	// Prepare a temp repo with an initial commit and then modify file
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")
	// initial commit
	if err := os.WriteFile(dir+"/b.txt", []byte("v1"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	runGit(t, dir, "add", "b.txt")
	runGit(t, dir, "commit", "-m", "init")

	oldHead := runGit(t, dir, "rev-parse", "HEAD")

	// modify the file (unstaged)
	if err := os.WriteFile(dir+"/b.txt", []byte("v2"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	// chdir into repo
	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	base, err := AutoCommit("pre-test: create-commit")
	if err != nil {
		t.Fatalf("AutoCommit returned error: %v", err)
	}
	if base != oldHead {
		t.Fatalf("expected baseHash %s, got %s", oldHead, base)
	}

	newHead := runGit(t, dir, "rev-parse", "HEAD")
	if newHead == base {
		t.Fatalf("expected a new commit, but HEAD did not change (HEAD=%s)", newHead)
	}

	// Rollback to base
	if err := Rollback(base); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	headAfter := runGit(t, dir, "rev-parse", "HEAD")
	if headAfter != base {
		t.Fatalf("expected HEAD to be %s after rollback, got %s", base, headAfter)
	}

	// ensure file content restored to v1
	data, err := os.ReadFile(dir + "/b.txt")
	if err != nil {
		t.Fatalf("read file failed: %v", err)
	}
	if string(data) != "v1" {
		t.Fatalf("expected file content v1 after rollback, got %s", string(data))
	}
}

func TestAutoCommit_InitialCommitCreatesHead(t *testing.T) {
	// Prepare repo with no commits
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "test")

	if err := os.WriteFile(dir+"/c.txt", []byte("content"), 0644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	cwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(cwd) }()

	base, err := AutoCommit("pre-test: initial-commit")
	if err != nil {
		t.Fatalf("AutoCommit returned error: %v", err)
	}
	if base != "" {
		t.Fatalf("expected baseHash to be empty for repos without initial commit, got %s", base)
	}

	// HEAD should now exist
	head := runGit(t, dir, "rev-parse", "HEAD")
	if head == "" {
		t.Fatalf("expected HEAD to be present after initial commit")
	}

	// Rollback with empty should error
	err = Rollback("")
	if err == nil {
		t.Fatalf("expected Rollback(\"\") to return error")
	}
}
