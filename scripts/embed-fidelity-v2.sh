#!/usr/bin/env bash
# embed-fidelity-v2.sh — Creative multi-axis Gemini embedding-2 probing
# Uses REST API directly. No SDK dependency hell.
#
# 5 creative query axes:
#   1. Raw video embedding (full mp4 inline)
#   2. Keyframe snapshots (embed individual frames as images)
#   3. Contrastive text prompts ("color identity" vs "behavioral trace")
#   4. Frame-delta embedding (diff between first and last frame)
#   5. Cross-modal: embed text description AGAINST video, measure alignment
#
# Each axis probed at dims: 768, 1536, 3072
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BOXXY_DIR="$(dirname "$SCRIPT_DIR")"
cd "$BOXXY_DIR"

API_KEY="${GEMINI_API_KEY:-${GOOGLE_API_KEY:-${GOOGLE_GENERATIVE_AI_API_KEY:-}}}"
if [ -z "$API_KEY" ]; then
  echo "ERROR: No Gemini API key found" >&2; exit 1
fi

MODEL="gemini-embedding-2"
BASE_URL="https://generativelanguage.googleapis.com/v1beta/models/${MODEL}:embedContent"

VIDEO_A="tapes/gifs/tile-self-awareness.mp4"
VIDEO_B="tapes/gifs/triangulate-contrast.mp4"
TMPDIR=$(mktemp -d)
RESULTS="tapes/gifs/embedding-fidelity-v2.json"

trap "rm -rf $TMPDIR" EXIT

echo "=== Gemini embedding-2: Multi-Axis Fidelity Probe ==="
echo "Video A: $VIDEO_A (seed 1069, self-awareness loop)"
echo "Video B: $VIDEO_B (seed 42, triangulate contrast)"
echo "Temp: $TMPDIR"
echo

# --- Helper: call embedding API ---
embed_call() {
  local payload="$1"
  local dim="$2"
  local label="$3"

  # Inject outputDimensionality into the payload
  local full_payload
  full_payload=$(echo "$payload" | jq --argjson dim "$dim" '. + {outputDimensionality: $dim}')

  local resp
  resp=$(curl -s -X POST "${BASE_URL}?key=${API_KEY}" \
    -H "Content-Type: application/json" \
    -d "$full_payload" \
    --max-time 120)

  # Check for error
  local err
  err=$(echo "$resp" | jq -r '.error.message // empty')
  if [ -n "$err" ]; then
    echo "  [$label dim=$dim] ERROR: $err" >&2
    echo "null"
    return
  fi

  echo "$resp" | jq '.embedding.values'
}

# --- Helper: cosine similarity via jq ---
cosine_sim() {
  local vec_a="$1"
  local vec_b="$2"

  if [ "$vec_a" = "null" ] || [ "$vec_b" = "null" ]; then
    echo "null"
    return
  fi

  python3 -c "
import json, math, sys
a = json.loads('''$vec_a''')
b = json.loads('''$vec_b''')
dot = sum(x*y for x,y in zip(a,b))
na = math.sqrt(sum(x*x for x in a))
nb = math.sqrt(sum(x*x for x in b))
sim = dot/(na*nb) if na>0 and nb>0 else 0
print(f'{sim:.8f}')
"
}

# --- Helper: bifurcation = 1 - cosine_sim ---
bifurc() {
  local sim="$1"
  if [ "$sim" = "null" ]; then echo "null"; return; fi
  python3 -c "print(f'{1.0 - $sim:.8f}')"
}

echo '{"axes": [' > "$TMPDIR/results.json"
FIRST_AXIS=true

add_axis_result() {
  local axis_name="$1"
  local json_data="$2"
  if [ "$FIRST_AXIS" = "false" ]; then
    echo "," >> "$TMPDIR/results.json"
  fi
  FIRST_AXIS=false
  echo "$json_data" >> "$TMPDIR/results.json"
}

######################################################################
# AXIS 1: Raw video embedding (full mp4 inline as base64)
######################################################################
echo ">>> AXIS 1: Raw Video Embedding (full mp4)"
B64_A=$(base64 -i "$VIDEO_A")
B64_B=$(base64 -i "$VIDEO_B")

PAYLOAD_A=$(jq -n --arg b64 "$B64_A" '{
  content: {parts: [{inlineData: {mimeType: "video/mp4", data: $b64}}]}
}')
PAYLOAD_B=$(jq -n --arg b64 "$B64_B" '{
  content: {parts: [{inlineData: {mimeType: "video/mp4", data: $b64}}]}
}')

