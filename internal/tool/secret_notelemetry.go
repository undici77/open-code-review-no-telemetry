// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

// No-telemetry fork: keeps credential files out of the LLM tools. Upstream
// applies allowlist.IsSecretPath only when selecting diffs; the file_read,
// code_search and file_find tools could still hand a .env or id_rsa to the
// model. The hooks calling into this file are tagged "[no-telemetry fork]".
// See NO_TELEMETRY_GUIDELINES.md.

package tool

import (
	"fmt"
	"path/filepath"
	"strings"

	allowedext "github.com/alibaba/open-code-review/internal/config/allowlist"
)

// isSecretToolPath reports whether a repository-relative path names a
// credential file. The path is normalised first so "./.env" or "a/../.env"
// cannot slip past the patterns.
func isSecretToolPath(path string) bool {
	p := filepath.ToSlash(filepath.Clean(path))
	p = strings.TrimPrefix(p, "./")
	return allowedext.IsSecretPath(p)
}

// secretGuard refuses to read a credential file through an LLM tool.
func secretGuard(path string) error {
	if isSecretToolPath(path) {
		return fmt.Errorf("file path %q is a protected secret file and cannot be read", path)
	}
	return nil
}

// dropSecretPaths removes credential files from a list of candidate paths.
func dropSecretPaths(paths []string) []string {
	out := paths[:0:0]
	for _, p := range paths {
		if !isSecretToolPath(p) {
			out = append(out, p)
		}
	}
	return out
}
