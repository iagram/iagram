/**
 * Orthogonal connector routing that goes around elements instead of through
 * them, the way the vendor diagrams draw their arrows.
 *
 * The route is searched on an orthogonal visibility grid: the candidate
 * x/y coordinates are the two endpoints, the edges of every nearby obstacle
 * (inflated by a margin) and the midpoint channel between the endpoints. A*
 * over the grid intersections, with a turn penalty, a forced exit direction
 * at the source and a forced entry direction at the target, yields a
 * polyline with few bends that never crosses an obstacle interior.
 */

export interface Rect {
  x: number
  y: number
  w: number
  h: number
}

export interface Point {
  x: number
  y: number
}

export type Side = 't' | 'r' | 'b' | 'l'

export interface RouteRequest {
  source: Point
  sourceSide: Side
  target: Point
  targetSide: Side
  /** boxes the connector must not cross (leaf icons; containers are crossed freely) */
  obstacles: Rect[]
}

const MARGIN = 12 // clearance kept around an obstacle
const TURN_COST = 18 // a bend costs about this many pixels of length
const MAX_NODES = 40_000 // safety valve on the search

type Dir = 0 | 1 | 2 | 3 // right, down, left, up
const DX = [1, 0, -1, 0]
const DY = [0, 1, 0, -1]

function sideToDir(side: Side): Dir {
  return side === 'r' ? 0 : side === 'b' ? 1 : side === 'l' ? 2 : 3
}

function inflate(r: Rect, m: number): Rect {
  return { x: r.x - m, y: r.y - m, w: r.w + 2 * m, h: r.h + 2 * m }
}

/** Obstacles the connector may meet: those inside the routing window. */
function relevant(obstacles: Rect[], a: Point, b: Point): Rect[] {
  const minX = Math.min(a.x, b.x) - 200
  const maxX = Math.max(a.x, b.x) + 200
  const minY = Math.min(a.y, b.y) - 200
  const maxY = Math.max(a.y, b.y) + 200
  return obstacles.filter((o) => o.x < maxX && o.x + o.w > minX && o.y < maxY && o.y + o.h > minY).map((o) => inflate(o, MARGIN))
}

function uniqSorted(vals: number[]): number[] {
  const out: number[] = []
  for (const v of vals.slice().sort((p, q) => p - q)) {
    if (out.length === 0 || Math.abs(out[out.length - 1] - v) > 0.5) out.push(v)
  }
  return out
}

/** Does the segment between two adjacent grid intersections cross an obstacle interior? */
function segmentBlocked(x0: number, y0: number, x1: number, y1: number, obs: Rect[]): boolean {
  const eps = 0.5
  if (y0 === y1) {
    const lo = Math.min(x0, x1)
    const hi = Math.max(x0, x1)
    for (const o of obs) {
      if (y0 > o.y + eps && y0 < o.y + o.h - eps && hi > o.x + eps && lo < o.x + o.w - eps) return true
    }
  } else {
    const lo = Math.min(y0, y1)
    const hi = Math.max(y0, y1)
    for (const o of obs) {
      if (x0 > o.x + eps && x0 < o.x + o.w - eps && hi > o.y + eps && lo < o.y + o.h - eps) return true
    }
  }
  return false
}

/** A small binary heap keyed on f. */
class Heap<T> {
  private a: { k: number; v: T }[] = []
  get size() {
    return this.a.length
  }
  push(k: number, v: T) {
    const a = this.a
    a.push({ k, v })
    let i = a.length - 1
    while (i > 0) {
      const p = (i - 1) >> 1
      if (a[p].k <= a[i].k) break
      ;[a[p], a[i]] = [a[i], a[p]]
      i = p
    }
  }
  pop(): T | undefined {
    const a = this.a
    if (a.length === 0) return undefined
    const top = a[0].v
    const last = a.pop()!
    if (a.length > 0) {
      a[0] = last
      let i = 0
      for (;;) {
        const l = 2 * i + 1
        const r = l + 1
        let m = i
        if (l < a.length && a[l].k < a[m].k) m = l
        if (r < a.length && a[r].k < a[m].k) m = r
        if (m === i) break
        ;[a[m], a[i]] = [a[i], a[m]]
        i = m
      }
    }
    return top
  }
}

/**
 * Route an orthogonal connector. Returns the bend points (source and target
 * included) or null when no obstacle-free route exists.
 */
