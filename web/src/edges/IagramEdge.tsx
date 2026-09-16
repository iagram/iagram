import { memo, useMemo } from 'react'
import { BaseEdge, EdgeLabelRenderer, Position, getBezierPath, getSmoothStepPath, getStraightPath, useNodes, useStore as useRFStore, type EdgeProps } from '@xyflow/react'
import { pathFromPoints, pointAlong, routeOrthogonal, type Rect, type Side } from './router'
import { effectiveStyle, type RFEdge } from '../convert'
import { usePlanClass } from '../nodes/NodeBadge'

const DASHES: Record<string, string | undefined> = { dashed: '7 5', dotted: '2 4', solid: undefined }

/**
 * The one edge type of the canvas. Draws the smooth-step path with the
 * edge's style (dash, colour, inactive), a step callout and a label.
 */
const SIDE: Record<Position, Side> = { [Position.Top]: 't', [Position.Right]: 'r', [Position.Bottom]: 'b', [Position.Left]: 'l' }

function IagramEdgeImpl({ id, source, target, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, data, markerEnd, markerStart, selected }: EdgeProps<RFEdge>) {
  const planClass = usePlanClass(id)
  const style = effectiveStyle(data)
  const orthogonal = !style.curve || style.curve === 'step'
  // Every node change re-renders the edge so the route follows dragged elements.
  const nodes = useNodes()
  const nodeLookup = useRFStore((s) => s.nodeLookup)
  const routed = useMemo(() => {
    if (!orthogonal) return null
    const obstacles: Rect[] = []
    for (const n of nodeLookup.values()) {
      if (n.id === source || n.id === target || n.hidden) continue
      // icons block the way; boxes are crossed at their border like the vendor diagrams
      if (!(n.type === 'resource' || (n.data as { collapsed?: boolean } | undefined)?.collapsed)) continue
      const w = n.measured?.width ?? 0
      const h = n.measured?.height ?? 0
      if (!w || !h) continue
      const { x, y } = n.internals.positionAbsolute
      obstacles.push({ x, y, w, h })
    }
    return routeOrthogonal({ source: { x: sourceX, y: sourceY }, sourceSide: SIDE[sourcePosition], target: { x: targetX, y: targetY }, targetSide: SIDE[targetPosition], obstacles })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [orthogonal, nodes, nodeLookup, source, target, sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition])
  let path: string
  let labelX: number
  let labelY: number
  if (routed && routed.length >= 2) {
    path = pathFromPoints(routed, 8)
    const mid = pointAlong(routed, 0.5)
    labelX = mid.x
    labelY = mid.y
  } else {
    ;[path, labelX, labelY] =
      style.curve === 'straight'
        ? getStraightPath({ sourceX, sourceY, targetX, targetY })
        : style.curve === 'bezier'
          ? getBezierPath({ sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition })
          : getSmoothStepPath({ sourceX, sourceY, targetX, targetY, sourcePosition, targetPosition, borderRadius: 8 })
  }
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
        style={{ stroke, strokeDasharray: DASHES[style?.dash ?? 'solid'], strokeWidth: style?.width ? style.width + (selected ? 1 : 0) : selected ? 2 : undefined }}
      />
      {(step || (showLabel && label)) && (
        <EdgeLabelRenderer>
          <div className={`edge-label ${style?.inactive ? 'inactive' : ''} ${step && showLabel && label ? 'has-step' : ''} ${selected ? 'selected' : ''}`} style={{ transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`, ...(stroke ? { borderColor: stroke } : {}) }}>
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
