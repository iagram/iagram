import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { useStore } from '../store'
import type { RFNode } from '../convert'
import { NodeBadge } from './NodeBadge'

function ResourceNodeImpl({ id, data, selected }: NodeProps<RFNode>) {
  const entry = useStore((s) => s.rules?.entry(data.type))
  return (
    <div className={`node resource ${selected ? 'selected' : ''}`} data-type={data.type}>
      <Handle type="target" position={Position.Left} />
      {entry?.icon && <img src={`/icons/${entry.icon}`} alt="" draggable={false} />}
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
