import { memo } from 'react'
import { NodeResizer, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import type { RFNode } from '../convert'
import { NodeBadge, usePlanClass } from './NodeBadge'
import { useTargetState } from './useTargetState'

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

  return (
    <div className={`node container ${planClass} ${targetState} ${dropState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={data.type.split('.')[0]}>
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
