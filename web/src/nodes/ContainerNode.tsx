import { memo } from 'react'
import { NodeResizer, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import type { RFNode } from '../convert'
import { NodeBadge } from './NodeBadge'

function ContainerNodeImpl({ id, data, selected }: NodeProps<RFNode>) {
  const entry = useStore((s) => s.rules?.entry(data.type))
  const dragging = useStore((s) => s.draggingType)
  const rules = useStore((s) => s.rules)
  const dropState = dragging && rules ? (rules.canContain(data.type, dragging) ? 'valid' : 'invalid') : ''

  return (
    <div className={`node container ${dropState} ${selected ? 'selected' : ''}`} data-type={data.type}>
      <NodeResizer isVisible={selected} minWidth={200} minHeight={120} lineClassName="resizer-line" handleClassName="resizer-handle" />
      <header>
        {entry?.icon && <img src={`/icons/${entry.icon}`} alt="" draggable={false} />}
        <span className="label">{entry?.label ?? data.type}</span>
        <span className="name">{data.name}</span>
        <NodeBadge id={id} />
      </header>
    </div>
  )
}

export const ContainerNode = memo(ContainerNodeImpl)
