#!/bin/bash
# Tests for boxxy-pfctl-helper.sh — validates argument parsing and safety checks
# without requiring sudo or pfctl.
#
# Run: bash scripts/boxxy-pfctl-helper_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HELPER="${SCRIPT_DIR}/boxxy-pfctl-helper.sh"
PASS=0
FAIL=0

assert_fails() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "FAIL: $desc (expected failure, got success)"
        FAIL=$((FAIL + 1))
    else
        echo "PASS: $desc"
        PASS=$((PASS + 1))
    fi
}

assert_succeeds() {
    local desc="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "FAIL: $desc (expected success, got failure)"
        FAIL=$((FAIL + 1))
    fi
}

assert_output_contains() {
    local desc="$1"
    local needle="$2"
    shift 2
    local output
    output=$("$@" 2>&1) || true
    if echo "$output" | grep -q "$needle"; then
        echo "PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "FAIL: $desc (output does not contain '$needle')"
        echo "  got: $output"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== boxxy-pfctl-helper.sh tests ==="
echo ""

# --- No arguments ---
assert_fails "no arguments exits nonzero" bash "$HELPER"

# --- Bad subcommand ---
assert_fails "unknown subcommand exits nonzero" bash "$HELPER" badcmd

# --- Anchor validation ---
assert_output_contains \
    "activate rejects non-com.boxxy anchor" \
    "anchor must start with com.boxxy" \
    bash "$HELPER" activate /tmp/boxxy-pinhole-test.conf evil.anchor

assert_output_contains \
    "deactivate rejects non-com.boxxy anchor" \
    "anchor must start with com.boxxy" \
    bash "$HELPER" deactivate evil.anchor

assert_output_contains \
    "status rejects non-com.boxxy anchor" \
    "anchor must start with com.boxxy" \
    bash "$HELPER" status not.boxxy.anchor

# --- Conf path validation ---
assert_output_contains \
    "activate rejects conf outside /tmp/boxxy-pinhole-*" \
    "conf must be in /tmp/boxxy-pinhole" \
    bash "$HELPER" activate /etc/pf.conf com.boxxy.test

assert_output_contains \
    "activate rejects relative conf path" \
    "conf must be in /tmp/boxxy-pinhole" \
    bash "$HELPER" activate boxxy-pinhole-test.conf com.boxxy.test

# --- Missing arguments ---
assert_fails "activate with no args" bash "$HELPER" activate
assert_fails "activate with only conf" bash "$HELPER" activate /tmp/boxxy-pinhole-test.conf
assert_fails "deactivate with no args" bash "$HELPER" deactivate
assert_fails "status with no args" bash "$HELPER" status

# --- Nonexistent conf file ---
assert_output_contains \
    "activate rejects nonexistent conf file" \
    "does not exist" \
    bash "$HELPER" activate /tmp/boxxy-pinhole-nonexistent-12345.conf com.boxxy.test

# --- Valid anchor names accepted (up to validation, fails at pfctl) ---
# Create a temporary conf file for this test
TMPCONF="/tmp/boxxy-pinhole-unittest-$$.conf"
echo "pass on bridge100 proto tcp from 192.168.64.2 to any port 443" > "$TMPCONF"

# This will fail at pfctl (no sudo), but should pass anchor+conf validation
output=$(bash "$HELPER" activate "$TMPCONF" com.boxxy.zerobrew 2>&1) || true
if echo "$output" | grep -q "anchor must start with"; then
    echo "FAIL: valid anchor rejected"
    FAIL=$((FAIL + 1))
elif echo "$output" | grep -q "conf must be in"; then
    echo "FAIL: valid conf path rejected"
    FAIL=$((FAIL + 1))
else
    echo "PASS: valid anchor and conf pass validation"
    PASS=$((PASS + 1))
fi

rm -f "$TMPCONF"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
