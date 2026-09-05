package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
)

type gitMeta struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Head   string `json:"head"`
}

type Git struct {
	files *FilesConnector
}

func NewGit(files *FilesConnector) *Git {
	return &Git{files: files}
}

func (g *Git) repoDir(projectID string) (string, error) {
	dir, err := g.files.projectDir(projectID)
	if err != nil {
		return "", err
	}
	dest := filepath.Join(dir, ".repo")
	if err := withinRoot(dir, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (g *Git) Clone(ctx context.Context, projectID, repoURL, branch string) (*domain.GitRepo, []domain.ProjectFile, error) {
	if err := validateCloneURL(repoURL); err != nil {
		return nil, nil, err
	}
	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "main"
	}
	if !safeGitRef(branch) {
		return nil, nil, domain.ErrInvalidInput
	}
	dest, err := g.repoDir(projectID)
	if err != nil {
		return nil, nil, err
	}
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return nil, nil, err
	}
	_ = os.RemoveAll(dest)

	args := []string{"clone", "--depth", "1", "--branch", branch, "--", repoURL, dest}
	if err := runGit(ctx, parent, args...); err != nil {
		return nil, nil, err
	}
	head, _ := gitOutput(ctx, dest, "rev-parse", "HEAD")
	head = strings.TrimSpace(head)
	meta := gitMeta{URL: repoURL, Branch: branch, Head: head}
	if err := g.writeMeta(dest, meta); err != nil {
		return nil, nil, err
	}
	imported, err := g.importAllowed(ctx, projectID, dest)
	if err != nil {
		return nil, nil, err
	}
	return &domain.GitRepo{URL: repoURL, Branch: branch, Head: head, Connected: true}, imported, nil
}

func (g *Git) Status(_ context.Context, projectID string) (*domain.GitRepo, error) {
	dest, err := g.repoDir(projectID)
	if err != nil {
		return nil, err
	}
	meta, err := g.readMeta(dest)
	if err != nil {
		return &domain.GitRepo{Connected: false}, nil
	}
	return &domain.GitRepo{URL: meta.URL, Branch: meta.Branch, Head: meta.Head, Connected: true}, nil
}

func (g *Git) Blame(ctx context.Context, projectID, path string) ([]domain.GitBlameLine, error) {
	dest, err := g.requireRepo(projectID)
	if err != nil {
		return nil, err
	}
	rel, err := safeRepoPath(path)
	if err != nil {
		return nil, err
	}
	out, err := gitOutput(ctx, dest, "blame", "-l", "--", rel)
	if err != nil {
		return nil, err
	}
	return parseBlame(out), nil
}

func (g *Git) Diff(ctx context.Context, projectID, refA, refB string) (string, error) {
	dest, err := g.requireRepo(projectID)
	if err != nil {
		return "", err
	}
	if refA == "" {
		refA = "HEAD~1"
	}
	if refB == "" {
		refB = "HEAD"
	}
	if !safeGitRef(refA) || !safeGitRef(refB) {
		return "", domain.ErrInvalidInput
	}
	out, err := gitOutput(ctx, dest, "diff", refA, refB)
	if err != nil {
		return "", err
	}
	return out, nil
}

func (g *Git) Log(ctx context.Context, projectID string, n int) ([]domain.GitCommit, error) {
	dest, err := g.requireRepo(projectID)
	if err != nil {
		return nil, err
	}
	if n <= 0 || n > 50 {
		n = 20
	}
	out, err := gitOutput(ctx, dest, "log", "-n", strconv.Itoa(n), "--format=%H%x09%an%x09%ad%x09%s", "--date=iso-strict")
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

func (g *Git) requireRepo(projectID string) (string, error) {
	dest, err := g.repoDir(projectID)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		if _, err2 := os.Stat(dest); err2 != nil {
			return "", domain.ErrGitNotConnected
		}
	}
	return dest, nil
}

func (g *Git) writeMeta(dest string, meta gitMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dest, ".azula-git.json"), b, 0o640)
}

func (g *Git) readMeta(dest string) (gitMeta, error) {
	var meta gitMeta
	b, err := os.ReadFile(filepath.Join(dest, ".azula-git.json"))
	if err != nil {
		return meta, err
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func (g *Git) importAllowed(ctx context.Context, projectID, dest string) ([]domain.ProjectFile, error) {
	var imported []domain.ProjectFile
	err := filepath.WalkDir(dest, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			base := d.Name()
			if base == ".git" || strings.EqualFold(base, "node_modules") || strings.EqualFold(base, "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if len(imported) >= 40 {
			return nil
		}
		rel, err := filepath.Rel(dest, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, ".git/") || strings.HasSuffix(rel, ".azula-git.json") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(rel))
		if _, ok := allowedExt[ext]; !ok {
			return nil
		}
		flat := flattenRepoName(rel)
		if err := validateName(flat); err != nil {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		saved, err := g.files.SaveUpload(ctx, projectID, flat, mimeFor(ext), f)
		_ = f.Close()
		if err != nil {
			return err
		}
		imported = append(imported, saved)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return imported, nil
}

func flattenRepoName(rel string) string {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "./")
	rel = strings.ReplaceAll(rel, "/", "__")
	return rel
}

func safeGitRef(ref string) bool {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return false
	}
	for _, r := range ref {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' || r == '/' || r == '~' {
			continue
		}
		return false
	}
	return true
}

func safeRepoPath(name string) (string, error) {
	name = filepath.ToSlash(strings.TrimSpace(name))
	if name == "" || strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", domain.ErrPathTraversal
	}
	parts := strings.Split(name, "/")
	if len(parts) == 0 || len(parts) > 12 {
		return "", domain.ErrPathTraversal
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".git" {
			return "", domain.ErrPathTraversal
		}
		if strings.ContainsAny(part, `\:`) {
			return "", domain.ErrPathTraversal
		}
	}
	ext := strings.ToLower(filepath.Ext(parts[len(parts)-1]))
	if _, ok := allowedExt[ext]; !ok {
		return "", domain.ErrForbiddenFile
	}
	return name, nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	_, err := gitOutput(ctx, dir, args...)
	return err
}

func gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", args[0], msg)
	}
	return string(out), nil
}

func parseBlame(out string) []domain.GitBlameLine {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	var result []domain.GitBlameLine
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		sha, rest, _ := strings.Cut(line, " ")
		author := ""
		summary := rest
		if idx := strings.Index(rest, "("); idx >= 0 {
			end := strings.Index(rest[idx:], ")")
			if end > 0 {
				inside := rest[idx+1 : idx+end]
				fields := strings.Fields(inside)
				if len(fields) > 0 {
					author = fields[0]
				}
				if t := strings.TrimSpace(rest[idx+end+1:]); t != "" {
					summary = t
				}
			}
		}
		result = append(result, domain.GitBlameLine{Line: i + 1, SHA: sha, Author: author, Summary: strings.TrimSpace(summary)})
	}
	return result
}

func parseLog(out string) []domain.GitCommit {
	lines := strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n")
	var result []domain.GitCommit
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		c := domain.GitCommit{}
		if len(parts) > 0 {
			c.SHA = parts[0]
		}
		if len(parts) > 1 {
			c.Author = parts[1]
		}
		if len(parts) > 2 {
			c.Date = parts[2]
		}
		if len(parts) > 3 {
			c.Message = parts[3]
		}
		result = append(result, c)
	}
	return result
}
