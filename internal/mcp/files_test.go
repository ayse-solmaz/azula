package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	c := NewFilesConnector(root)
	_, err := c.ReadFile(context.Background(), "proj1", "../secret.txt")
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestSeedAndReadSample(t *testing.T) {
	root := t.TempDir()
	c := NewFilesConnector(root)
	wd, _ := os.Getwd()
	sample := filepath.Clean(filepath.Join(wd, "..", "..", "samples", "broken-pipeline"))
	if _, err := os.Stat(filepath.Join(sample, "training.log")); err != nil {
		t.Skip("sample pipeline not found from module path")
	}
	files, err := c.SeedFromDir(context.Background(), "abc123", sample)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 4 {
		t.Fatalf("expected sample files, got %d", len(files))
	}
	body, err := c.ReadFile(context.Background(), "abc123", "training.log")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "CUDA out of memory") {
		t.Fatalf("unexpected log content")
	}
}