export function routeOrthogonal(req: RouteRequest): Point[] | null {
  const { source: s, target: t } = req
  const obs = relevant(req.obstacles, s, t)
  const xs = uniqSorted([s.x, t.x, (s.x + t.x) / 2, ...obs.flatMap((o) => [o.x, o.x + o.w])])
  const ys = uniqSorted([s.y, t.y, (s.y + t.y) / 2, ...obs.flatMap((o) => [o.y, o.y + o.h])])
  const ix = (x: number) => xs.findIndex((v) => Math.abs(v - x) <= 0.5)
  const iy = (y: number) => ys.findIndex((v) => Math.abs(v - y) <= 0.5)
  const sxI = ix(s.x)
  const syI = iy(s.y)
  const txI = ix(t.x)
  const tyI = iy(t.y)
  if (sxI < 0 || syI < 0 || txI < 0 || tyI < 0) return null
  const W = xs.length
  const H = ys.length
  const exitDir = sideToDir(req.sourceSide)
  const entryDir = ((sideToDir(req.targetSide) + 2) % 4) as Dir // moving into the target from its side

  // state = (x index, y index, direction of the last move)
  const key = (x: number, y: number, d: number) => (x * H + y) * 4 + d
  const g = new Map<number, number>()
  const parent = new Map<number, number>()
  const heap = new Heap<[number, number, number]>()
  const h = (x: number, y: number) => Math.abs(xs[x] - t.x) + Math.abs(ys[y] - t.y)

  // the first move is forced along the exit side
  {
    const nx = sxI + DX[exitDir]
    const ny = syI + DY[exitDir]
    if (nx < 0 || ny < 0 || nx >= W || ny >= H) return null
    if (segmentBlocked(xs[sxI], ys[syI], xs[nx], ys[ny], obs)) return null
    const cost = Math.abs(xs[nx] - xs[sxI]) + Math.abs(ys[ny] - ys[syI])
    const k = key(nx, ny, exitDir)
    g.set(k, cost)
    parent.set(k, -1)
    heap.push(cost + h(nx, ny), [nx, ny, exitDir])
  }

  let expanded = 0
  let goalKey = -1
  while (heap.size > 0 && expanded < MAX_NODES) {
    const [x, y, d] = heap.pop()!
    const k = key(x, y, d)
    const gk = g.get(k)!
    expanded++
    if (x === txI && y === tyI) {
      if (d === entryDir) {
        goalKey = k
        break
      }
      continue
    }
    for (let nd = 0 as Dir; nd < 4; nd = ((nd + 1) as Dir)) {
      if (nd === (d + 2) % 4) continue // no U-turns
      const nx = x + DX[nd]
      const ny = y + DY[nd]
      if (nx < 0 || ny < 0 || nx >= W || ny >= H) continue
      if (segmentBlocked(xs[x], ys[y], xs[nx], ys[ny], obs)) continue
      const step = Math.abs(xs[nx] - xs[x]) + Math.abs(ys[ny] - ys[y])
      const ng = gk + step + (nd === d ? 0 : TURN_COST)
      const nk = key(nx, ny, nd)
      const old = g.get(nk)
      if (old !== undefined && old <= ng) continue
      g.set(nk, ng)
      parent.set(nk, k)
      heap.push(ng + h(nx, ny), [nx, ny, nd])
    }
  }
  if (goalKey < 0) return null

  // unwind: state keys back to points, then drop collinear points
  const pts: Point[] = []
  for (let k = goalKey; k !== -1; k = parent.get(k)!) {
    const cell = Math.floor(k / 4)
    const x = Math.floor(cell / H)
    const y = cell % H
    pts.push({ x: xs[x], y: ys[y] })
  }
  pts.push({ x: s.x, y: s.y })
  pts.reverse()
  const out: Point[] = [pts[0]]
  for (let i = 1; i < pts.length - 1; i++) {
    const a = out[out.length - 1]
    const b = pts[i]
    const c = pts[i + 1]
    const collinear = (a.x === b.x && b.x === c.x) || (a.y === b.y && b.y === c.y)
    if (!collinear) out.push(b)
  }
  out.push(pts[pts.length - 1])
  return out
}

/** SVG path through the points with rounded corners. */
export function pathFromPoints(pts: Point[], radius = 8): string {
  if (pts.length === 0) return ''
  if (pts.length === 1) return `M ${pts[0].x} ${pts[0].y}`
  let d = `M ${pts[0].x} ${pts[0].y}`
  for (let i = 1; i < pts.length - 1; i++) {
    const p = pts[i - 1]
    const c = pts[i]
    const n = pts[i + 1]
    const inLen = Math.hypot(c.x - p.x, c.y - p.y)
    const outLen = Math.hypot(n.x - c.x, n.y - c.y)
    const r = Math.min(radius, inLen / 2, outLen / 2)
    if (r <= 0) {
      d += ` L ${c.x} ${c.y}`
      continue
    }
    const ax = c.x - ((c.x - p.x) / inLen) * r
    const ay = c.y - ((c.y - p.y) / inLen) * r
    const bx = c.x + ((n.x - c.x) / outLen) * r
    const by = c.y + ((n.y - c.y) / outLen) * r
    d += ` L ${ax} ${ay} Q ${c.x} ${c.y} ${bx} ${by}`
  }
  const last = pts[pts.length - 1]
  d += ` L ${last.x} ${last.y}`
  return d
}

/** The point a fraction of the way along the polyline (for labels). */
export function pointAlong(pts: Point[], frac: number): Point {
  let total = 0
  for (let i = 1; i < pts.length; i++) total += Math.hypot(pts[i].x - pts[i - 1].x, pts[i].y - pts[i - 1].y)
  let want = total * frac
  for (let i = 1; i < pts.length; i++) {
    const seg = Math.hypot(pts[i].x - pts[i - 1].x, pts[i].y - pts[i - 1].y)
    if (want <= seg) {
      const f = seg === 0 ? 0 : want / seg
      return { x: pts[i - 1].x + (pts[i].x - pts[i - 1].x) * f, y: pts[i - 1].y + (pts[i].y - pts[i - 1].y) * f }
    }
    want -= seg
  }
  return pts[pts.length - 1]
}