axis1='{"axis": "raw_video", "dims": ['
FIRST_DIM=true
for DIM in 768 1536 3072; do
  echo "  dim=$DIM ..."
  emb_a=$(embed_call "$PAYLOAD_A" "$DIM" "video_A")
  emb_b=$(embed_call "$PAYLOAD_B" "$DIM" "video_B")
  sim=$(cosine_sim "$emb_a" "$emb_b")
  bif=$(bifurc "$sim")
  echo "    cosine=$sim bifurc=$bif"
  [ "$FIRST_DIM" = "false" ] && axis1="$axis1,"
  FIRST_DIM=false
  axis1="$axis1{\"dim\":$DIM,\"cosine\":$sim,\"bifurcation\":$bif}"
done
axis1="$axis1]}"
add_axis_result "raw_video" "$axis1"

######################################################################
# AXIS 2: Keyframe snapshots (first, middle, last frame as PNG)
######################################################################
echo
echo ">>> AXIS 2: Keyframe Snapshots (3 frames per video)"

extract_frames() {
  local video="$1"
  local prefix="$2"
  local duration
  duration=$(ffprobe -v quiet -show_entries format=duration -of csv=p=0 "$video" | head -1)
  local mid
  mid=$(python3 -c "print(f'{float(\"$duration\")/2:.2f}')")

  ffmpeg -y -v quiet -ss 0 -i "$video" -frames:v 1 "${TMPDIR}/${prefix}_first.png"
  ffmpeg -y -v quiet -ss "$mid" -i "$video" -frames:v 1 "${TMPDIR}/${prefix}_mid.png"
  ffmpeg -y -v quiet -sseof -0.1 -i "$video" -frames:v 1 "${TMPDIR}/${prefix}_last.png"
}

extract_frames "$VIDEO_A" "a"
extract_frames "$VIDEO_B" "b"

