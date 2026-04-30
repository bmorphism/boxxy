#!/usr/bin/env bash
# cross-format-embed.sh — Compare Gemini embeddings of frames extracted from GIF vs MP4
#
# Extracts N keyframes from both formats as PNG (normalizing the container),
# then embeds each pair at 3 matryoshka dims and measures divergence.
# This tells us whether pixel-level Gärdenfors equivalence holds at the semantic level.
#
# Usage: ./scripts/cross-format-embed.sh [--dims 768,1536] [--frames 5]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BOXXY_DIR="$(dirname "$SCRIPT_DIR")"
cd "$BOXXY_DIR"

API_KEY="${GEMINI_API_KEY:-${GOOGLE_API_KEY:-${GOOGLE_GENERATIVE_AI_API_KEY:-}}}"
if [ -z "$API_KEY" ]; then
  echo "ERROR: No Gemini API key. Set GEMINI_API_KEY." >&2; exit 1
fi

MODEL="gemini-embedding-2"
BASE_URL="https://generativelanguage.googleapis.com/v1beta/models/${MODEL}:embedContent"
N_FRAMES=4
DIMS="768 1536"

while [ $# -gt 0 ]; do
  case "$1" in
    --dims) DIMS="${2//,/ }"; shift 2 ;;
    --frames) N_FRAMES="$2"; shift 2 ;;
    *) shift ;;
  esac
done

PAIRS=(
  "parallel-spi"
  "tile-self-awareness"
  "triangulate-contrast"
)

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

embed_call() {
  local payload_file="$1" dim="$2" label="$3"
  local pfile="${TMPDIR}/_embed_p.json"
  jq --argjson dim "$dim" '. + {outputDimensionality: $dim}' < "$payload_file" > "$pfile"
  local rfile="${TMPDIR}/_embed_r.json"
  curl -s -X POST "${BASE_URL}?key=${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "@${pfile}" --max-time 60 > "$rfile"
  local err
  err=$(jq -r '.error.message // empty' < "$rfile")
  if [ -n "$err" ]; then
    echo "  [$label dim=$dim] ERROR: $err" >&2
    echo "null"
    return
  fi
  jq '.embedding.values' < "$rfile"
}

cosine_sim() {
  local fa="$1" fb="$2"
  python3 -c "
import json, math
a=json.load(open('$fa')); b=json.load(open('$fb'))
if a is None or b is None: print('null'); exit()
d=sum(x*y for x,y in zip(a,b))
na=math.sqrt(sum(x*x for x in a)); nb=math.sqrt(sum(x*x for x in b))
print(f'{d/(na*nb) if na>0 and nb>0 else 0:.8f}')
"
}

extract_frames() {
  local src="$1" prefix="$2" n="$3"
  local dur
  dur=$(ffprobe -v quiet -show_entries format=duration -of csv=p=0 "$src" | head -1)
  dur=$(python3 -c "print(max(float('${dur:-0.5}'), 0.1))")
  for i in $(seq 0 $((n-1))); do
    local t
    t=$(python3 -c "d=$dur; t=d*$i/max($n-1,1); t=min(t,d-0.05) if t>d-0.05 else t; print(f'{max(0,t):.3f}')")
    ffmpeg -y -v quiet -ss "$t" -i "$src" -frames:v 1 "${TMPDIR}/${prefix}_${i}.png" 2>/dev/null || true
  done
}

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  cross-format-embed: GIF vs MP4 Embedding Comparison       ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  Dims:      $DIMS"
echo "║  Frames:    $N_FRAMES per format"
echo "║  Pairs:     ${#PAIRS[@]}"
echo "╚══════════════════════════════════════════════════════════════╝"
echo

RESULTS="[]"

