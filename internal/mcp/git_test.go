package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCloneURL(t *testing.T) {
	if err := validateCloneURL("https://github.com/org/repo.git"); err != nil {
		if strings.Contains(err.Error(), "could not be resolved") {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	if err := validateCloneURL("http://github.com/org/repo.git"); err == nil {
		t.Fatal("http should be rejected")
	}
	if err := validateCloneURL("https://127.0.0.1/repo.git"); err == nil {
		t.Fatal("loopback should be rejected")
	}
	if err := validateCloneURL("git@github.com:org/repo.git"); err == nil {
		t.Fatal("ssh should be rejected")
	}
	if err := validateCloneURL("file:///tmp/repo"); err == nil {
		t.Fatal("file urls should be rejected")
	}
	if err := validateCloneURL("https://github.com/../etc"); err == nil {
		t.Fatal("traversal should be rejected")
	}
}

func TestSafeGitRef(t *testing.T) {
	if !safeGitRef("main") || !safeGitRef("HEAD~1") || !safeGitRef("feature/fix") {
		t.Fatal("expected valid refs")
	}
	if safeGitRef("-evil") || safeGitRef("a;rm") {
		t.Fatal("expected invalid refs")
	}
}

func TestBlameAndLogOnLocalRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	files := NewFilesConnector(root)
	g := NewGit(files)
	dest, err := g.repoDir("proj1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dest
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Azula", "GIT_AUTHOR_EMAIL=azula@test", "GIT_COMMITTER_NAME=Azula", "GIT_COMMITTER_EMAIL=azula@test")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
	run("init")
	run("config", "user.email", "azula@test")
	run("config", "user.name", "Azula")
	if err := os.WriteFile(filepath.Join(dest, "pipeline.py"), []byte("print('ok')\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	run("add", "pipeline.py")
	run("commit", "-m", "seed")
	if err := g.writeMeta(dest, gitMeta{URL: "https://example.com/repo.git", Branch: "main", Head: "HEAD"}); err != nil {
		t.Fatal(err)
	}

	blame, err := g.Blame(context.Background(), "proj1", "pipeline.py")
	if err != nil {
		t.Fatal(err)
	}
	if len(blame) == 0 {
		t.Fatal("expected blame lines")
	}
	log, err := g.Log(context.Background(), "proj1", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(log) == 0 || !strings.Contains(log[0].Message, "seed") {
		t.Fatalf("unexpected log: %+v", log)
	}
}

func TestImportNestedAllowedFiles(t *testing.T) {
	root := t.TempDir()
	files := NewFilesConnector(root)
	g := NewGit(files)
	dest, err := g.repoDir("proj2")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dest, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "src", "pipeline.py"), []byte("print(1)\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	imported, err := g.importAllowed(context.Background(), "proj2", dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 1 || imported[0].Name != "src__pipeline.py" {
		t.Fatalf("imported: %+v", imported)
	}
	body, err := files.ReadFile(context.Background(), "proj2", "src__pipeline.py")
	if err != nil || !strings.Contains(body, "print") {
		t.Fatalf("body %q err %v", body, err)
	}
}