# Embed each frame, compare corresponding frames across videos
axis2='{"axis": "keyframe_snapshots", "dims": ['
FIRST_DIM=true
for DIM in 768 1536 3072; do
  echo "  dim=$DIM ..."
  sims=""
  for FRAME in first mid last; do
    B64_FA=$(base64 -i "${TMPDIR}/a_${FRAME}.png")
    B64_FB=$(base64 -i "${TMPDIR}/b_${FRAME}.png")
    PA=$(jq -n --arg b64 "$B64_FA" '{
      content: {parts: [{inlineData: {mimeType: "image/png", data: $b64}}]}
    }')
    PB=$(jq -n --arg b64 "$B64_FB" '{
      content: {parts: [{inlineData: {mimeType: "image/png", data: $b64}}]}
    }')
    ea=$(embed_call "$PA" "$DIM" "frame_a_$FRAME")
    eb=$(embed_call "$PB" "$DIM" "frame_b_$FRAME")
    s=$(cosine_sim "$ea" "$eb")
    echo "    $FRAME: cosine=$s"
    sims="$sims $s"
  done
  # Average the 3 frame similarities
  avg=$(python3 -c "
vals = [float(x) for x in '$sims'.split() if x != 'null']
print(f'{sum(vals)/len(vals):.8f}' if vals else 'null')
")
  bif=$(bifurc "$avg")
  echo "    avg_cosine=$avg bifurc=$bif"
  [ "$FIRST_DIM" = "false" ] && axis2="$axis2,"
  FIRST_DIM=false
  axis2="$axis2{\"dim\":$DIM,\"avg_cosine\":$avg,\"bifurcation\":$bif}"
done
axis2="$axis2]}"
add_axis_result "keyframe_snapshots" "$axis2"

######################################################################
# AXIS 3: Contrastive text prompts (semantic lenses)
######################################################################
echo
echo ">>> AXIS 3: Contrastive Text Prompts (semantic lenses)"

PROMPTS=(
  "terminal recording showing deterministic color identity from seed 1069"
  "terminal recording showing deterministic color identity from seed 42"
  "REPL session demonstrating fibonacci computation with memoization"
  "behavioral triangulation of three virtual machines measuring Hamming distance"
  "rainbow parentheses in a Lisp REPL with OSC 8 hyperlinks"
)

axis3='{"axis": "text_prompts", "dims": ['
FIRST_DIM=true
for DIM in 768 1536 3072; do
  echo "  dim=$DIM ..."
  # Embed all prompts, then compute pairwise matrix
  embeddings=()
  for i in "${!PROMPTS[@]}"; do
    P=$(jq -n --arg txt "${PROMPTS[$i]}" '{content: {parts: [{text: $txt}]}}')
    e=$(embed_call "$P" "$DIM" "prompt_$i")
    embeddings+=("$e")
  done

  # Key comparison: prompt[0] (seed 1069) vs prompt[1] (seed 42)
  s01=$(cosine_sim "${embeddings[0]}" "${embeddings[1]}")
  # Cross-domain: prompt[0] vs prompt[3] (triangulation)
  s03=$(cosine_sim "${embeddings[0]}" "${embeddings[3]}")
  # Orthogonal: prompt[2] (fib) vs prompt[4] (rainbow)
  s24=$(cosine_sim "${embeddings[2]}" "${embeddings[4]}")

  echo "    seed1069_vs_seed42=$s01"
  echo "    identity_vs_triangulation=$s03"
  echo "    fib_vs_rainbow=$s24"

  [ "$FIRST_DIM" = "false" ] && axis3="$axis3,"
  FIRST_DIM=false
  axis3="$axis3{\"dim\":$DIM,\"seed_contrast\":$s01,\"identity_vs_triangulation\":$s03,\"fib_vs_rainbow\":$s24}"
done
axis3="$axis3]}"
add_axis_result "text_prompts" "$axis3"

######################################################################
# AXIS 4: Video + Prompt (multimodal: video grounded by text query)
######################################################################
echo
echo ">>> AXIS 4: Video + Prompt (multimodal grounding)"

GROUNDING_PROMPTS=(
  "What color identity does this seed produce?"
  "How does memoization affect behavioral traces?"
  "Are these two seeds semantically equivalent under REBEL?"
)

axis4='{"axis": "video_plus_prompt", "dims": ['
FIRST_DIM=true
for DIM in 768 1536 3072; do
  echo "  dim=$DIM ..."
  sims_per_prompt=""
  for GP in "${GROUNDING_PROMPTS[@]}"; do
    # Embed video A + prompt
    PA=$(jq -n --arg b64 "$B64_A" --arg txt "$GP" '{
      content: {parts: [
        {text: $txt},
        {inlineData: {mimeType: "video/mp4", data: $b64}}
      ]}
    }')
    # Embed video B + same prompt
    PB=$(jq -n --arg b64 "$B64_B" --arg txt "$GP" '{
      content: {parts: [
        {text: $txt},
        {inlineData: {mimeType: "video/mp4", data: $b64}}
      ]}
    }')
    ea=$(embed_call "$PA" "$DIM" "grounded_a")
    eb=$(embed_call "$PB" "$DIM" "grounded_b")
    s=$(cosine_sim "$ea" "$eb")
    echo "    '$GP' → cosine=$s"
    sims_per_prompt="$sims_per_prompt $s"
  done
  # Average across prompts
  avg=$(python3 -c "
vals = [float(x) for x in '$sims_per_prompt'.split() if x != 'null']
print(f'{sum(vals)/len(vals):.8f}' if vals else 'null')
")
  bif=$(bifurc "$avg")
  echo "    avg=$avg bifurc=$bif"
  [ "$FIRST_DIM" = "false" ] && axis4="$axis4,"
  FIRST_DIM=false
  axis4="$axis4{\"dim\":$DIM,\"avg_cosine\":$avg,\"bifurcation\":$bif}"
done
axis4="$axis4]}"
add_axis_result "video_plus_prompt" "$axis4"

######################################################################
# AXIS 5: Frame trajectory divergence (embed N frames, compare paths)
######################################################################
echo
echo ">>> AXIS 5: Frame Trajectory Divergence (10-frame paths)"

extract_trajectory() {
  local video="$1"
  local prefix="$2"
  local n=10
  local duration
  duration=$(ffprobe -v quiet -show_entries format=duration -of csv=p=0 "$video" | head -1)

  for i in $(seq 0 $((n-1))); do
    local t
    t=$(python3 -c "
d=float('$duration')
t=d * $i / ($n-1)
if t > d - 0.1: t = max(0, d - 0.1)
print(f'{t:.2f}')
")
    ffmpeg -y -v quiet -ss "$t" -i "$video" -frames:v 1 "${TMPDIR}/${prefix}_traj_${i}.png" 2>/dev/null || \
    ffmpeg -y -v quiet -sseof -0.1 -i "$video" -frames:v 1 "${TMPDIR}/${prefix}_traj_${i}.png" 2>/dev/null || true
  done
}

extract_trajectory "$VIDEO_A" "a"
extract_trajectory "$VIDEO_B" "b"

# At dim=1536 only (trajectory analysis is about shape, not dimensionality)
DIM=1536
echo "  dim=$DIM, comparing 10-frame trajectories ..."

# Embed all frames for both videos
traj_sims=""
for i in $(seq 0 9); do
  B64_FA=$(base64 -i "${TMPDIR}/a_traj_${i}.png")
  B64_FB=$(base64 -i "${TMPDIR}/b_traj_${i}.png")
  PA=$(jq -n --arg b64 "$B64_FA" '{
    content: {parts: [{inlineData: {mimeType: "image/png", data: $b64}}]}
  }')
  PB=$(jq -n --arg b64 "$B64_FB" '{
    content: {parts: [{inlineData: {mimeType: "image/png", data: $b64}}]}
  }')
  ea=$(embed_call "$PA" "$DIM" "traj_a_$i")
  eb=$(embed_call "$PB" "$DIM" "traj_b_$i")
  s=$(cosine_sim "$ea" "$eb")
  echo "    frame $i: cosine=$s"
  traj_sims="$traj_sims $s"
done

# Compute trajectory statistics
traj_stats=$(python3 -c "
import json
vals = [float(x) for x in '$traj_sims'.split() if x != 'null']
if not vals:
    print(json.dumps({'error': 'no data'}))
else:
    mean = sum(vals)/len(vals)
    variance = sum((v-mean)**2 for v in vals)/len(vals)
    import math
    std = math.sqrt(variance)
    # Where does max divergence occur? (frame index)
    min_sim = min(vals)
    max_div_frame = vals.index(min_sim)
    # Where is peak similarity?
    max_sim = max(vals)
    peak_frame = vals.index(max_sim)
    print(json.dumps({
        'mean_cosine': round(mean, 8),
        'std': round(std, 8),
        'min_cosine': round(min_sim, 8),
        'max_divergence_frame': max_div_frame,
        'max_cosine': round(max_sim, 8),
        'peak_similarity_frame': peak_frame,
        'frame_sims': [round(v, 6) for v in vals]
    }))
")
echo "  trajectory stats: $traj_stats"
axis5="{\"axis\": \"frame_trajectory\", \"dim\": $DIM, \"stats\": $traj_stats}"
add_axis_result "frame_trajectory" "$axis5"

######################################################################
# SUMMARY: Find the fidelity/decoherence boundary
######################################################################
echo
echo "]}" >> "$TMPDIR/results.json"
cp "$TMPDIR/results.json" "$RESULTS"

echo "=== FIDELITY BOUNDARY ANALYSIS ==="
python3 -c "
import json

with open('$RESULTS') as f:
    data = json.load(f)

print()
print('Axis-by-axis bifurcation across dimensions:')
print('=' * 65)

for axis in data['axes']:
    name = axis['axis']
    if 'dims' in axis:
        print(f'  {name}:')
        prev_bif = None
        for d in axis['dims']:
            dim = d.get('dim', '?')
            bif = d.get('bifurcation', d.get('avg_bifurcation', 'N/A'))
            if bif not in ('null', 'N/A', None):
                bif = float(bif)
                delta = ''
                if prev_bif is not None:
                    db = bif - prev_bif
                    arrow = '↑ MORE scrutiny' if db > 0 else '↓ LESS (decoherence)'
                    delta = f'  Δ={db:+.6f} {arrow}'
                print(f'    dim={dim}: bifurc={bif:.6f}{delta}')
                prev_bif = bif
            else:
                print(f'    dim={dim}: bifurc={bif}')
        print()
    elif 'stats' in axis:
        stats = axis['stats']
        print(f'  {name} (dim={axis.get(\"dim\",\"?\")}, trajectory):')
        if 'error' not in stats:
            print(f'    mean_cosine={stats[\"mean_cosine\"]:.6f} std={stats[\"std\"]:.6f}')
            print(f'    max_divergence at frame {stats[\"max_divergence_frame\"]} (cosine={stats[\"min_cosine\"]:.6f})')
            print(f'    peak_similarity at frame {stats[\"peak_similarity_frame\"]} (cosine={stats[\"max_cosine\"]:.6f})')
        print()

# Cross-axis decoherence boundary
print('DECOHERENCE BOUNDARY DETECTION:')
print('-' * 65)
for axis in data['axes']:
    if 'dims' not in axis:
        continue
    name = axis['axis']
    bifs = [(d['dim'], float(d.get('bifurcation', d.get('avg_bifurcation', 0)) or 0))
            for d in axis['dims']
            if d.get('bifurcation') not in ('null', None)]
    if len(bifs) >= 2:
        deltas = [(bifs[i][0], bifs[i+1][0], bifs[i+1][1] - bifs[i][1])
                  for i in range(len(bifs)-1)]
        # Decoherence = where bifurcation DROPS (higher dim collapses distinction)
        decohere = [d for d in deltas if d[2] < 0]
        scrutiny = [d for d in deltas if d[2] > 0]
        if decohere:
            d = max(decohere, key=lambda x: abs(x[2]))
            print(f'  {name}: decoherence at {d[0]}→{d[1]} (Δ={d[2]:+.6f})')
        elif scrutiny:
            d = max(scrutiny, key=lambda x: x[2])
            print(f'  {name}: NO decoherence — scrutiny increases through {d[1]} (Δ={d[2]:+.6f})')
        else:
            print(f'  {name}: flat — no signal')

print()
print(f'Full results: {\"$RESULTS\"}'  )
"

echo
echo "Done. Results in $RESULTS"
