import { memo } from 'react'
import { Handle, NodeResizer, Position, type NodeProps } from '@xyflow/react'
import { COMMON } from '../types'
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
  const provider = data.type.split('.')[0]
  if (data.type === 'common.note') {
    const text = typeof data.props.text === 'string' ? data.props.text : ''
    return (
      <div className={`node note ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={provider}>
        <NodeResizer isVisible={selected} minWidth={120} minHeight={60} lineClassName="resizer-line" handleClassName="resizer-handle" />
        <Handle type="target" position={Position.Left} />
        <div className="text">{text || <span className="muted">{data.name}</span>}</div>
        <Handle type="source" position={Position.Right} />
      </div>
    )
  }
  if (provider === COMMON) {
    const caption = typeof data.props.caption === 'string' ? data.props.caption : ''
    return (
      <div className={`node resource actor ${targetState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={provider}>
        <Handle type="target" position={Position.Left} />
        {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
        <div className="name" title={data.name}>
          {data.name}
        </div>
        <div className="label">{caption || (entry?.label ?? data.type)}</div>
        <Handle type="source" position={Position.Right} />
      </div>
    )
  }
  return (
    <div className={`node resource ${planClass} ${targetState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={provider}>
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
