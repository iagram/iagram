// iagram.json <-> React Flow nodes/edges.
import type { Edge, Node } from '@xyflow/react'
import { MarkerType } from '@xyflow/react'
import type { DocEdge, DocNode, Document, Entry } from './types'
import type { Rules } from './rules'

export interface NodeData extends Record<string, unknown> {
  type: string
  name: string
  props: Record<string, unknown>
  outputs?: Record<string, unknown>
}

export interface EdgeData extends Record<string, unknown> {
  kind: string
  label: string
  attr?: string
  output?: string
}

export type RFNode = Node<NodeData, 'container' | 'resource'>
export type RFEdge = Edge<EdgeData>

export const LEAF_W = 120
export const LEAF_H = 90

export function makeNode(rules: Rules, n: DocNode): RFNode {
  const entry = rules.entry(n.type)
  const container = entry?.kind === 'container'
  const w = n.layout.w ?? entry?.size?.w ?? LEAF_W
  const h = n.layout.h ?? entry?.size?.h ?? LEAF_H
  return {
    id: n.id,
    type: container ? 'container' : 'resource',
    position: { x: n.layout.x, y: n.layout.y },
    data: { type: n.type, name: n.name, props: n.props ?? {}, outputs: n.outputs },
    parentId: n.parent || undefined,
    style: { width: w, height: h },
    // containers sit under their children in z-order
    zIndex: container ? 0 : 1,
  }
}

export function makeEdge(e: DocEdge, label: string): RFEdge {
  return {
    id: e.id,
    source: e.source,
    target: e.target,
    type: 'smoothstep',
    label,
    labelBgPadding: [6, 3],
    labelBgBorderRadius: 8,
    labelShowBg: true,
    data: { kind: e.kind, label, attr: e.attr, output: e.output },
    markerEnd: { type: MarkerType.ArrowClosed },
  }
}

/** Parents must precede children for React Flow sub-flows. */
export function sortByDepth(nodes: DocNode[]): DocNode[] {
  const byId = new Map(nodes.map((n) => [n.id, n]))
  const depth = (n: DocNode): number => {
    let d = 0
    let cur: DocNode | undefined = n
    const seen = new Set<string>()
    while (cur?.parent && !seen.has(cur.id)) {
      seen.add(cur.id)
      cur = byId.get(cur.parent)
      d++
    }
    return d
  }
  return [...nodes].sort((a, b) => depth(a) - depth(b) || a.id.localeCompare(b.id))
}

export function fromDocument(rules: Rules, doc: Document): { nodes: RFNode[]; edges: RFEdge[] } {
  const nodes = sortByDepth(doc.nodes).map((n) => makeNode(rules, n))
  const types = new Map(doc.nodes.map((n) => [n.id, n.type]))
  const edges = doc.edges.map((e) => {
    const rule = rules.connection(types.get(e.source) ?? '', types.get(e.target) ?? '')
    const label = e.kind === 'references' && e.attr ? e.attr : rule?.label ?? e.kind
    return makeEdge(e, label)
  })
  return { nodes, edges }
}

export function toDocument(name: string, nodes: RFNode[], edges: RFEdge[]): Document {
  return {
    version: 1,
    name: name || undefined,
    nodes: nodes.map((n) => {
      const w = num(n.style?.width) ?? n.measured?.width
      const h = num(n.style?.height) ?? n.measured?.height
      const container = n.type === 'container'
      return {
        id: n.id,
        type: n.data.type,
        name: n.data.name,
        parent: n.parentId || undefined,
        props: n.data.props,
        ...(n.data.outputs && Object.keys(n.data.outputs).length ? { outputs: n.data.outputs } : {}),
        layout: {
          x: round(n.position.x),
          y: round(n.position.y),
          ...(container && w ? { w: round(w) } : {}),
          ...(container && h ? { h: round(h) } : {}),
        },
      }
    }),
    edges: edges.map((e) => ({ id: e.id, kind: e.data?.kind ?? '', source: e.source, target: e.target, ...(e.data?.attr ? { attr: e.data.attr } : {}), ...(e.data?.output ? { output: e.data.output } : {}) })),
  }
}

function num(v: unknown): number | undefined {
  return typeof v === 'number' ? v : undefined
}

function round(v: number): number {
  return Math.round(v)
}

export function newId(entry: Entry): string {
  const short = entry.id.split('.')[1].replace(/_.*$/, '').slice(0, 6)
  return `${short}-${Math.random().toString(36).slice(2, 6)}`
}
