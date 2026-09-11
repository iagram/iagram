import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import type { RFNode } from '../convert'
import { NodeBadge, usePlanClass } from './NodeBadge'

function ResourceNodeImpl({ id, data, selected }: NodeProps<RFNode>) {
  const entry = useStore((s) => s.rules?.entry(data.type))
  const icon = useStore((s) => s.rules?.iconFor(data.type, data.props))
  const planClass = usePlanClass(id)
  return (
    <div className={`node resource ${planClass} ${selected ? 'selected' : ''}`} data-type={data.type}>
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
