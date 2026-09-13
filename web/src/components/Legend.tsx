import { useMemo } from 'react'
import { useStore } from '../store'
import { effectiveStyle } from '../convert'

const DASHES: Record<string, string> = { dashed: '7 5', dotted: '2 4' }

/** Floating legend: the container colours and arrow kinds used by the diagram. */
export function Legend() {
  const show = useStore((s) => s.showLegend)
  const setShow = useStore((s) => s.setShowLegend)
  const nodes = useStore((s) => s.nodes)
  const edges = useStore((s) => s.edges)
  const rules = useStore((s) => s.rules)
  const steps = useStore((s) => s.steps)
  const boxes = useMemo(() => {
    const seen = new Map<string, { label: string; border: string; dash: string }>()
    for (const n of nodes) {
      const e = rules?.entry(n.data.type)
      if (!e || e.kind !== 'container' || seen.has(e.id)) continue
      seen.set(e.id, { label: e.label, border: e.style?.border ?? '#b9c2d0', dash: e.style?.dash ?? 'dashed' })
    }
    return [...seen.values()]
  }, [nodes, rules])
  const arrows = useMemo(() => {
    const seen = new Map<string, { label: string; style: ReturnType<typeof effectiveStyle> }>()
    for (const e of edges) {
      if (!e.data) continue
      const st = effectiveStyle(e.data)
      const key = `${e.data.kind}|${st.dash ?? ''}|${st.direction ?? ''}|${st.color ?? ''}|${st.inactive ? 'i' : ''}`
      if (seen.has(key)) continue
      const label = e.data.userLabel ?? e.data.ruleLabel ?? e.data.kind
      seen.set(key, { label: label || e.data.kind, style: st })
    }
    return [...seen.values()]
  }, [edges])
  if (!show) return null
  return (
    <div className="legend" role="note">
      <header>
        <b>Legend</b>
        <button className="icon" onClick={() => setShow(false)} title="Hide legend">
          ✕
        </button>
      </header>
      {boxes.length > 0 && (
        <ul>
          {boxes.map((b) => (
            <li key={b.label}>
              <i className="box" style={{ borderColor: b.border, borderStyle: b.dash }} /> {b.label}
            </li>
          ))}
        </ul>
      )}
      {arrows.length > 0 && (
        <ul>
          {arrows.map((a, i) => (
            <li key={i} className={a.style.inactive ? 'inactive' : ''}>
              <svg width="42" height="12" aria-hidden>
                <line x1="2" y1="6" x2="34" y2="6" stroke={a.style.color ?? '#7d8998'} strokeWidth="1.5" strokeDasharray={DASHES[a.style.dash ?? '']} />
                {a.style.direction !== 'none' && <polygon points="34,2 41,6 34,10" fill={a.style.color ?? '#7d8998'} />}
                {a.style.direction === 'both' && <polygon points="9,2 2,6 9,10" fill={a.style.color ?? '#7d8998'} />}
              </svg>
              {a.label}
            </li>
          ))}
        </ul>
      )}
      {steps.length > 0 && (
        <ol className="steps">
          {steps.map((s) => (
            <li key={s.n}>
              <span className="step">{s.n}</span> {s.text}
            </li>
          ))}
        </ol>
      )}
      {boxes.length === 0 && arrows.length === 0 && steps.length === 0 && <p className="muted">Nothing to describe yet.</p>}
    </div>
  )
}
