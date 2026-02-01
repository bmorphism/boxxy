#!/bin/bash
# Build Isabelle theories for boxxy
# 
# Prerequisites:
#   brew install --cask isabelle
# OR
#   Download from https://isabelle.in.tum.de/

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
THEORIES_DIR="$SCRIPT_DIR/../theories"

# Find Isabelle
ISABELLE=""
for candidate in \
    "/Applications/Isabelle2024.app/Isabelle/bin/isabelle" \
    "/Applications/Isabelle2023.app/Isabelle/bin/isabelle" \
    "$HOME/isabelle/bin/isabelle" \
    "$(which isabelle 2>/dev/null)"; do
    if [[ -x "$candidate" ]]; then
        ISABELLE="$candidate"
        break
    fi
done

if [[ -z "$ISABELLE" ]]; then
    echo "❌ Isabelle not found. Install with:"
    echo "   brew install --cask isabelle"
    echo ""
    echo "Or download from: https://isabelle.in.tum.de/"
    echo ""
    echo "Alternatively, verify theories manually in Proof General:"
    echo "   emacs $THEORIES_DIR/AGM_Base.thy"
    exit 1
fi

echo "🔧 Using Isabelle: $ISABELLE"
echo "📁 Theories: $THEORIES_DIR"

# Build
cd "$THEORIES_DIR"
"$ISABELLE" build -D . -v

echo "✅ Build complete"
