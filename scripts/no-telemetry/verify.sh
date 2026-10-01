#!/usr/bin/env bash

# SPDX-License-Identifier: Apache-2.0
# Copyright 2026 alibaba/open-code-review Contributors

# No-telemetry fork: release gate. Run after every upstream merge and before
# every tag; it must exit 0. See NO_TELEMETRY_GUIDELINES.md.
#
#   scripts/no-telemetry/verify.sh                # static + runtime checks
#   scripts/no-telemetry/verify.sh --static-only  # skip the strace run
#
# Exit codes: 0 clean, 1 a check failed, 2 the egress detector itself is broken.

set -euo pipefail
export LC_ALL=C # stable sort order for comm

ROOT="$(git rev-parse --show-toplevel)"
HERE="$ROOT/scripts/no-telemetry"
cd "$ROOT"

STATIC_ONLY=0
[ "${1:-}" = "--static-only" ] && STATIC_ONLY=1

FAILED=0
fail() { echo "  FAIL: $*"; FAILED=1; }
pass() { echo "  ok:   $*"; }
section() { echo; echo "== $*"; }

# ── 1. Fork patch markers ────────────────────────────────────────────────────
# Every hook in an upstream file carries the tag; a merge that drops or
# duplicates one shows up here. Fork-owned files are excluded: they are whole
# files, not hooks.
section "1. [no-telemetry fork] markers (expected: $HERE/markers.txt)"
actual="$(git grep -c '\[no-telemetry fork\]' -- . \
  ':!scripts/no-telemetry' ':!NO_TELEMETRY_GUIDELINES.md' ':!*_notelemetry*' | sort || true)"
expected="$(grep -vE '^\s*(#|$)' "$HERE/markers.txt" | sort)"
if [ "$actual" = "$expected" ]; then
  pass "$(echo "$expected" | wc -l | tr -d ' ') files, markers match"
else
  fail "marker inventory drifted (< expected, > actual):"
  diff <(echo "$expected") <(echo "$actual") | sed 's/^/        /' || true
fi

# ── 2. Linked dependencies ───────────────────────────────────────────────────
# The OTel API may be linked (in-process no-ops); the SDK, its exporters and
# gRPC must not be.
section "2. telemetry SDK / exporters absent from the binary"
if ! deps="$(go list -deps ./cmd/opencodereview)" || [ -z "$deps" ]; then
  fail "go list failed; cannot prove the SDK is absent"
  deps=""
fi
banned="$(echo "$deps" | grep -E '^go\.opentelemetry\.io/otel/(sdk|exporters)|^google\.golang\.org/grpc' || true)"
if [ -z "$deps" ]; then
  :
elif [ -z "$banned" ]; then
  pass "no otel/sdk, otel/exporters or grpc packages linked"
else
  fail "banned packages linked into ./cmd/opencodereview:"
  echo "$banned" | sed 's/^/        /'
fi

# ── 3. Hard-coded hosts ──────────────────────────────────────────────────────
# Every host literal in runtime code must be reviewed once and listed in
# known-hosts.txt. A new one after a merge means new egress to audit.
section "3. hard-coded hosts in runtime code (reviewed: $HERE/known-hosts.txt)"
hosts="$(git grep -hoE 'https?://[A-Za-z0-9.-]+\.[A-Za-z]{2,}' -- \
  'internal/*.go' 'cmd/*.go' 'bin/*.js' 'scripts/*.js' 'scripts/*.sh' \
  'extensions/vscode/src/*.ts' 'extensions/frontend/src/*.ts' 'extensions/frontend/src/*.tsx' \
  'extensions/idea/src/main/*.kt' 'extensions/idea/src/main/*.java' 'extensions/idea/src/main/*.xml' \
  'plugins/*.ts' 'plugins/*.js' \
  'pages/src/*.ts' 'pages/src/*.tsx' 'pages/index.html' \
  'install.sh' 'install.ps1' 'action.yml' \
  ':!*_test.go' ':!*.test.js' ':!*.test.ts' ':!*.test.tsx' ':!*.test.mjs' \
  ':!pages/src/content' ':!pages/src/i18n' ':!scripts/no-telemetry' \
  | sed -E 's#^https?://##' | tr 'A-Z' 'a-z' | sort -u || true)"
known="$(grep -vE '^\s*(#|$)' "$HERE/known-hosts.txt" | awk '{print $1}' | sort -u)"
new_hosts="$(comm -23 <(echo "$hosts") <(echo "$known"))"
if [ -z "$new_hosts" ]; then
  pass "$(echo "$hosts" | wc -l | tr -d ' ') hosts, all reviewed"
else
  fail "new host literals; audit each, then add it to known-hosts.txt with a reason:"
  for h in $new_hosts; do
    git grep -nF "$h" -- ':!*_test.go' ':!*.test.*' ':!scripts/no-telemetry' | head -3 | sed 's/^/        /'
  done
fi

