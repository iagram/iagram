import { memo } from 'react'
import { Handle, NodeResizer, Position, type NodeProps } from '@xyflow/react'
import { COMMON } from '../types'
import { useStore } from '../store'
import { captionOf, type RFNode } from '../convert'
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
        <Handle id="t" type="source" position={Position.Top} />
      <Handle id="r" type="source" position={Position.Right} />
      <Handle id="b" type="source" position={Position.Bottom} />
      <Handle id="l" type="source" position={Position.Left} />
        {data.step && <span className="step-badge">{data.step}</span>}
        <div className="text">{text || <span className="muted">{data.name}</span>}</div>
        </div>
    )
  }
  if (provider === COMMON) {
    const caption = typeof data.props.caption === 'string' ? data.props.caption : ''
    return (
      <div className={`node resource actor ${targetState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={provider}>
        <Handle id="t" type="source" position={Position.Top} />
      <Handle id="r" type="source" position={Position.Right} />
      <Handle id="b" type="source" position={Position.Bottom} />
      <Handle id="l" type="source" position={Position.Left} />
        {data.step && <span className="step-badge">{data.step}</span>}
        {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
        <div className="name" title={data.name}>
          {data.name}
        </div>
        <div className="label">{caption || (entry?.label ?? data.type)}</div>
        </div>
    )
  }
  const caption = captionOf(entry, data)
  return (
    <div className={`node resource ${planClass} ${targetState} ${selected ? 'selected' : ''}`} data-type={data.type} data-provider={provider}>
      <Handle id="t" type="source" position={Position.Top} />
      <Handle id="r" type="source" position={Position.Right} />
      <Handle id="b" type="source" position={Position.Bottom} />
      <Handle id="l" type="source" position={Position.Left} />
      {data.step && <span className="step-badge">{data.step}</span>}
      {icon && <img src={`/icons/${icon}`} alt="" draggable={false} />}
      <div className="name" title={data.name}>
        {data.name}
      </div>
      <div className="label">{caption ? <i>{caption}</i> : entry?.label ?? data.type}</div>
      <NodeBadge id={id} />
    </div>
  )
}

export const ResourceNode = memo(ResourceNodeImpl)
