import { useMemo } from 'react'
import { useStore } from '../store'
import { COMMON } from '../types'

const LABEL: Record<string, string> = { aws: 'AWS', gcp: 'Google Cloud', azure: 'Azure' }

/** One canvas per cloud provider over the same .iad file. */
export function CanvasTabs() {
  const catalog = useStore((s) => s.catalog)
  const nodes = useStore((s) => s.nodes)
  const active = useStore((s) => s.activeProvider)
  const setActive = useStore((s) => s.setActiveProvider)
  const providers = useMemo(() => [...new Set((catalog?.entries ?? []).map((e) => e.provider))].filter((p) => p !== COMMON).sort(), [catalog])
  const mirror = useStore((s) => s.mirror)
  const primary = useStore((s) => s.primaryProvider())
  const projection = useStore((s) => s.projection)
  const projecting = useStore((s) => s.projecting)
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const n of nodes) {
      const p = n.data.type.split('.')[0]
      if (p === COMMON) continue
      c[p] = (c[p] ?? 0) + 1
    }
    return c
  }, [nodes])
  if (providers.length < 2) return null
  return (
    <div className="canvas-tabs" role="tablist">
      {providers.map((p) => {
        const isSource = p === primary
        const mirrored = mirror && primary !== '' && !isSource && counts[p] === undefined
        const count = mirrored ? (projection?.provider === p ? projection.nodes.length : '≈') : counts[p] ?? 0
        return (
          <button key={p} role="tab" aria-selected={p === active} className={`${p === active ? 'active' : ''} ${mirrored ? 'mirrored' : ''}`} onClick={() => setActive(p)} title={mirrored ? `Live equivalence of ${LABEL[primary] ?? primary}` : isSource && mirror ? 'Source tab' : ''}>
          {LABEL[p] ?? p}
          <span className="count">{p === active && projecting ? '…' : count}</span>
          {mirror && isSource && primary && <i className="src" title="source" />}
        </button>
        )
      })}
    </div>
  )
}
