import {
  Tldraw,
  TLComponents,
  TLUiOverrides,
  TldrawEditor,
  createShapeId,
  TLShapeId,
  BaseBoxShapeUtil,
  HTMLContainer,
  RecordProps,
  T,
  DefaultColorStyle,
  TLBaseShape,
} from 'tldraw'
import 'tldraw/tldraw.css'

// -- $BOXXY Tile Shape --

type BoxxyTileProps = {
  w: number
  h: number
  bootMode: string    // 'efi' | 'linux' | 'macos'
  state: string       // 'none' | 'created' | 'installed' | 'running' | 'stopped'
  trit: number        // -1, 0, +1
  gayColor: string    // hex from seed 1069
  tileLabel: string
  configHash: string
  stage: string       // MINT, COMPOSE, PARALLEL, SWAP, BOOT, ATTEST, TRIAD, SNIPE, BRIDGE, REGRET, PINHOLE, ISOTOPY
}

type BoxxyTileShape = TLBaseShape<'boxxy-tile', BoxxyTileProps>

const STAGE_COLORS: Record<string, string> = {
  MINT:     '#E82D4A',
  COMPOSE:  '#D06546',
  PARALLEL: '#2BBDA5',
  SWAP:     '#C9A82B',
  BOOT:     '#4B35CC',
  ATTEST:   '#1DB854',
  TRIAD:    '#76B0F0',
  SNIPE:    '#E84B89',
  BRIDGE:   '#CAC828',
  REGRET:   '#E86E4B',
  PINHOLE:  '#1D9E7E',
  ISOTOPY:  '#A76BF0',
}

const STATE_LABELS: Record<string, string> = {
  none:      '○',
  created:   '◐',
  installed: '◑',
  running:   '●',
  stopped:   '◌',
}

const TRIT_LABELS: Record<number, string> = {
  [-1]: '−1',
  [0]:  ' 0',
  [1]:  '+1',
}

class BoxxyTileUtil extends BaseBoxShapeUtil<BoxxyTileShape> {
  static override type = 'boxxy-tile' as const

  static override props: RecordProps<BoxxyTileProps> = {
    w: T.number,
    h: T.number,
    bootMode: T.string,
    state: T.string,
    trit: T.number,
    gayColor: T.string,
    tileLabel: T.string,
    configHash: T.string,
    stage: T.string,
  }

  getDefaultProps(): BoxxyTileProps {
    return {
      w: 200,
      h: 140,
      bootMode: 'linux',
      state: 'none',
      trit: 0,
      gayColor: '#a855f7',
      tileLabel: 'VM Tile',
      configHash: '',
      stage: 'MINT',
    }
  }

  component(shape: BoxxyTileShape) {
    const { bootMode, state, trit, gayColor, tileLabel, stage } = shape.props
    const stageColor = STAGE_COLORS[stage] || gayColor

    return (
      <HTMLContainer id={shape.id}>
        <div style={{
          width: '100%',
          height: '100%',
          background: `linear-gradient(135deg, ${stageColor}22, ${stageColor}44)`,
          border: `2px solid ${stageColor}`,
          borderRadius: 8,
          display: 'flex',
          flexDirection: 'column',
          padding: 8,
          fontFamily: 'monospace',
          color: '#fafafa',
          position: 'relative',
          overflow: 'hidden',
        }}>
          {/* Header */}
          <div style={{
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
            marginBottom: 4,
          }}>
            <span style={{ fontSize: 11, fontWeight: 700, color: stageColor }}>
              {stage}
            </span>
            <span style={{ fontSize: 10, opacity: 0.7 }}>
              GF(3): {TRIT_LABELS[trit]}
            </span>
          </div>

          {/* Title */}
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
            {tileLabel}
          </div>

          {/* State + Boot */}
          <div style={{
            display: 'flex',
            justifyContent: 'space-between',
            fontSize: 10,
            opacity: 0.8,
          }}>
            <span>{STATE_LABELS[state] || '?'} {state}</span>
            <span>{bootMode}</span>
          </div>

          {/* Wire anchors (top 3 dots, bottom 3 dots) */}
          <div style={{
            position: 'absolute',
            top: -4,
            left: '50%',
            transform: 'translateX(-50%)',
            display: 'flex',
            gap: 12,
          }}>
            {[0, 1, 2].map(i => (
              <div key={`in-${i}`} style={{
                width: 6, height: 6,
                borderRadius: '50%',
                background: stageColor,
                border: '1px solid #fafafa',
              }} />
            ))}
          </div>
          <div style={{
            position: 'absolute',
            bottom: -4,
            left: '50%',
            transform: 'translateX(-50%)',
            display: 'flex',
            gap: 12,
          }}>
            {[0, 1, 2].map(i => (
              <div key={`out-${i}`} style={{
                width: 6, height: 6,
                borderRadius: '50%',
                background: stageColor,
                border: '1px solid #fafafa',
              }} />
            ))}
          </div>

          {/* Gay.jl color swatch */}
          <div style={{
            position: 'absolute',
            bottom: 8,
            right: 8,
            width: 16,
            height: 16,
            borderRadius: 4,
            background: gayColor,
            border: '1px solid rgba(255,255,255,0.3)',
          }} />
        </div>
      </HTMLContainer>
    )
  }

