// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package viewer

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoopbackOnly(t *testing.T) {
	var buf bytes.Buffer
	old := loopbackWarnWriter
	loopbackWarnWriter = &buf
	t.Cleanup(func() { loopbackWarnWriter = old })

	cases := []struct {
		in, want string
		warn     bool
	}{
		{"localhost:5483", "localhost:5483", false},
		{"127.0.0.1:0", "127.0.0.1:0", false},
		{"[::1]:5483", "[::1]:5483", false},
		{":3000", "127.0.0.1:3000", true},
		{"0.0.0.0:3000", "127.0.0.1:3000", true},
		{"[::]:3000", "127.0.0.1:3000", true},
		{"192.168.1.10:5483", "127.0.0.1:5483", true},
		{"example.com:80", "127.0.0.1:80", true},
		{"not-an-addr", "not-an-addr", false},
	}
	for _, c := range cases {
		buf.Reset()
		if got := loopbackOnly(c.in); got != c.want {
			t.Errorf("loopbackOnly(%q) = %q, want %q", c.in, got, c.want)
		}
		if warned := strings.Contains(buf.String(), "loopback-only"); warned != c.warn {
			t.Errorf("loopbackOnly(%q) warned = %v, want %v", c.in, warned, c.warn)
		}
	}
}
