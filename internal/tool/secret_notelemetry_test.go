// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package tool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsSecretToolPath(t *testing.T) {
	cases := map[string]bool{
		".env":                true,
		"./.env":              true,
		"sub/../.env":         true,
		"config/.env.local":   true,
		"home/.ssh/config":    true,
		"deploy/id_ed25519":   true,
		".npmrc":              true,
		"main.go":             false,
		"docs/env.md":         false,
		"internal/.envoy.yml": false,
	}
	for path, want := range cases {
		if got := isSecretToolPath(path); got != want {
			t.Errorf("isSecretToolPath(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestDropSecretPaths(t *testing.T) {
	in := []string{"a.go", ".env", "b/id_rsa", "c.ts"}
	got := dropSecretPaths(in)
	if strings.Join(got, ",") != "a.go,c.ts" {
		t.Errorf("dropSecretPaths = %v", got)
	}
	if strings.Join(in, ",") != "a.go,.env,b/id_rsa,c.ts" {
		t.Errorf("input slice was modified: %v", in)
	}
}

// newSecretRepo builds a git repo with a tracked .env next to source code.
func newSecretRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	files := map[string]string{
		"main.go": "package main // SECRET_TOKEN usage\n",
		".env":    "SECRET_TOKEN=hunter2\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestFileReader_RefusesSecretFiles(t *testing.T) {
	dir := newSecretRepo(t)
	for _, fr := range []*FileReader{
		{RepoDir: dir, Mode: ModeWorkspace},
		{RepoDir: dir, Mode: ModeCommit, Ref: "HEAD"},
	} {
		if _, err := fr.Read(context.Background(), ".env"); err == nil || !strings.Contains(err.Error(), "protected secret") {
			t.Errorf("mode %v Read(.env) err = %v, want protected secret error", fr.Mode, err)
		}
		if _, _, err := fr.ReadLines(context.Background(), "./.env", 1, 10); err == nil {
			t.Errorf("mode %v ReadLines(./.env) returned no error", fr.Mode)
		}
		if _, err := fr.Read(context.Background(), "main.go"); err != nil {
			t.Errorf("mode %v Read(main.go) err = %v", fr.Mode, err)
		}
	}
}

func TestCodeSearch_HidesSecretFiles(t *testing.T) {
	dir := newSecretRepo(t)
	p := NewCodeSearch(&FileReader{RepoDir: dir, Mode: ModeWorkspace})
	out, err := p.Execute(context.Background(), map[string]any{"search_text": "SECRET_TOKEN"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "hunter2") || strings.Contains(out, ".env") {
		t.Errorf("secret file leaked into search results:\n%s", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("expected main.go match, got:\n%s", out)
	}
}

func TestFileFind_HidesSecretFiles(t *testing.T) {
	dir := newSecretRepo(t)
	p := NewFileFind(&FileReader{RepoDir: dir, Mode: ModeWorkspace})
	out, err := p.Execute(context.Background(), map[string]any{"query_name": ".env"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, ".env") {
		t.Errorf("secret file listed by file_find:\n%s", out)
	}
}
