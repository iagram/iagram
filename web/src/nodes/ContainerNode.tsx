import { memo, useMemo, type CSSProperties } from 'react'
import type { BoxStyle, Entry } from '../types'
import { NodeResizer, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import type { RFNode } from '../convert'
import { NodeBadge, usePlanClass } from './NodeBadge'
import { useTargetState } from './useTargetState'

// styleOf resolves the entry style, applying the first style_variant whose
// boolean property is true. Computed outside the store selector: a merged
// object per call would re-render forever.
function styleOf(entry: Entry | undefined, props: Record<string, unknown>): BoxStyle | undefined {
  if (!entry) return undefined
  for (const [prop, style] of Object.entries(entry.style_variants ?? {})) {
    if (props[prop] === true) return { ...entry.style, ...style }
  }
  return entry.style
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
  const dropState = dragging && rules ? (rules.canContain(data.type, dragging) ? 'valid' : 'invalid') : ''

  const style = useMemo(() => styleOf(entry, data.props), [entry, data.props])
  const css: Record<string, string> = {}
  if (style?.border) {
    css['--box-border'] = style.border
    css['--box-fill'] = style.fill ?? `color-mix(in srgb, ${style.border} 5%, transparent)`
  }
  if (style?.dash) css['--box-dash'] = style.dash
  return (
    <div
      className={`node container ${planClass} ${targetState} ${dropState} ${selected ? 'selected' : ''} ${style?.label === 'center' ? 'label-center' : ''}`}
      data-type={data.type}
      data-provider={data.type.split('.')[0]}
      style={css as CSSProperties}
    >
      <NodeResizer isVisible={selected} onResizeStart={() => commit()} minWidth={200} minHeight={120} lineClassName="resizer-line" handleClassName="resizer-handle" />
      <header>
        {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
        <span className="label">{entry?.label ?? data.type}</span>
        <span className="name">{data.name}</span>
        <NodeBadge id={id} />
      </header>
    </div>
  )
}

export const ContainerNode = memo(ContainerNodeImpl)
