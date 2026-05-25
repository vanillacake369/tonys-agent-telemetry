#!/usr/bin/env bash
# Smoke tests that exercise real binaries — run by the user on a real tty.
# Each test exits the binary cleanly (no Ctrl+C handling required).
#
# Usage: ./test/smoke.sh
set -euo pipefail

cd "$(dirname "$0")/.."

PASS=0
FAIL=0
note() { printf "  \033[36m%s\033[0m\n" "$*"; }
ok()   { PASS=$((PASS+1)); printf "  \033[32m✓\033[0m %s\n" "$*"; }
bad()  { FAIL=$((FAIL+1)); printf "  \033[31m✗\033[0m %s\n" "$*"; }

echo "=== build ==="
make build > /dev/null
ok "built tonys-agent-telemetry + tonys-agent-telemetry-hook"

echo ""
echo "=== T1: --version ==="
out=$(./bin/tonys-agent-telemetry --version)
if [[ "$out" =~ tonys-agent-telemetry ]]; then ok "--version output: $out"; else bad "got: $out"; fi

echo ""
echo "=== T2: --help mentions Phase 4 flags ==="
help=$(./bin/tonys-agent-telemetry --help)
for flag in "--otlp-export" "--snapshot-record" "--replay"; do
    if grep -q -- "$flag" <<< "$help"; then ok "--help mentions $flag"; else bad "missing: $flag"; fi
done

echo ""
echo "=== T3: non-tty stdout exits cleanly ==="
out=$(echo "" | ./bin/tonys-agent-telemetry 2>&1 || true)
if [[ "$out" =~ "not a terminal" ]]; then ok "non-tty reports clean error"; else bad "got: $out"; fi

echo ""
echo "=== T4: hook-handler honors Control Plane deny ==="
tmp=$(mktemp -d)
mkdir -p "$tmp/.config/tonys-agent-telemetry"
cat > "$tmp/.config/tonys-agent-telemetry/policy.toml" <<EOF
[budget]
session_max_usd = 5.0
[tools]
denylist = ["Bash:rm -rf*"]
EOF
set +e
out=$(echo '{"session_id":"smoke","tool_name":"Bash","tool_input":{"command":"rm -rf /tmp/x"}}' \
    | HOME="$tmp" XDG_CONFIG_HOME="$tmp/.config" XDG_CACHE_HOME="$tmp/.cache" \
      ./bin/tonys-agent-telemetry-hook PreToolUse 2>&1)
rc=$?
set -e
if [[ $rc == 2 && "$out" =~ BLOCKED ]]; then
    ok "denylisted tool blocked with exit 2 + BLOCKED stderr"
else
    bad "rc=$rc out=$out"
fi
rm -rf "$tmp"

echo ""
echo "=== T5: hook-handler allows non-listed tools ==="
tmp=$(mktemp -d)
mkdir -p "$tmp/.config/tonys-agent-telemetry"
cat > "$tmp/.config/tonys-agent-telemetry/policy.toml" <<EOF
[budget]
session_max_usd = 5.0
[tools]
denylist = ["Bash:rm -rf*"]
EOF
echo '{"session_id":"smoke","tool_name":"Bash","tool_input":{"command":"echo hello"}}' \
    | HOME="$tmp" XDG_CONFIG_HOME="$tmp/.config" XDG_CACHE_HOME="$tmp/.cache" \
      ./bin/tonys-agent-telemetry-hook PreToolUse
if [[ $? == 0 ]]; then ok "allowed tool exits 0"; else bad "should allow"; fi
rm -rf "$tmp"

echo ""
echo "=== T6: OTLP receiver round-trip (go test) ==="
set +e
out=$(go test -run TestIngest_AcceptsValidExport -count=1 -v ./internal/provider/otlp/ 2>&1)
rc=$?
set -e
if [[ $rc == 0 && "$out" =~ PASS ]]; then
    ok "OTLP receiver accepts POST and emits Span"
else
    bad "OTLP test failed: $out"
fi

echo ""
echo "=== T7: snapshot record + replay round-trip (go test) ==="
set +e
out=$(go test -run TestRecorderPlayer_RoundTrip -count=1 -v ./internal/snapshot/ 2>&1)
rc=$?
set -e
if [[ $rc == 0 && "$out" =~ PASS ]]; then
    ok "snapshot round-trip preserves all spans"
else
    bad "snapshot test: $out"
fi

echo ""
echo "=== T8: splitChannel delivers all spans (no silent drop) ==="
set +e
out=$(go test -run TestSplitChannel_DeliversAllSpansToAllBranches -count=1 -v -race . 2>&1)
rc=$?
set -e
if [[ $rc == 0 && "$out" =~ PASS ]]; then
    ok "splitChannel delivers 10,000 spans to all 3 branches"
else
    bad "splitChannel: $out"
fi

echo ""
echo "=== T9: App.Update routes load-messages to owning tabs ==="
set +e
out=$(go test -run 'TestApp_Routes' -count=1 -v -race ./internal/tui/ 2>&1)
rc=$?
set -e
if [[ $rc == 0 && "$out" =~ PASS ]]; then
    routes_passed=$(grep -c '^--- PASS' <<< "$out")
    ok "App routes $routes_passed message types to correct tabs"
else
    bad "routing: $out"
fi

echo ""
echo "=== T10: Hooks/Control tabs auto-load (not stuck in loading) ==="
set +e
out=$(go test -run 'TestApp_HooksLoadingStartsTrueResolvesToFalse|TestApp_RoutesHooksLoadedMsg|TestApp_RoutesControlRefreshMsg' -count=1 -v ./internal/tui/ 2>&1)
rc=$?
set -e
if [[ $rc == 0 && "$out" =~ PASS ]]; then
    ok "Hooks + Control tabs leave loading state on first Init"
else
    bad "auto-load: $out"
fi

echo ""
echo "=== T11: DAG tab receives spans via App.Update routing ==="
set +e
out=$(go test -run 'TestApp_RoutesSpanBatchMsg|TestApp_RoutesSpanCollectedMsg' -count=1 -v ./internal/tui/ 2>&1)
rc=$?
set -e
if [[ $rc == 0 && "$out" =~ PASS ]]; then
    ok "DAG tab receives both single and batch span messages"
else
    bad "DAG routing: $out"
fi

echo ""
echo "=== Summary ==="
echo "  passed: $PASS"
echo "  failed: $FAIL"
if [[ $FAIL -gt 0 ]]; then exit 1; fi
