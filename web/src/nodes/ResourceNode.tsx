import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import type { RFNode } from '../convert'
import { NodeBadge, usePlanClass } from './NodeBadge'
import { useTargetState } from './useTargetState'

function ResourceNodeImpl({ id, data, selected }: NodeProps<RFNode>) {
  const entry = useStore((s) => s.rules?.entry(data.type))
  const icon = useStore((s) => s.rules?.iconFor(data.type, data.props))
  const planClass = usePlanClass(id)
  const targetState = useTargetState(id, data.type)
  useStore((s) => s.catalogVersion) // re-render when a generated entry arrives
  return (
    <div className={`node resource ${planClass} ${targetState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={data.type.split('.')[0]}>
      <Handle type="target" position={Position.Left} />
      {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
      <div className="name" title={data.name}>
        {data.name}
      </div>
      <div className="label">{entry?.label ?? data.type}</div>
      <NodeBadge id={id} />
      <Handle type="source" position={Position.Right} />
    </div>
  )
}

export const ResourceNode = memo(ResourceNodeImpl)
