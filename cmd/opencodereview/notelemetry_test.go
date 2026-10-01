// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestRunNeverExportsTelemetry is the inverse of the upstream
// TestRunFlushesTelemetryOnError: even with telemetry requested, the run
// prints the compiled-out notice and no exporter output.
func TestRunNeverExportsTelemetry(t *testing.T) {
	if os.Getenv("OCR_TEST_RUN_ERROR") == "1" {
		rootCmd.SetArgs([]string{"--invalid-flag"})
		os.Exit(run())
	}

	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestRunNeverExportsTelemetry$")
	cmd.Env = append(os.Environ(),
		"OCR_TEST_RUN_ERROR=1",
		"OCR_ENABLE_TELEMETRY=1",
		"OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317",
		"HOME="+home,
		"USERPROFILE="+home,
	)
	output, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("exit error = %v, want exit code 1; output:\n%s", err, output)
	}
	if strings.Contains(string(output), `"Resource"`) {
		t.Fatalf("telemetry was exported; output:\n%s", output)
	}
	if !strings.Contains(string(output), "compiled out") {
		t.Fatalf("compiled-out notice missing; output:\n%s", output)
	}
}

func TestApplyNoTelemetryEnv(t *testing.T) {
	t.Setenv("AWS_EC2_METADATA_DISABLED", "false")
	t.Setenv("GIT_NO_LAZY_FETCH", "0")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://collector:4317")
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "x=y")
	t.Setenv("OTEL_TRACES_EXPORTER", "otlp")
	t.Setenv("OTEL_SERVICE_NAME", "kept")

	applyNoTelemetryEnv()

	for k, want := range forcedEnv {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	for _, k := range []string{"OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_TRACES_EXPORTER"} {
		if _, ok := os.LookupEnv(k); ok {
			t.Errorf("%s still set", k)
		}
	}
	if os.Getenv("OTEL_SERVICE_NAME") != "kept" {
		t.Error("unrelated OTEL_SERVICE_NAME was removed")
	}
}

func TestVersionCarriesNoTelemetrySuffix(t *testing.T) {
	if !strings.HasSuffix(Version, versionSuffix) {
		t.Errorf("Version = %q, want suffix %q", Version, versionSuffix)
	}
}
