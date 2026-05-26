#!/usr/bin/env bash
# Idempotent demo-data seeder used by every .tape in this directory.
#
# Populates a self-contained $TONYS_DEMO_HOME with:
#  - 3 synthetic claudecode session JSONLs (under .claude/projects/)
#  - A signal store with 10 days of bucketed history (so Trends has a
#    populated sparkline immediately, no waiting 5 min for the first flush)
#
# Everything stays under /tmp/tonys-demo so it never collides with the
# user's real ~/.claude/. The .tape files set:
#   Env XDG_CONFIG_HOME /tmp/tonys-demo/config
#   Env XDG_CACHE_HOME  /tmp/tonys-demo/cache
#   Env TONYS_SIGNAL_STORE /tmp/tonys-demo/cache/tonys-agent-telemetry/signals
# and run `bash scripts/demo/seed.sh` once before launching the binary.

set -euo pipefail

DEMO_HOME="${TONYS_DEMO_HOME:-/tmp/tonys-demo}"
PROJECTS_DIR="$DEMO_HOME/.claude/projects/demo-refactor-auth"
STORE_DIR="${TONYS_SIGNAL_STORE:-$DEMO_HOME/cache/tonys-agent-telemetry/signals}"

mkdir -p "$PROJECTS_DIR" "$STORE_DIR"

# ── Synthetic claudecode session: refactor-auth (3 traces) ────────────
cat > "$PROJECTS_DIR/session-001.jsonl" <<'EOF'
{"type":"user","uuid":"u1","sessionId":"s001","timestamp":"2026-05-26T03:10:00Z","message":{"role":"user","content":"refactor the auth middleware to use the new policy module"}}
{"type":"assistant","uuid":"a1","parentUuid":"u1","sessionId":"s001","timestamp":"2026-05-26T03:10:02Z","message":{"role":"assistant","model":"claude-opus","usage":{"input_tokens":240,"output_tokens":891},"content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"internal/auth/middleware.go"}}]}}
{"type":"user","uuid":"u2","parentUuid":"a1","sessionId":"s001","timestamp":"2026-05-26T03:10:04Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"<file contents>"}]}}
{"type":"assistant","uuid":"a2","parentUuid":"u2","sessionId":"s001","timestamp":"2026-05-26T03:10:09Z","message":{"role":"assistant","model":"claude-opus","usage":{"input_tokens":450,"output_tokens":1240},"content":[{"type":"tool_use","id":"t2","name":"Edit","input":{"file_path":"internal/auth/middleware.go"}}]}}
{"type":"assistant","uuid":"a3","parentUuid":"a2","sessionId":"s001","timestamp":"2026-05-26T03:10:12Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t3","name":"Bash","input":{"command":"go test ./internal/auth/..."}}]}}
EOF

cat > "$PROJECTS_DIR/session-002.jsonl" <<'EOF'
{"type":"user","uuid":"u3","sessionId":"s002","timestamp":"2026-05-25T11:42:00Z","message":{"role":"user","content":"add tests for the new control plane"}}
{"type":"assistant","uuid":"a4","parentUuid":"u3","sessionId":"s002","timestamp":"2026-05-25T11:42:03Z","message":{"role":"assistant","model":"claude-opus","content":[{"type":"tool_use","id":"t4","name":"Bash","input":{"command":"grep -rn TestEnforce internal/"}}]}}
{"type":"assistant","uuid":"a5","parentUuid":"a4","sessionId":"s002","timestamp":"2026-05-25T11:42:14Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t5","name":"git_commit","input":{"message":"test(control): EnforceEvidence policy"}}],"stop_reason":"error"}}
EOF

cat > "$PROJECTS_DIR/session-003.jsonl" <<'EOF'
{"type":"user","uuid":"u4","sessionId":"s003","timestamp":"2026-05-24T18:15:00Z","message":{"role":"user","content":"investigate why DAG renders break on resize"}}
{"type":"assistant","uuid":"a6","parentUuid":"u4","sessionId":"s003","timestamp":"2026-05-24T18:15:04Z","message":{"role":"assistant","model":"claude-opus","content":[{"type":"tool_use","id":"t6","name":"Read","input":{"file_path":"internal/tui/tab_dag.go"}}]}}
EOF

# ── Signal store seed: 10 days of buckets so Trends shows a sparkline ─
# We write a single session file with 10 daily SnapshotEntries.
# Schema version 1 (matches internal/signalstore/schema.go).
STORE_FILE="$STORE_DIR/proj-demo.jsonl"
{
  echo '{"schema_version":"1","written_at":"2026-05-16T00:00:00Z","producer":"tonys-agent-telemetry demo seed"}'
  for d in 10 9 8 7 6 5 4 3 2 1; do
    DATE=$(date -u -v-${d}d +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u -d "${d} days ago" +%Y-%m-%dT00:00:00Z)
    # Stalled count varies; this gives the sparkline a recognisable shape.
    STALL_N=$(( (d * 3) % 7 + 1 ))
    FAIL_N=$(( (d * 2) % 4 ))
    echo "{\"captured_at\":\"$DATE\",\"signals\":[{\"id\":\"sig-stall-d$d\",\"type\":\"stalled_node\",\"trace_id\":\"t-d$d\",\"span_ids\":[\"s-d$d\"],\"evidence\":{\"stall_duration_ms\":${STALL_N}000,\"tool_name\":\"bash\"},\"confidence\":0.85,\"emitted_at\":\"$DATE\",\"provider_tier\":\"full\"},{\"id\":\"sig-fail-d$d\",\"type\":\"failed_handoff\",\"trace_id\":\"t-d$d\",\"span_ids\":[\"s-d$d-x\"],\"evidence\":{\"error_type\":\"merge_conflict\"},\"confidence\":0.7,\"emitted_at\":\"$DATE\",\"provider_tier\":\"full\"}]}"
  done
} > "$STORE_FILE"

# ── A sample replay file for the CLI demo (scenario 5) ────────────────
cat > "$DEMO_HOME/sample.jsonl" <<'EOF'
{"trace_id":"trace-demo","span_id":"root","start_time_unix_nano":1748194000000000000,"end_time_unix_nano":1748194015000000000,"system":"anthropic","status":"done","attrs":{"gen_ai.operation.name":"agent.session"}}
{"trace_id":"trace-demo","span_id":"llm1","parent_span_id":"root","start_time_unix_nano":1748194001000000000,"end_time_unix_nano":1748194004000000000,"system":"anthropic","model":"claude-opus","input_tokens":240,"output_tokens":891,"attrs":{"gen_ai.operation.name":"llm.request"}}
{"trace_id":"trace-demo","span_id":"stalled","parent_span_id":"root","start_time_unix_nano":1748194004000000000,"end_time_unix_nano":1748194028000000000,"system":"anthropic","status":"done","attrs":{"gen_ai.tool.name":"bash"}}
EOF

echo "demo data seeded under $DEMO_HOME (10 buckets, 3 sessions, 1 replay file)"
