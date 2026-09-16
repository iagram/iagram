// Arrange algorithms for the canvas. Each works per container, bottom-up:
// children are placed, the container grows to fit, then the parent is laid
// out with the new sizes. Spanning bands keep their place; attachments are
// hidden and ignored.
import type { RFEdge, RFNode } from './convert'
import type { Rules } from './rules'

export type Algo = 'flow-h' | 'flow-v' | 'grid' | 'circle'

const LEAF_W = 120
const LEAF_H = 90
const GAP = 40
const PAD_X = 30
const PAD_TOP = 56
const PAD_BOTTOM = 24
const MIN_W = 240
const MIN_H = 150

interface Box {
  id: string
  w: number
  h: number
  x: number
  y: number
}

export function arrange(nodes: RFNode[], edges: RFEdge[], rules: Rules, algo: Algo, rootId: string | null): RFNode[] {
  const byId = new Map(nodes.map((n) => [n.id, { ...n, position: { ...n.position }, style: { ...n.style } }]))
  const children = new Map<string | null, string[]>()
  for (const n of nodes) {
    const e = rules.entry(n.data.type)
    if (e?.attachment) continue
    const p = n.parentId ?? null
    ;(children.get(p) ?? children.set(p, []).get(p)!).push(n.id)
  }
  // ancestor chain lookup, so edges between deep descendants count between siblings
  const parentOf = (id: string) => byId.get(id)?.parentId ?? null
  const ancestorUnder = (id: string, parent: string | null): string | null => {
    let cur: string | null = id
    while (cur && parentOf(cur) !== parent) cur = parentOf(cur)
    return cur
  }
  const sizeOf = (id: string): { w: number; h: number } => {
    const n = byId.get(id)!
    if (n.type !== 'container') {
      const e = rules.entry(n.data.type)
      return { w: Number(n.style?.width ?? (e?.size?.w ?? LEAF_W)), h: Number(n.style?.height ?? (e?.size?.h ?? LEAF_H)) }
    }
    return { w: Number(n.style?.width ?? MIN_W), h: Number(n.style?.height ?? MIN_H) }
  }

  const layoutContainer = (parent: string | null) => {
    const kids = (children.get(parent) ?? []).filter((id) => !rules.entry(byId.get(id)!.data.type)?.span)
    // containers first, so their sizes are known
    for (const id of kids) if (byId.get(id)!.type === 'container') layoutContainer(id)
    if (kids.length === 0) return
    const boxes: Box[] = kids.map((id) => ({ id, ...sizeOf(id), x: 0, y: 0 }))
    const idx = new Map(kids.map((id, i) => [id, i]))
    // edges between kids (via descendants)
    const out = new Map<string, Set<string>>()
    const indeg = new Map<string, number>(kids.map((k) => [k, 0]))
    for (const e of edges) {
      const a = ancestorUnder(e.source, parent)
      const b = ancestorUnder(e.target, parent)
      if (!a || !b || a === b || !idx.has(a) || !idx.has(b)) continue
      const set = out.get(a) ?? out.set(a, new Set()).get(a)!
      if (!set.has(b)) {
        set.add(b)
        indeg.set(b, (indeg.get(b) ?? 0) + 1)
      }
    }
    // reading order: actors/data centers first, then by name
    const order = [...kids].sort((a, b) => {
      const ca = byId.get(a)!.data.type.startsWith('common.') ? 0 : 1
      const cb = byId.get(b)!.data.type.startsWith('common.') ? 0 : 1
      return ca - cb || byId.get(a)!.data.name.localeCompare(byId.get(b)!.data.name)
    })
    // longest-path ranks (cycles broken by visiting order)
    const rank = new Map<string, number>()
    const visiting = new Set<string>()
    const rankOf = (id: string): number => {
      if (rank.has(id)) return rank.get(id)!
      if (visiting.has(id)) return 0
      visiting.add(id)
      let r = 0
      for (const k of kids) if (out.get(k)?.has(id)) r = Math.max(r, rankOf(k) + 1)
      visiting.delete(id)
      rank.set(id, r)
      return r
    }
    for (const k of order) rankOf(k)
    const place = (b: Box, x: number, y: number) => ((b.x = x), (b.y = y))
    const boxOf = (id: string) => boxes[idx.get(id)!]

    if (algo === 'flow-h' || algo === 'flow-v') {
      const cols = new Map<number, string[]>()
      for (const k of order) (cols.get(rank.get(k)!) ?? cols.set(rank.get(k)!, []).get(rank.get(k)!)!).push(k)
      const ranks = [...cols.keys()].sort((a, b) => a - b)
      let main = algo === 'flow-h' ? PAD_X : PAD_TOP
      for (const r of ranks) {
        const ids = cols.get(r)!
        const thick = Math.max(...ids.map((id) => (algo === 'flow-h' ? boxOf(id).w : boxOf(id).h)))
        let cross = algo === 'flow-h' ? PAD_TOP : PAD_X
        for (const id of ids) {
          const b = boxOf(id)
          if (algo === 'flow-h') place(b, main, cross), (cross += b.h + GAP)
          else place(b, cross, main), (cross += b.w + GAP)
        }
        main += thick + GAP * 1.5
      }
    } else if (algo === 'circle') {
      const n = order.length
      const maxW = Math.max(...boxes.map((b) => b.w))
      const maxH = Math.max(...boxes.map((b) => b.h))
      const radius = n === 1 ? 0 : Math.max((n * (Math.max(maxW, maxH) + GAP)) / (2 * Math.PI), maxW)
      const cx = PAD_X + radius + maxW / 2
      const cy = PAD_TOP + radius + maxH / 2
      order.forEach((id, i) => {
        const a = (2 * Math.PI * i) / n - Math.PI / 2
        const b = boxOf(id)
        place(b, cx + radius * Math.cos(a) - b.w / 2, cy + radius * Math.sin(a) - b.h / 2)
      })
    } else {
      // grid: rows of about sqrt(n), containers first
      const sorted = [...order].sort((a, b) => Number(byId.get(b)!.type === 'container') - Number(byId.get(a)!.type === 'container'))
      const perRow = Math.max(1, Math.min(4, Math.ceil(Math.sqrt(sorted.length))))
      let x = PAD_X
      let y = PAD_TOP
      let rowH = 0
      sorted.forEach((id, i) => {
        const b = boxOf(id)
        if (i > 0 && i % perRow === 0) ((x = PAD_X), (y += rowH + GAP), (rowH = 0))
        place(b, x, y)
        x += b.w + GAP
        rowH = Math.max(rowH, b.h)
      })
    }
    // normalise to the padding origin and grow the container
    const minX = Math.min(...boxes.map((b) => b.x))
    const minY = Math.min(...boxes.map((b) => b.y))
    let maxX = 0
    let maxY = 0
    for (const b of boxes) {
      b.x += PAD_X - minX
      b.y += PAD_TOP - minY
      const n = byId.get(b.id)!
      n.position = { x: Math.round(b.x), y: Math.round(b.y) }
      maxX = Math.max(maxX, b.x + b.w)
      maxY = Math.max(maxY, b.y + b.h)
    }
    if (parent) {
      const p = byId.get(parent)!
      p.style = { ...p.style, width: Math.max(MIN_W, Math.round(maxX + PAD_X)), height: Math.max(MIN_H, Math.round(maxY + PAD_BOTTOM)) }
    }
  }

  if (rootId) layoutContainer(rootId)
  else layoutContainer(null)
  return nodes.map((n) => byId.get(n.id)!)
}
