// iagram.json <-> React Flow nodes/edges.
import type { Edge, Node } from '@xyflow/react'
import { MarkerType } from '@xyflow/react'
import type { DocEdge, DocNode, Document, EdgeStyle, Entry, Step } from './types'
import type { Rules } from './rules'

export interface NodeData extends Record<string, unknown> {
  type: string
  name: string
  props: Record<string, unknown>
  outputs?: Record<string, unknown>
  z?: number
  caption?: string
  view?: 'box' | 'icon'
  step?: string
}

/** Second line of a node: its own caption, else the catalog's caption_prop value. */
export function captionOf(entry: Entry | undefined, data: NodeData): string {
  if (data.caption) return data.caption
  const prop = entry?.caption_prop
  if (!prop) return ''
  const v = data.props[prop]
  return v === undefined || v === null || v === '' ? '' : String(v)
}

/** Containers sit under leaves; within each tier, z orders siblings. */
export function zIndexFor(container: boolean, z: number | undefined): number {
  return (container ? 0 : 1000) + (z ?? 0)
}

export interface EdgeData extends Record<string, unknown> {
  kind: string
  /** label shown on the canvas: the user's label, else the connection kind's */
  label: string
  /** the user's own label, persisted */
  userLabel?: string
  /** the kind's default label, from the catalog rule */
  ruleLabel: string
  step?: string
  style?: EdgeStyle
  /** the kind's default style, from the catalog rule */
  ruleStyle?: EdgeStyle
  /** set by the canvas: false hides the label (labels-off mode, not hovered) */
  showLabel?: boolean
  /** true until the author picks the anchors: the canvas chooses the facing sides */
  autoPorts?: boolean
  attr?: string
  output?: string
  /** link edges: link element id, resource name, attributes besides the ends */
  type?: string
  name?: string
  props?: Record<string, unknown>
}

/** Effective style: the edge's own over the kind's default. */
export function effectiveStyle(d: EdgeData | undefined): EdgeStyle {
  return { ...(d?.ruleStyle ?? {}), ...(d?.style ?? {}) }
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
    data: { type: n.type, name: n.name, props: n.props ?? {}, outputs: n.outputs, z: n.layout.z, caption: n.caption, step: n.step, view: n.view },
    parentId: n.parent || undefined,
    style: { width: w, height: h },
    zIndex: zIndexFor(container, n.layout.z),
  }
}

export function makeEdge(e: DocEdge, ruleLabel: string, ruleStyle?: EdgeStyle): RFEdge {
  // Reference diagrams never write "references" or "protects" on a line:
  // those kinds stay silent unless the author typed a label.
  const quiet = e.kind === 'references' || e.kind === 'protects'
  // Without hand-set ports the canvas picks the sides facing each other.
  const data: EdgeData = { kind: e.kind, label: e.label || ruleLabel, userLabel: e.label, ruleLabel, step: e.step, style: e.style, ruleStyle, attr: e.attr, output: e.output, type: e.type, name: e.name, props: e.props, showLabel: !!e.label || !quiet, autoPorts: !e.from_port && !e.to_port }
  return withMarkers({ id: e.id, source: e.source, target: e.target, sourceHandle: e.from_port || 'r', targetHandle: e.to_port || 'l', type: 'iagram', data })
}

/** Arrow heads follow the effective direction (one, both, none) and colour. */
export function withMarkers(edge: RFEdge): RFEdge {
  const st = effectiveStyle(edge.data)
  const marker = { type: MarkerType.ArrowClosed, ...(st.color ? { color: st.color } : {}) }
  const direction = st.direction ?? 'one'
  return { ...edge, markerEnd: direction === 'none' ? undefined : marker, markerStart: direction === 'both' ? marker : undefined }
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
    const rule = e.kind === 'link' ? rules.linkRule(types.get(e.source) ?? '', types.get(e.target) ?? '', e.type) ?? rules.connection(types.get(e.source) ?? '', types.get(e.target) ?? '') : rules.connection(types.get(e.source) ?? '', types.get(e.target) ?? '')
    // A rule's label only applies when it is the rule this edge follows: the
    // fallback "references" rule must not caption an arrow of another kind.
    const ruleLabel = rule && rule.kind === e.kind ? rule.label : undefined
    const label = e.kind === 'references' && e.attr ? e.attr : ruleLabel ?? (e.kind === 'flow' ? '' : e.kind)
    return makeEdge(e, label, rule?.style)
  })
  return { nodes, edges }
}

export function toDocument(name: string, nodes: RFNode[], edges: RFEdge[], steps: Step[] = []): Document {
  return {
    version: 1,
    name: name || undefined,
    ...(steps.length ? { steps } : {}),
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
        ...(n.data.caption ? { caption: n.data.caption } : {}),
        ...(n.data.view ? { view: n.data.view } : {}),
        ...(n.data.step ? { step: n.data.step } : {}),
        layout: {
          x: round(n.position.x),
          y: round(n.position.y),
          ...(container && w ? { w: round(w) } : {}),
          ...(container && h ? { h: round(h) } : {}),
          ...(n.data.z ? { z: n.data.z } : {}),
        },
      }
    }),
    edges: edges.map((e) => ({
      id: e.id,
      kind: e.data?.kind ?? '',
      source: e.source,
      target: e.target,
      ...(e.data?.attr ? { attr: e.data.attr } : {}),
      ...(e.data?.output ? { output: e.data.output } : {}),
      ...(e.data?.userLabel ? { label: e.data.userLabel } : {}),
      ...(e.data?.step ? { step: e.data.step } : {}),
      ...(e.data?.style && Object.keys(e.data.style).length ? { style: e.data.style } : {}),
      ...(e.data?.type ? { type: e.data.type } : {}),
      ...(e.data?.name ? { name: e.data.name } : {}),
      ...(e.data?.props && Object.keys(e.data.props).length ? { props: e.data.props } : {}),
      ...(e.sourceHandle && e.sourceHandle !== 'r' ? { from_port: e.sourceHandle } : {}),
      ...(e.targetHandle && e.targetHandle !== 'l' ? { to_port: e.targetHandle } : {}),
    })),
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

interface Rect {
  x: number
  y: number
  w: number
  h: number
}

/** The anchors two boxes face each other with: horizontal when they are more
 *  apart sideways than vertically, else vertical (reference-diagram routing). */
export function facingPorts(s: Rect, t: Rect): [string, string] {
  const dx = t.x + t.w / 2 - (s.x + s.w / 2)
  const dy = t.y + t.h / 2 - (s.y + s.h / 2)
  const gapX = Math.abs(dx) - (s.w + t.w) / 2
  const gapY = Math.abs(dy) - (s.h + t.h) / 2
  if (gapX >= gapY) return dx >= 0 ? ['r', 'l'] : ['l', 'r']
  return dy >= 0 ? ['b', 't'] : ['t', 'b']
}