# ── 4. Runtime egress trace ──────────────────────────────────────────────────
if [ "$STATIC_ONLY" = 1 ]; then
  section "4. runtime egress: skipped (--static-only)"
  [ "$FAILED" = 0 ] && echo && echo "PASS (static only)" && exit 0
  echo && echo "FAILED" && exit 1
fi

section "4. runtime egress (strace)"
command -v strace >/dev/null || { fail "strace not installed; rerun with --static-only or install it"; exit 1; }

WORK="$(mktemp -d)"
FAKE_PID=""
cleanup() { [ -n "$FAKE_PID" ] && kill "$FAKE_PID" 2>/dev/null; rm -rf "$WORK"; }
trap cleanup EXIT

go build -o "$WORK/ocr" ./cmd/opencodereview
go build -o "$WORK/fakellm" "$HERE/fakellm.go"
"$WORK/fakellm" >"$WORK/fakellm.addr" 2>"$WORK/fakellm.err" &
FAKE_PID=$!
for _ in $(seq 50); do [ -s "$WORK/fakellm.addr" ] && break; sleep 0.1; done
LLM_ADDR="$(head -1 "$WORK/fakellm.addr")"
[ -n "$LLM_ADDR" ] || { fail "fake LLM did not start: $(cat "$WORK/fakellm.err")"; exit 1; }

# Scratch repo with one reviewable commit, a tracked secret and an MCP-free,
# rc-free HOME so nothing but the fake LLM is configured.
REPO="$WORK/repo"
mkdir -p "$REPO" "$WORK/home"
git -C "$REPO" init -q -b main
printf 'package p\n\nfunc A() int { return 1 }\n' >"$REPO/a.go"
printf 'TOKEN=hunter2\n' >"$REPO/.env"
git -C "$REPO" add -A
git -C "$REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qm base
printf 'package p\n\nfunc A() int { return 2 }\n' >"$REPO/a.go"
git -C "$REPO" -c user.name=t -c user.email=t@t -c commit.gpgsign=false commit -qam change

# run_traced NAME ALLOWED_ADDR CMD... : trace every connect/sendto of CMD and
# its children; print each destination not in ALLOWED_ADDR as a LEAK row.
# Loopback-only AF_UNIX sockets and AF_UNSPEC disconnects are always fine.
run_traced() {
  local name="$1" allowed="$2"
  shift 2
  local log="$WORK/$name.strace"
  env -i PATH="$PATH" HOME="$WORK/home" USERPROFILE="$WORK/home" LC_ALL=C \
    OCR_LLM_URL="http://$LLM_ADDR/v1/messages" OCR_LLM_TOKEN=fake OCR_LLM_MODEL=fake \
    OCR_LLM_PROTOCOL=anthropic OCR_LLM_AUTH_HEADER=x-api-key OCR_LLM_TIMEOUT=30 \
    OCR_ENABLE_TELEMETRY=1 OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4317 \
    "${EXTRA_ENV[@]}" \
    strace -f -qq -e trace=connect,sendto,sendmsg -s 200 -o "$log" "$@" \
    >"$WORK/$name.out" 2>&1 || true
  { grep -E 'sa_family=AF_INET6?' "$log" || true; } |
    sed -nE 's/.*sin6?_port=htons\(([0-9]+)\).*(inet_addr\("|inet_pton\(AF_INET6, ")([^"]+)".*/\3:\1/p' |
    sort -u | while read -r dest; do
      if [ "$dest" != "$allowed" ]; then echo "LEAK $name -> $dest"; fi
    done
}

EXTRA_ENV=()
leaks=""
leaks+="$(run_traced version "-" "$WORK/ocr" version)"
leaks+="$(run_traced review "$LLM_ADDR" "$WORK/ocr" review --repo "$REPO" --from HEAD~1 --to HEAD --audience agent)"
if [ -z "$leaks" ]; then
  pass "ocr version: no sockets; ocr review: only $LLM_ADDR (telemetry requested via env, ignored)"
else
  fail "unexpected egress:"
  echo "$leaks" | sed 's/^/        /'
fi
if ! grep -q "htons(${LLM_ADDR##*:})" "$WORK/review.strace"; then
  fail "review never reached the fake LLM; the trace proves nothing. Output:"
  sed 's/^/        /' "$WORK/review.out" | tail -20
fi
if grep -q 'hunter2' "$WORK/review.strace"; then
  fail "the .env secret was sent over a socket"
fi

# Self-test: an LLM URL on an unresolvable host must be flagged (DNS lookup
# or connect). If it is not, the detector is broken and nothing above counts.
section "5. detector self-test"
selftest="$(EXTRA_ENV=(OCR_LLM_URL=http://egress-selftest.invalid/v1/messages);
  run_traced selftest "$LLM_ADDR" "$WORK/ocr" llm test)"
if [ -n "$selftest" ]; then
  pass "probe to egress-selftest.invalid flagged"
else
  echo "  BROKEN: the self-test probe was not flagged; the egress check cannot be trusted"
  exit 2
fi

echo
if [ "$FAILED" = 0 ]; then echo "PASS"; else echo "FAILED"; exit 1; fi
