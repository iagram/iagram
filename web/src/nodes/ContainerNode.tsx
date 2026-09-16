import { memo, useMemo, type CSSProperties } from 'react'
import type { BoxStyle, Entry } from '../types'
import { Handle, NodeResizer, Position, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import { captionOf, type RFNode } from '../convert'
import { NodeBadge, usePlanClass } from './NodeBadge'
import { useTargetState } from './useTargetState'

// styleOf resolves the entry style, applying the first style_variant whose
// boolean property is true. Computed outside the store selector: a merged
// object per call would re-render forever.
function styleOf(entry: Entry | undefined, props: Record<string, unknown>): BoxStyle | undefined {
  if (!entry) return undefined
  let style = entry.style
  for (const [prop, variant] of Object.entries(entry.style_variants ?? {})) {
    if (props[prop] === true) style = { ...style, ...variant }
  }
  // A group's own colour property wins over the catalog colour.
  if (typeof props.color === 'string' && props.color.trim()) style = { ...style, border: props.color.trim() }
  return style
}

function ContainerNodeImpl({ id, data, selected }: NodeProps<RFNode>) {
  const entry = useStore((s) => s.rules?.entry(data.type))
  const icon = useStore((s) => s.rules?.iconFor(data.type, data.props))
  const planClass = usePlanClass(id)
  const targetState = useTargetState(id, data.type)
  useStore((s) => s.catalogVersion) // re-render when a generated entry arrives
  const dragging = useStore((s) => s.draggingType)
  const rules = useStore((s) => s.rules)
  const commit = useStore((s) => s.commit)
  const syncSpans = useStore((s) => s.syncSpans)
  const span = entry?.span
  const ghosts = span?.ghost ? Math.min(8, Math.max(0, Number(data.props[span.ghost.count] ?? data.props.min_size ?? 0) || 0)) : 0
  const dropState = dragging && rules ? (rules.canContain(data.type, dragging) ? 'valid' : 'invalid') : ''

  const style = useMemo(() => styleOf(entry, data.props), [entry, data.props])
  if (data.collapsed) {
    // Composite service with nothing inside: the icon look of a resource; it
    // becomes a box as soon as a component is dropped in.
    return (
      <div className={`node resource composite ${planClass} ${targetState} ${dropState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={data.type.split('.')[0]} title="Drop components inside to open the box">
        <Handle id="t" type="source" position={Position.Top} />
        <Handle id="r" type="source" position={Position.Right} />
        <Handle id="b" type="source" position={Position.Bottom} />
        <Handle id="l" type="source" position={Position.Left} />
        {data.step && <span className="step-badge">{data.step}</span>}
        {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
        <div className="name" title={data.name}>
          {data.name}
        </div>
        <div className="label">{captionOf(entry, data) ? <i>{captionOf(entry, data)}</i> : entry?.label ?? data.type}</div>
        <NodeBadge id={id} />
      </div>
    )
  }
  const css: Record<string, string> = {}
  if (style?.border) {
    css['--box-border'] = style.border
    css['--box-fill'] = style.fill ?? `color-mix(in srgb, ${style.border} 5%, transparent)`
  }
  if (style?.dash) css['--box-dash'] = style.dash
  return (
    <div
      className={`node container ${planClass} ${targetState} ${dropState} ${selected ? 'selected' : ''} ${style?.label === 'center' ? 'label-center' : ''} ${span ? 'span' : ''}`}
      data-type={data.type}
      data-provider={data.type.split('.')[0]}
      style={css as CSSProperties}
    >
      <NodeResizer isVisible={selected} onResizeStart={() => commit()} onResizeEnd={() => span && syncSpans()} minWidth={200} minHeight={span ? 80 : 120} lineClassName="resizer-line" handleClassName="resizer-handle" />
      <Handle id="t" type="source" position={Position.Top} />
      <Handle id="r" type="source" position={Position.Right} />
      <Handle id="b" type="source" position={Position.Bottom} />
      <Handle id="l" type="source" position={Position.Left} />
      <header>
        {data.step && <span className="step-badge">{data.step}</span>}
        {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
        <span className="label">{entry?.label ?? data.type}</span>
        <span className="name">{data.name}</span>
        {captionOf(entry, data) && <span className="caption">{captionOf(entry, data)}</span>}
        <NodeBadge id={id} />
      </header>
      {ghosts > 0 && span?.ghost && (
        <div className="ghosts" title={`${ghosts} instances (capacity)`}>
          {Array.from({ length: ghosts }, (_, i) => (
            <img key={i} src={`/icons/${span.ghost!.icon}`} alt="" draggable={false} />
          ))}
        </div>
      )}
    </div>
  )
}

export const ContainerNode = memo(ContainerNodeImpl)
