// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 alibaba/open-code-review Contributors

// No-telemetry fork: this file replaces provider.go, which is excluded from
// the build together with exporter.go. It keeps the same package API so the
// rest of the codebase compiles unchanged, but it never installs an OTel SDK
// provider or exporter, so no span or metric can leave the process. The OTel
// API packages used by span.go, events.go and metrics.go fall back to their
// built-in no-op providers. See NO_TELEMETRY_GUIDELINES.md.

package telemetry

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// Package state shared with span.go, metrics.go, shutdown.go and the tests.
// The provider fields use the API interfaces rather than the SDK types so the
// SDK is never linked into the binary.
var (
	tracerProvider trace.TracerProvider
	meterProvider  metric.MeterProvider
	shutdownFuncs  []func(context.Context) error
	initialized    bool
)

// serviceName holds the name resolved during Init.
var serviceName = "open-code-review"

// noticeWriter receives the one-line notice printed when the user asks for
// telemetry. It is a variable so tests can capture it.
var noticeWriter io.Writer = os.Stderr

// Init resolves the telemetry configuration but never enables telemetry.
// When the config file or environment asks for it, a one-line notice tells
// the user that telemetry is compiled out of this build. Always returns false.
func Init(ctx context.Context) bool {
	if initialized {
		return false
	}
	initialized = true

	cfg := ResolveConfig(HomeConfigPath())
	serviceName = cfg.ServiceName
	if cfg.Enabled {
		fmt.Fprintln(noticeWriter, "[ocr] telemetry is compiled out in this no-telemetry build; nothing is exported")
	}
	return false
}

// IsEnabled reports whether a telemetry provider is active. Init never
// registers one, so this is false outside tests that set the state directly.
func IsEnabled() bool {
	return initialized && len(shutdownFuncs) > 0
}

// ContentLogging always returns false: prompt and response content is never
// attached to telemetry in this build.
func ContentLogging() bool {
	return false
}
