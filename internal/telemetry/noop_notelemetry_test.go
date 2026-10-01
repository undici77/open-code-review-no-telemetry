// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

package telemetry

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
)

func resetNoopState(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	oldWriter := noticeWriter
	noticeWriter = &buf
	initialized = false
	shutdownFuncs = nil
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OCR_ENABLE_TELEMETRY", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OCR_CONTENT_LOGGING", "")
	t.Cleanup(func() {
		noticeWriter = oldWriter
		initialized = false
		shutdownFuncs = nil
	})
	return &buf
}

func TestNoopInit_DefaultIsSilentAndDisabled(t *testing.T) {
	buf := resetNoopState(t)
	if Init(context.Background()) {
		t.Fatal("Init returned true; telemetry must never be enabled")
	}
	if IsEnabled() {
		t.Error("IsEnabled returned true after Init")
	}
	if buf.Len() != 0 {
		t.Errorf("unexpected notice: %q", buf.String())
	}
}

func TestNoopInit_EnvRequestPrintsNoticeOnly(t *testing.T) {
	buf := resetNoopState(t)
	t.Setenv("OCR_ENABLE_TELEMETRY", "1")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:4317")
	t.Setenv("OCR_CONTENT_LOGGING", "1")

	if Init(context.Background()) {
		t.Fatal("Init returned true; telemetry must never be enabled")
	}
	if !strings.Contains(buf.String(), "compiled out") {
		t.Errorf("notice missing, got %q", buf.String())
	}
	if IsEnabled() || ContentLogging() {
		t.Error("telemetry or content logging reported as enabled")
	}
	if len(shutdownFuncs) != 0 {
		t.Error("Init registered shutdown functions")
	}
	// Second call is a no-op and prints nothing more.
	buf.Reset()
	if Init(context.Background()) {
		t.Error("second Init returned true")
	}
	if buf.Len() != 0 {
		t.Errorf("second Init printed %q", buf.String())
	}
}

func TestNoopInit_ConfigFileRequestPrintsNotice(t *testing.T) {
	buf := resetNoopState(t)
	home := os.Getenv("HOME")
	dir := filepath.Join(home, ".opencodereview")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := `{"telemetry":{"enabled":true,"otlp_endpoint":"http://127.0.0.1:4317","content_logging":true}}`
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	if Init(context.Background()) {
		t.Fatal("Init returned true; telemetry must never be enabled")
	}
	if !strings.Contains(buf.String(), "compiled out") {
		t.Errorf("notice missing, got %q", buf.String())
	}
}

func TestNoopInit_LeavesGlobalProvidersUntouched(t *testing.T) {
	resetNoopState(t)
	t.Setenv("OCR_ENABLE_TELEMETRY", "1")
	beforeTP := otel.GetTracerProvider()
	beforeMP := otel.GetMeterProvider()
	Init(context.Background())
	if otel.GetTracerProvider() != beforeTP {
		t.Error("Init replaced the global tracer provider")
	}
	if otel.GetMeterProvider() != beforeMP {
		t.Error("Init replaced the global meter provider")
	}
}