  indicator(shape: BoxxyTileShape) {
    const stageColor = STAGE_COLORS[shape.props.stage] || shape.props.gayColor
    return (
      <rect
        width={shape.props.w}
        height={shape.props.h}
        rx={8}
        stroke={stageColor}
        strokeWidth={2}
        fill="none"
      />
    )
  }
}

// -- Helper: create tile shapes for each $BOXXY stage --

function createLifecycleDemo(editor: TldrawEditor) {
  const stages = [
    { stage: 'MINT',     label: '(vz/new-vm config)',         boot: 'efi',   trit: 1,  state: 'none' },
    { stage: 'COMPOSE',  label: 'tile-1 ; tile-2',            boot: 'linux', trit: 0,  state: 'created' },
    { stage: 'PARALLEL', label: 'tile-1 ⊗ tile-2',            boot: 'linux', trit: -1, state: 'created' },
    { stage: 'SWAP',     label: 'σ(tile-1, tile-2)',           boot: 'efi',   trit: 1,  state: 'installed' },
    { stage: 'BOOT',     label: '(vz/start-vm! vm)',           boot: 'macos', trit: 0,  state: 'running' },
    { stage: 'ATTEST',   label: 'sideref(skill, device)',     boot: 'linux', trit: -1, state: 'running' },
    { stage: 'TRIAD',    label: 'gen(+1) coord(0) verif(-1)', boot: 'linux', trit: 0,  state: 'running' },
    { stage: 'SNIPE',    label: 'vibesnipe_selection(vs)',     boot: 'efi',   trit: 1,  state: 'running' },
    { stage: 'BRIDGE',   label: 'submit_bridge(WL-2, WL-7)',  boot: 'linux', trit: 0,  state: 'running' },
    { stage: 'REGRET',   label: '$REGRET ↔ $BOXXY exchange',  boot: 'efi',   trit: -1, state: 'stopped' },
    { stage: 'PINHOLE',  label: 'pinhole :8080 → guest',      boot: 'linux', trit: 1,  state: 'running' },
    { stage: 'ISOTOPY',  label: 'tiling-A ≅ tiling-B',        boot: 'efi',   trit: 0,  state: 'stopped' },
  ]

  const cols = 4
  const tileW = 220
  const tileH = 160
  const gap = 40

  stages.forEach((s, i) => {
    const col = i % cols
    const row = Math.floor(i / cols)
    editor.createShape({
      id: createShapeId(`tile-${s.stage.toLowerCase()}`),
      type: 'boxxy-tile',
      x: col * (tileW + gap) + 100,
      y: row * (tileH + gap) + 100,
      props: {
        w: tileW,
        h: tileH,
        bootMode: s.boot,
        state: s.state,
        trit: s.trit,
        gayColor: STAGE_COLORS[s.stage],
        tileLabel: s.label,
        configHash: '',
        stage: s.stage,
      },
    })
  })

  // Sequential composition arrows: MINT → COMPOSE, COMPOSE → PARALLEL
  const arrowPairs = [
    ['tile-mint', 'tile-compose'],
    ['tile-compose', 'tile-parallel'],
    ['tile-boot', 'tile-attest'],
    ['tile-attest', 'tile-triad'],
    ['tile-triad', 'tile-snipe'],
    ['tile-snipe', 'tile-bridge'],
    ['tile-bridge', 'tile-regret'],
  ]

  arrowPairs.forEach(([from, to], i) => {
    editor.createShape({
      id: createShapeId(`wire-${i}`),
      type: 'arrow',
      props: {
        start: { type: 'binding', boundShapeId: createShapeId(from), normalizedAnchor: { x: 0.5, y: 1 } },
        end: { type: 'binding', boundShapeId: createShapeId(to), normalizedAnchor: { x: 0.5, y: 0 } },
      },
    })
  })
}

// -- Main Canvas Component --

export function BoxxyCanvas() {
  return (
    <div style={{ position: 'fixed', inset: 0, background: '#09090b' }}>
      <Tldraw
        shapeUtils={[BoxxyTileUtil]}
        onMount={(editor) => {
          createLifecycleDemo(editor)
        }}
      />
    </div>
  )
}

export default BoxxyCanvas
