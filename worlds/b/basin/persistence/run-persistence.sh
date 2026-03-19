#!/usr/bin/env bash
# run-persistence.sh — Generate boxxy complexity distance matrix and
# compute persistent homology via ripser on dgx-spark (a@10.1.10.153)
#
# Usage: ./run-persistence.sh [--dim N] [--threshold T]
# Defaults: --dim 2 --threshold 50
#
# Output: persistence/ directory with diagram, barcode, JSON results
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
SPARK_HOST="${SPARK_HOST:-dgx-spark}"
VENV_PYTHON="\$HOME/ripser-venv/bin/python3"
DIM="${1:-2}"
THRESHOLD="${2:-50}"

echo "basin/persistence: boxxy → ripser on ${SPARK_HOST}"
echo "  repo: ${REPO_ROOT}"
echo "  dim=${DIM} threshold=${THRESHOLD}"

# 1. Generate distance matrix locally
echo "=== Building complexity distance matrix ==="
python3 "${SCRIPT_DIR}/complexity-matrix.py" "${REPO_ROOT}" \
  > /tmp/boxxy-dm.csv \
  2>/tmp/boxxy-labels.txt

NFUNCS=$(wc -l < /tmp/boxxy-dm.csv | tr -d ' ')
echo "  ${NFUNCS} functions extracted"

# 2. SCP to spark
echo "=== Dispatching to ${SPARK_HOST} ==="
BRIDGE="${REPO_ROOT}/../ripser_plus_plus_bridge.py"
scp -q "$BRIDGE" "${SPARK_HOST}:/tmp/ripser_plus_plus_bridge.py"
scp -q /tmp/boxxy-dm.csv "${SPARK_HOST}:/tmp/boxxy-dm.csv"

# 3. Run ripser + persim on remote
ssh "$SPARK_HOST" "${VENV_PYTHON} /tmp/ripser_plus_plus_bridge.py \
  --input /tmp/boxxy-dm.csv --dim ${DIM} --threshold ${THRESHOLD} \
  --backend ripser" > "${SCRIPT_DIR}/boxxy-persistence-result.json" 2>/tmp/ripser-stderr.txt

cat /tmp/ripser-stderr.txt

# 4. Copy labels
cp /tmp/boxxy-func-labels.json "${SCRIPT_DIR}/boxxy-func-labels.json" 2>/dev/null || true

# 5. Extract summary
python3 -c "
import json, sys
with open('${SCRIPT_DIR}/boxxy-persistence-result.json') as f:
    r = json.load(f)
b = r.get('betti_numbers', [])
print(f'  Betti: {b}')
print(f'  Euler: {r.get(\"euler_characteristic\", \"?\")}')
print(f'  Pairs: {len(r.get(\"pairs\", []))}')
print(f'  Backend: {r.get(\"backend\", \"?\")}')
print(f'  Time: {r.get(\"computation_time_ms\", 0):.0f}ms')
# GF(3) trit
if len(b) >= 2:
    trit = 1 if b[0] > b[1] else (-1 if b[0] < b[1] else 0)
    roles = {1: 'PLUS/generator', -1: 'MINUS/validator', 0: 'ERGODIC/coordinator'}
    print(f'  GF(3) trit: {trit:+d} ({roles[trit]})')
"

echo "=== Results in ${SCRIPT_DIR}/ ==="
ls -lh "${SCRIPT_DIR}"/boxxy-persistence-*.{json,png} 2>/dev/null || echo "  (run with --viz for diagrams)"