for TAPE in "${PAIRS[@]}"; do
  GIF="tapes/gifs/${TAPE}.gif"
  MP4="tapes/gifs/${TAPE}.mp4"
  if [ ! -f "$GIF" ] || [ ! -f "$MP4" ]; then
    echo "SKIP $TAPE: missing file(s)" >&2
    continue
  fi

  echo "━━━ $TAPE ━━━"
  echo "  Extracting $N_FRAMES frames from GIF..."
  extract_frames "$GIF" "gif_${TAPE}" "$N_FRAMES"
  echo "  Extracting $N_FRAMES frames from MP4..."
  extract_frames "$MP4" "mp4_${TAPE}" "$N_FRAMES"

  PAIR_RESULT="{\"tape\": \"$TAPE\", \"frames\": []}"

  for i in $(seq 0 $((N_FRAMES-1))); do
    gif_png="${TMPDIR}/gif_${TAPE}_${i}.png"
    mp4_png="${TMPDIR}/mp4_${TAPE}_${i}.png"
    if [ ! -f "$gif_png" ] || [ ! -f "$mp4_png" ]; then
      echo "  Frame $i: extraction failed, skipping"
      continue
    fi

    # Encode both as base64
    base64 -i "$gif_png" | tr -d '\n' > "${TMPDIR}/gif_f${i}.b64"
    base64 -i "$mp4_png" | tr -d '\n' > "${TMPDIR}/mp4_f${i}.b64"

    # Build payloads (both PNG — normalizing the container)
    jq -n --rawfile b64 "${TMPDIR}/gif_f${i}.b64" \
      '{content:{parts:[{inlineData:{mimeType:"image/png",data:$b64}}]}}' > "${TMPDIR}/gif_pay.json"
    jq -n --rawfile b64 "${TMPDIR}/mp4_f${i}.b64" \
      '{content:{parts:[{inlineData:{mimeType:"image/png",data:$b64}}]}}' > "${TMPDIR}/mp4_pay.json"

    FRAME_RESULT="{\"frame\": $i, \"dims\": {}}"

    for DIM in $DIMS; do
      # Embed GIF-sourced frame
      embed_call "${TMPDIR}/gif_pay.json" "$DIM" "gif_f$i" > "${TMPDIR}/gif_e_${i}_${DIM}.json"
      # Embed MP4-sourced frame
      embed_call "${TMPDIR}/mp4_pay.json" "$DIM" "mp4_f$i" > "${TMPDIR}/mp4_e_${i}_${DIM}.json"

      sim=$(cosine_sim "${TMPDIR}/gif_e_${i}_${DIM}.json" "${TMPDIR}/mp4_e_${i}_${DIM}.json")
      divergence=$(python3 -c "s=$sim if '$sim'!='null' else 0; print(f'{1-s:.8f}')" 2>/dev/null || echo "null")

      echo "  Frame $i dim=$DIM: cosine=$sim divergence=$divergence"

      FRAME_RESULT=$(echo "$FRAME_RESULT" | jq --arg d "$DIM" --arg s "$sim" --arg dv "$divergence" \
        '.dims[$d] = {"cosine": ($s|tonumber? // null), "divergence": ($dv|tonumber? // null)}')
    done

    PAIR_RESULT=$(echo "$PAIR_RESULT" | jq --argjson fr "$FRAME_RESULT" '.frames += [$fr]')
  done

  # Aggregate per-pair
  for DIM in $DIMS; do
    mean_sim=$(echo "$PAIR_RESULT" | jq --arg d "$DIM" \
      '[.frames[].dims[$d].cosine // empty] | if length>0 then add/length else null end')
    mean_div=$(echo "$PAIR_RESULT" | jq --arg d "$DIM" \
      '[.frames[].dims[$d].divergence // empty] | if length>0 then add/length else null end')
    echo "  ── dim=$DIM mean_cosine=$mean_sim mean_divergence=$mean_div"
  done

  RESULTS=$(echo "$RESULTS" | jq --argjson pr "$PAIR_RESULT" '. += [$pr]')
  echo
done

# Write final results
OUT="tapes/gifs/cross-format-embed.json"
echo "$RESULTS" | jq '.' > "$OUT"
echo "Results written to $OUT"

# Final summary
echo
echo "=== CROSS-FORMAT EMBEDDING SUMMARY ==="
for DIM in $DIMS; do
  global_mean=$(echo "$RESULTS" | jq --arg d "$DIM" \
    '[.[].frames[].dims[$d].cosine // empty] | if length>0 then add/length else null end')
  echo "  dim=$DIM global_mean_cosine=$global_mean"
done
echo "  Interpretation: cosine > 0.95 → same conceptual representation"
echo "                  cosine 0.80-0.95 → similar but distinguishable"
echo "                  cosine < 0.80 → different conceptual spaces"
