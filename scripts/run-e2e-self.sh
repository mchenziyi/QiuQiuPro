#!/usr/bin/env bash
# QiuQiuPro Self-test runner — 无需 DeepSeek，覆盖 e2e-test-cases-self.md
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

PASS=0
FAIL=0
SKIP=0

pass() { echo "PASS  $1"; PASS=$((PASS + 1)); }
fail() { echo "FAIL  $1  $2"; FAIL=$((FAIL + 1)); }
skip() { echo "SKIP  $1  $2"; SKIP=$((SKIP + 1)); }

echo "=== 1/4 build ==="
if go build ./...; then pass "build"; else fail "build" "compile error"; fi

echo "=== 2/4 go test ==="
if go test ./... -count=1; then pass "go-test"; else fail "go-test" "see above"; fi

echo "=== 3/4 race ==="
if go test -race ./... -count=1; then pass "go-test-race"; else fail "go-test-race" "see above"; fi

echo "=== 4/4 tool regression ==="
if go test ./tool/ -count=1 -run 'TestEditFileTool|TestMultiEditTool|TestDeleteRangeTool|TestGrepTool|TestGitCommitTool'; then
  pass "tool-regression"
else
  fail "tool-regression" "see above"
fi

echo "=== CLI commands (isolated HOME) ==="
WT=$(mktemp -d /tmp/qiuqiu-self-wt.XXXXXX)
git worktree add "$WT" HEAD -q 2>/dev/null || { cp -R "$ROOT/." "$WT/"; cd "$WT" && git init -q 2>/dev/null || true; }
cd "$WT"
go build -o qiuqiupro .
BIN="$WT/qiuqiupro"
HOME_DIR=$(mktemp -d /tmp/qiuqiu-self-home.XXXXXX)
mkdir -p "$HOME_DIR/.qiuqiu"
if [ -s "$HOME/.qiuqiu/key" ]; then cp "$HOME/.qiuqiu/key" "$HOME_DIR/.qiuqiu/key"; chmod 600 "$HOME_DIR/.qiuqiu/key"
elif [ -n "${DEEPSEEK_API_KEY:-}" ]; then printf '%s' "$DEEPSEEK_API_KEY" > "$HOME_DIR/.qiuqiu/key"; chmod 600 "$HOME_DIR/.qiuqiu/key"
else skip "cli-batch" "no API key for CLI startup"; HOME_DIR=""; fi

cli_check() {
  local id="$1" pat="$2" input="$3"
  [ -z "$HOME_DIR" ] && return
  local out
  out=$(printf '%b' "$input" | HOME="$HOME_DIR" "$BIN" -q 2>&1) || true
  if echo "$out" | grep -q "$pat"; then pass "$id"; else fail "$id" "expected /$pat/"; fi
}

if [ -n "$HOME_DIR" ]; then
  cli_check "TC-CMD-01" "help" '/help\nexit\n'
  cli_check "TC-CMD-03" "只读" '/readonly on\nexit\n'
  cli_check "TC-CMD-13" "maxSteps" '/maxsteps\n/maxsteps 5\nexit\n'
  cli_check "TC-CMD-15" "没有可恢复" '/resume\nexit\n'
  cli_check "TC-CMD-17" "再见" 'exit\n'
  # /test via CLI is flaky under pipe (stdin shared with LLM); verify package test directly
  if go test ./command/ -count=1 >/dev/null 2>&1; then pass "TC-CMD-07"; else fail "TC-CMD-07" "go test ./command/ failed"; fi

  MCP_HOME=$(mktemp -d /tmp/qiuqiu-self-mcp.XXXXXX)
  mkdir -p "$MCP_HOME/.qiuqiu"
  cp "$HOME_DIR/.qiuqiu/key" "$MCP_HOME/.qiuqiu/key"
  printf '[{"name":"bad","command":"/no/such/mcp","args":[]}]' > "$MCP_HOME/.qiuqiu/mcp_servers.json"
  out=$(printf 'exit\n' | HOME="$MCP_HOME" "$BIN" -q 2>&1) || true
  if echo "$out" | grep -q "加载失败"; then pass "TC-MCP-02"; else fail "TC-MCP-02" "no load failure msg"; fi
fi

# TC-CKPT-04: 已知 Bug（session ID 不跨重启），不计入 self pass/fail
echo "KNOWN TC-CKPT-04  cross-restart checkpoint — product bug B-7"

echo ""
echo "=========================================="
echo "SELF-TEST SUMMARY: $PASS pass, $FAIL fail, $SKIP skip"
echo "LLM cases (49): run manually — docs/e2e-test-cases-llm.md"
echo "=========================================="

[ "$FAIL" -eq 0 ]
