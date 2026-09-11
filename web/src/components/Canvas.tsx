import { useCallback, useMemo, useRef, type DragEvent } from 'react'
import {
  Background,
  MiniMap,
  ReactFlow,
  useReactFlow,
  type IsValidConnection,
  type XYPosition,
} from '@xyflow/react'
import { useStore } from '../store'
import { ContainerNode } from '../nodes/ContainerNode'
import { ResourceNode } from '../nodes/ResourceNode'
import { DND_TYPE } from './Palette'
import { LEAF_H, LEAF_W, type RFEdge, type RFNode } from '../convert'
import { ROOT } from '../types'

const nodeTypes = { container: ContainerNode, resource: ResourceNode }

export function Canvas() {
  const nodes = useStore((s) => s.nodes)
  const edges = useStore((s) => s.edges)
  const rules = useStore((s) => s.rules)
  const onNodesChange = useStore((s) => s.onNodesChange)
  const onEdgesChange = useStore((s) => s.onEdgesChange)
  const connect = useStore((s) => s.connect)
  const addNode = useStore((s) => s.addNode)
  const select = useStore((s) => s.select)
  const setDragging = useStore((s) => s.setDragging)
  const commit = useStore((s) => s.commit)
  const reparent = useStore((s) => s.reparent)
  const showToast = useStore((s) => s.showToast)
  const problems = useStore((s) => s.problems)
  // Positions at drag start, to snap back when a drop target is not allowed.
  const dragStart = useRef<Map<string, { x: number; y: number }>>(new Map())
  const { screenToFlowPosition, getInternalNode } = useReactFlow<RFNode, RFEdge>()

  /** Absolute rect of a node, using React Flow's measured internals. */
  const absRect = useCallback(
    (id: string) => {
      const n = getInternalNode(id)
      if (!n) return null
      const w = n.measured?.width ?? (typeof n.style?.width === 'number' ? n.style.width : 0)
      const h = n.measured?.height ?? (typeof n.style?.height === 'number' ? n.style.height : 0)
      const { x, y } = n.internals.positionAbsolute
      return { x, y, w, h }
    },
    [getInternalNode],
  )

  const depthOf = useCallback(
    (id: string) => {
      let d = 0
      let cur = nodes.find((n) => n.id === id)
      while (cur?.parentId) {
        d++
        cur = nodes.find((n) => n.id === cur!.parentId)
      }
      return d
    },
    [nodes],
  )

  /** Deepest container whose rect contains the point. */
  const containerAt = useCallback(
    (p: XYPosition, exclude?: Set<string>): RFNode | null => {
      let best: RFNode | null = null
      let bestDepth = -1
      for (const n of nodes) {
        if (n.type !== 'container' || exclude?.has(n.id)) continue
        const r = absRect(n.id)
        if (!r) continue
        if (p.x >= r.x && p.x <= r.x + r.w && p.y >= r.y && p.y <= r.y + r.h) {
          const d = depthOf(n.id)
          if (d > bestDepth) {
            best = n
            bestDepth = d
          }
        }
      }
      return best
    },
    [nodes, absRect, depthOf],
  )

  const onDragOver = useCallback((ev: DragEvent) => {
    ev.preventDefault()
    ev.dataTransfer.dropEffect = 'move'
  }, [])

  const onDrop = useCallback(
    (ev: DragEvent) => {
      ev.preventDefault()
      setDragging(null)
      const type = ev.dataTransfer.getData(DND_TYPE)
      if (!type || !rules) return
      const entry = rules.entry(type)
      if (!entry) return
      const point = screenToFlowPosition({ x: ev.clientX, y: ev.clientY })
      const parent = containerAt(point)
      const size = entry.kind === 'container' ? entry.size ?? { w: 400, h: 300 } : { w: LEAF_W, h: LEAF_H }
      // Centre the new element on the cursor, relative to its parent.
      let pos = { x: point.x - size.w / 2, y: point.y - size.h / 2 }
      if (parent) {
        const r = absRect(parent.id)!
        pos = { x: Math.max(8, pos.x - r.x), y: Math.max(36, pos.y - r.y) }
      }
      addNode(type, parent?.id ?? null, pos)
    },
    [rules, screenToFlowPosition, containerAt, absRect, addNode, setDragging],
  )

  /** The node and everything inside it. */
  const subtree = useCallback(
    (id: string): Set<string> => {
      const out = new Set<string>([id])
      let grew = true
      while (grew) {
        grew = false
        for (const n of nodes) if (n.parentId && out.has(n.parentId) && !out.has(n.id)) (out.add(n.id), (grew = true))
      }
      return out
    },
    [nodes],
  )

  const onNodeDragStart = useCallback(
    (_: unknown, node: RFNode, dragged: RFNode[]) => {
      commit()
      dragStart.current = new Map(dragged.map((n) => [n.id, { ...n.position }]))
      setDragging(node.data.type) // containers highlight green/red for this type
    },
    [commit, setDragging],
  )

  /** On drop, re-parent into the deepest container under the node's centre, or snap back. */
  const onNodeDragStop = useCallback(
    (_: unknown, __: RFNode, dragged: RFNode[]) => {
      setDragging(null)
      const starts = dragStart.current
      dragStart.current = new Map()
      for (const n of dragged) {
        if (!rules) continue
        const r = absRect(n.id)
        if (!r) continue
        const centre = { x: r.x + r.w / 2, y: r.y + r.h / 2 }
        const target = containerAt(centre, subtree(n.id))
        const targetId = target?.id ?? null
        if (targetId === (n.parentId ?? null)) continue // same parent: a plain move
        const targetType = target ? target.data.type : ROOT
        if (!rules.canContain(targetType, n.data.type)) {
          const start = starts.get(n.id)
          if (start) reparent(n.id, n.parentId ?? null, start)
          const label = rules.entry(n.data.type)?.label ?? n.data.type
          showToast(`${label} cannot be placed ${target ? `inside a ${rules.entry(targetType)?.label ?? targetType}` : 'on the canvas'}`)
          continue
        }
        let pos = { x: r.x, y: r.y }
        if (target) {
          const tr = absRect(target.id)!
          pos = { x: Math.max(8, r.x - tr.x), y: Math.max(36, r.y - tr.y) }
        }
        reparent(n.id, targetId, pos)
      }
    },
    [rules, absRect, containerAt, subtree, reparent, setDragging, showToast],
  )

  const isValidConnection: IsValidConnection<RFEdge> = useCallback(
    (c) => {
      if (!rules || !c.source || !c.target || c.source === c.target) return false
      const s = nodes.find((n) => n.id === c.source)
      const t = nodes.find((n) => n.id === c.target)
      if (!s || !t) return false
      if (edges.some((e) => e.source === c.source && e.target === c.target)) return false
      return !!rules.connection(s.data.type, t.data.type)
    },
    [rules, nodes, edges],
  )

  const badEdges = useMemo(() => new Set(problems.filter((p) => p.edge).map((p) => p.edge!)), [problems])
  const styledEdges = useMemo(() => edges.map((e) => (badEdges.has(e.id) ? { ...e, className: 'edge-error' } : e)), [edges, badEdges])

  return (
    <div className="canvas" onDrop={onDrop} onDragOver={onDragOver}>
      <ReactFlow<RFNode, RFEdge>
        nodes={nodes}
        edges={styledEdges}
        nodeTypes={nodeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={connect}
        onNodeDragStart={onNodeDragStart}
        onNodeDragStop={onNodeDragStop}
        selectionOnDrag
        panOnDrag={[1, 2]}
        selectionKeyCode="Shift"
        multiSelectionKeyCode={['Meta', 'Control']}
        isValidConnection={isValidConnection}
        onSelectionChange={({ nodes: sel }) => select(sel.length === 1 ? sel[0].id : null)}
        onPaneClick={() => select(null)}
        deleteKeyCode={['Backspace', 'Delete']}
        fitView
        snapToGrid
        snapGrid={[10, 10]}
        minZoom={0.1}
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={20} />
        <MiniMap pannable zoomable nodeStrokeWidth={2} />
      </ReactFlow>
    </div>
  )
}
