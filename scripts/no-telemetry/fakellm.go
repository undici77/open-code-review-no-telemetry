// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

//go:build ignore

// No-telemetry fork: a minimal Anthropic-messages-compatible server on
// 127.0.0.1, used by verify.sh as the only allowed egress destination while
// it traces a real `ocr review`. It prints its listen address on the first
// stdout line, then serves until killed. Run with:
//
//	go run scripts/no-telemetry/fakellm.go
package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintln(os.Stderr, "fakellm:", err)
		os.Exit(1)
	}
	fmt.Println(ln.Addr().String())

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		// Rounds that carry tool definitions end the task at once; anything
		// else (planning, grouping) gets a short text answer.
		if bytes.Contains(body, []byte(`"tools"`)) {
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"fake",
				"content":[{"type":"tool_use","id":"tu_1","name":"task_done","input":{"state":"DONE"}}],
				"stop_reason":"tool_use","usage":{"input_tokens":1,"output_tokens":1}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"fake",
			"content":[{"type":"text","text":"plan: call task_done"}],
			"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`))
	})
	if err := http.Serve(ln, handler); err != nil {
		fmt.Fprintln(os.Stderr, "fakellm:", err)
		os.Exit(1)
	}
}
