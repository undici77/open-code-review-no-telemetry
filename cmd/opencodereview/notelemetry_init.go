// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

// No-telemetry fork: process-wide egress hardening applied before any command
// runs. Environment variables set here are inherited by every child process
// (git, MCP stdio servers, api_key_cmd), so one file covers code paths that
// bypass the shared runners. See NO_TELEMETRY_GUIDELINES.md.

package main

import (
	"os"
	"strings"
)

// versionSuffix marks binaries built from the no-telemetry fork.
const versionSuffix = "-no-telemetry"

// forcedEnv lists variables that are always overridden, whatever the user set.
var forcedEnv = map[string]string{
	// The AWS SDK credential chain (Bedrock) must never fall back to the EC2
	// instance metadata service at 169.254.169.254.
	"AWS_EC2_METADATA_DISABLED": "true",
	// git must not lazily fetch missing objects from a promisor remote in a
	// partial clone, nor block on a credential prompt.
	"GIT_NO_LAZY_FETCH":   "1",
	"GIT_TERMINAL_PROMPT": "0",
}

// strippedEnvPrefixes lists variable prefixes removed from the environment so
// child processes cannot pick up an OpenTelemetry exporter configuration.
var strippedEnvPrefixes = []string{"OTEL_EXPORTER_", "OTEL_TRACES_EXPORTER", "OTEL_METRICS_EXPORTER", "OTEL_LOGS_EXPORTER"}

func init() {
	applyNoTelemetryEnv()
	if !strings.HasSuffix(Version, versionSuffix) {
		Version += versionSuffix
	}
}

// applyNoTelemetryEnv enforces forcedEnv and strips exporter configuration.
func applyNoTelemetryEnv() {
	for k, v := range forcedEnv {
		_ = os.Setenv(k, v)
	}
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		for _, p := range strippedEnvPrefixes {
			if strings.HasPrefix(name, p) {
				_ = os.Unsetenv(name)
				break
			}
		}
	}
}
