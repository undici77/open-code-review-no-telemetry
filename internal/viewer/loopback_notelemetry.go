// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

// No-telemetry fork: the viewer serves session JSONL (reviewed source code and
// LLM output), so it only ever listens on loopback. A wildcard or external
// --addr is rewritten to 127.0.0.1 with the same port. The hook calling into
// this file is tagged "[no-telemetry fork]". See NO_TELEMETRY_GUIDELINES.md.

package viewer

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// loopbackWarnWriter receives the rewrite warning; a variable for tests.
var loopbackWarnWriter io.Writer = os.Stderr

// loopbackOnly returns addr unchanged when its host is loopback, and otherwise
// the same port on 127.0.0.1. Unparseable input is returned as-is so
// net.Listen reports the error exactly as upstream would.
func loopbackOnly(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if isLoopbackHost(strings.ToLower(host)) {
		return addr
	}
	safe := net.JoinHostPort("127.0.0.1", port)
	fmt.Fprintf(loopbackWarnWriter, "[ocr] no-telemetry build: viewer is loopback-only, listening on %s instead of %s\n", safe, addr)
	return safe
}
