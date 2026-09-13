import { memo } from 'react'
import { BaseEdge, EdgeLabelRenderer, getSmoothStepPath, type EdgeProps } from '@xyflow/react'
import type { RFEdge } from '../convert'
import { usePlanClass } from '../nodes/NodeBadge'

const DASHES: Record<string, string | undefined> = { dashed: '7 5', dotted: '2 4', solid: undefined }

/**
 * The one edge type of the canvas. Draws the smooth-step path with the
 * edge's style (dash, colour, inactive), a step callout and a label.
 */
function IagramEdgeImpl({ id, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data, markerEnd, markerStart, selected }: EdgeProps<RFEdge>) {
  const planClass = usePlanClass(id)
  const [path, labelX, labelY] = getSmoothStepPath({ sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, borderRadius: 8 })
  const style = data?.style
  const stroke = style?.color || undefined
  const showLabel = data?.showLabel !== false
  const label = data?.label ?? ''
  const step = data?.step ?? ''
  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        markerEnd={markerEnd}
        markerStart={markerStart}
        className={`${style?.inactive ? 'inactive' : ''} ${planClass}`}
        style={{ stroke, strokeDasharray: DASHES[style?.dash ?? 'solid'], strokeWidth: selected ? 2 : undefined }}
      />
      {(step || (showLabel && label)) && (
        <EdgeLabelRenderer>
          <div className={`edge-label ${style?.inactive ? 'inactive' : ''}`} style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`, ...(stroke ? { borderColor: stroke } : {}) }}>
            {step && <span className="step">{step}</span>}
            {showLabel && label && <span className="text">{label}</span>}
          </div>
        </EdgeLabelRenderer>
      )}
    </>
  )
}

export const IagramEdge = memo(IagramEdgeImpl)
export const edgeTypes = { iagram: IagramEdge }
