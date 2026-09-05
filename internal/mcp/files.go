package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ayse-solmaz/azula/internal/domain"
)

const maxFileBytes = 50 * 1024 * 1024

var allowedExt = map[string]struct{}{
	".log":   {},
	".yaml":  {},
	".yml":   {},
	".py":    {},
	".json":  {},
	".jsonl": {},
	".csv":   {},
	".txt":   {},
}

type Connector interface {
	ListFiles(ctx context.Context, projectID string) ([]domain.ProjectFile, error)
	ReadFile(ctx context.Context, projectID, name string) (string, error)
	SaveUpload(ctx context.Context, projectID, filename, mimeType string, r io.Reader) (domain.ProjectFile, error)
	SeedFromDir(ctx context.Context, projectID, sampleDir string) ([]domain.ProjectFile, error)
	RestoreFileVersion(ctx context.Context, projectID, name string, version int) (domain.ProjectFile, error)
	RemoveProject(ctx context.Context, projectID string) error
	ListFileVersions(ctx context.Context, projectID, name string) ([]domain.FileVersion, error)
	ReadFileVersion(ctx context.Context, projectID, name string, version int) (string, error)
}

type FilesConnector struct {
	root string
}

func NewFilesConnector(root string) *FilesConnector {
	return &FilesConnector{root: root}
}

func (c *FilesConnector) projectDir(projectID string) (string, error) {
	if !safeID(projectID) {
		return "", domain.ErrPathTraversal
	}
	return filepath.Abs(filepath.Join(c.root, projectID))
}

func (c *FilesConnector) ListFiles(_ context.Context, projectID string) ([]domain.ProjectFile, error) {
	dir, err := c.projectDir(projectID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.ProjectFile{}, nil
		}
		return nil, err
	}
	var files []domain.ProjectFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, domain.ProjectFile{
			Name:       e.Name(),
			Path:       e.Name(),
			UploadedAt: info.ModTime().UTC(),
		})
	}
	return files, nil
}

func (c *FilesConnector) ReadFile(_ context.Context, projectID, name string) (string, error) {
	path, err := c.safePath(projectID, name)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (c *FilesConnector) SaveUpload(_ context.Context, projectID, filename, mimeType string, r io.Reader) (domain.ProjectFile, error) {
	filename = filepath.Base(filename)
	if err := validateName(filename); err != nil {
		return domain.ProjectFile{}, err
	}
	dir, err := c.projectDir(projectID)
	if err != nil {
		return domain.ProjectFile{}, err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return domain.ProjectFile{}, err
	}
	dest := filepath.Join(dir, filename)
	if err := withinRoot(dir, dest); err != nil {
		return domain.ProjectFile{}, err
	}
	if _, err := os.Stat(dest); err == nil {
		if err := c.snapshot(projectID, filename); err != nil {
			return domain.ProjectFile{}, err
		}
	}

	limited := io.LimitReader(r, maxFileBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return domain.ProjectFile{}, err
	}
	if len(data) > maxFileBytes {
		return domain.ProjectFile{}, domain.ErrFileTooLarge
	}
	if err := os.WriteFile(dest, data, 0o640); err != nil {
		return domain.ProjectFile{}, err
	}

	now := time.Now().UTC()
	return domain.ProjectFile{
		Name:       filename,
		Path:       filename,
		MimeType:   mimeType,
		UploadedAt: now,
	}, nil
}

func (c *FilesConnector) safePath(projectID, name string) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	dir, err := c.projectDir(projectID)
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, filepath.Base(name))
	if err := withinRoot(dir, full); err != nil {
		return "", err
	}
	return full, nil
}

func validateName(name string) error {
	name = filepath.Base(name)
	if name == "." || name == ".." || name == "" {
		return domain.ErrPathTraversal
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return domain.ErrPathTraversal
	}
	ext := strings.ToLower(filepath.Ext(name))
	if _, ok := allowedExt[ext]; !ok {
		return domain.ErrForbiddenFile
	}
	return nil
}

func safeID(id string) bool {
	if id == "" || strings.Contains(id, "..") {
		return false
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func withinRoot(root, target string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") {
		return domain.ErrPathTraversal
	}
	return nil
}

func FileRelPath(projectID, name string) string {
	return fmt.Sprintf("%s/%s", projectID, name)
}

func (c *FilesConnector) SeedFromDir(ctx context.Context, projectID, sampleDir string) ([]domain.ProjectFile, error) {
	src, err := filepath.Abs(sampleDir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("sample pipeline: %w", err)
	}
	var files []domain.ProjectFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if _, ok := allowedExt[ext]; !ok {
			continue
		}
		f, err := os.Open(filepath.Join(src, e.Name()))
		if err != nil {
			return nil, err
		}
		saved, err := c.SaveUpload(ctx, projectID, e.Name(), mimeFor(ext), f)
		_ = f.Close()
		if err != nil {
			return nil, err
		}
		files = append(files, saved)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no allowed files in %s", sampleDir)
	}
	return files, nil
}

func (c *FilesConnector) versionsDir(projectID, name string) (string, error) {
	dir, err := c.projectDir(projectID)
	if err != nil {
		return "", err
	}
	vdir := filepath.Join(dir, ".versions", filepath.Base(name))
	if err := withinRoot(dir, vdir); err != nil {
		return "", err
	}
	return vdir, nil
}

func (c *FilesConnector) snapshot(projectID, name string) error {
	src, err := c.safePath(projectID, name)
	if err != nil {
		return err
	}
	vdir, err := c.versionsDir(projectID, name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(vdir, 0o750); err != nil {
		return err
	}
	entries, _ := os.ReadDir(vdir)
	n := len(entries) + 1
	dest := filepath.Join(vdir, fmt.Sprintf("%d", n))
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0o640)
}

func (c *FilesConnector) RestoreFileVersion(_ context.Context, projectID, name string, version int) (domain.ProjectFile, error) {
	if err := validateName(name); err != nil {
		return domain.ProjectFile{}, err
	}
	if version < 1 {
		return domain.ProjectFile{}, domain.ErrVersionNotFound
	}
	vdir, err := c.versionsDir(projectID, name)
	if err != nil {
		return domain.ProjectFile{}, err
	}
	src := filepath.Join(vdir, fmt.Sprintf("%d", version))
	if err := withinRoot(vdir, src); err != nil {
		return domain.ProjectFile{}, err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.ProjectFile{}, domain.ErrVersionNotFound
		}
		return domain.ProjectFile{}, err
	}
	if err := c.snapshot(projectID, name); err != nil && !os.IsNotExist(err) {
		return domain.ProjectFile{}, err
	}
	dest, err := c.safePath(projectID, name)
	if err != nil {
		return domain.ProjectFile{}, err
	}
	if err := os.WriteFile(dest, data, 0o640); err != nil {
		return domain.ProjectFile{}, err
	}
	st, _ := os.Stat(dest)
	uploaded := time.Now().UTC()
	if st != nil {
		uploaded = st.ModTime().UTC()
	}
	return domain.ProjectFile{Name: filepath.Base(name), Path: filepath.Base(name), MimeType: mimeFor(filepath.Ext(name)), UploadedAt: uploaded}, nil
}

func (c *FilesConnector) RemoveProject(_ context.Context, projectID string) error {
	dir, err := c.projectDir(projectID)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (c *FilesConnector) ListFileVersions(_ context.Context, projectID, name string) ([]domain.FileVersion, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	vdir, err := c.versionsDir(projectID, name)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(vdir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.FileVersion{}, nil
		}
		return nil, err
	}
	out := make([]domain.FileVersion, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(e.Name(), "%d", &n); err != nil {
			continue
		}
		info, _ := e.Info()
		uploaded := time.Now().UTC()
		if info != nil {
			uploaded = info.ModTime().UTC()
		}
		out = append(out, domain.FileVersion{
			ProjectID: projectID, FileName: filepath.Base(name), Version: n,
			Path:      filepath.Join(".versions", filepath.Base(name), e.Name()),
			CreatedAt: uploaded,
		})
	}
	return out, nil
}

func (c *FilesConnector) ReadFileVersion(_ context.Context, projectID, name string, version int) (string, error) {
	if err := validateName(name); err != nil {
		return "", err
	}
	if version < 1 {
		return "", domain.ErrVersionNotFound
	}
	vdir, err := c.versionsDir(projectID, name)
	if err != nil {
		return "", err
	}
	src := filepath.Join(vdir, fmt.Sprintf("%d", version))
	if err := withinRoot(vdir, src); err != nil {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return "", domain.ErrVersionNotFound
		}
		return "", err
	}
	return string(data), nil
}

func mimeFor(ext string) string {
	switch ext {
	case ".json", ".jsonl":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".py":
		return "text/x-python"
	default:
		return "text/plain"
	}
}

var _ Connector = (*FilesConnector)(nil)
