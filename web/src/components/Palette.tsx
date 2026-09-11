import { useMemo, useState, type DragEvent } from 'react'
import { useStore } from '../store'
import type { Entry } from '../types'
import { ROOT } from '../types'

const ORDER = ['account', 'network', 'compute', 'database', 'storage', 'security', 'serverless', 'integration']

export const DND_TYPE = 'application/x-iagram-type'

const PROVIDER_LABEL: Record<string, string> = { aws: 'AWS', gcp: 'Google Cloud', azure: 'Azure' }

export function Palette() {
  const catalog = useStore((s) => s.catalog)
  const rules = useStore((s) => s.rules)
  const selectedId = useStore((s) => s.selectedId)
  const nodes = useStore((s) => s.nodes)
  const setDragging = useStore((s) => s.setDragging)

  const providers = useMemo(() => [...new Set((catalog?.entries ?? []).map((e) => e.provider))].sort(), [catalog])
  const inUse = useMemo(() => [...new Set(nodes.map((n) => n.data.type.split('.')[0]))], [nodes])
  // The tab the user picked; until then, derive it (no effect, no setState loop).
  const [chosen, setChosen] = useState<string | null>(null)
  const provider = chosen && providers.includes(chosen) ? chosen : inUse.find((p) => providers.includes(p)) ?? providers[0] ?? ''
  const setProvider = setChosen

  if (!catalog || !rules) return <aside className="palette" />

  // Context: what can go inside the selected container (or on the canvas)?
  const selected = nodes.find((n) => n.id === selectedId)
  const contextType = selected ? (selected.type === 'container' ? selected.data.type : selected.parentId ? nodes.find((n) => n.id === selected.parentId)?.data.type ?? ROOT : ROOT) : ROOT
  const contextLabel = contextType === ROOT ? 'the canvas' : rules.entry(contextType)?.label ?? contextType

  const groups = new Map<string, Entry[]>()
  for (const e of catalog.entries) {
    if (e.provider !== provider) continue
    ;(groups.get(e.category) ?? groups.set(e.category, []).get(e.category)!).push(e)
  }

  const onDragStart = (ev: DragEvent, e: Entry) => {
    ev.dataTransfer.setData(DND_TYPE, e.id)
    ev.dataTransfer.effectAllowed = 'move'
    setDragging(e.id)
  }

  return (
    <aside className="palette">
      <h2>Elements</h2>
      {providers.length > 1 && (
        <div className="providers" role="tablist">
          {providers.map((p) => (
            <button key={p} role="tab" aria-selected={p === provider} className={p === provider ? 'active' : ''} onClick={() => setProvider(p)}>
              {PROVIDER_LABEL[p] ?? p}
              {inUse.includes(p) && <i title="used in this diagram" />}
            </button>
          ))}
        </div>
      )}
      <p className="hint">
        Drag onto the canvas. Highlighted items fit inside <b>{contextLabel}</b>.
      </p>
      {[...groups.entries()]
        .sort(([a], [b]) => ORDER.indexOf(a) - ORDER.indexOf(b))
        .map(([cat, entries]) => (
          <section key={cat}>
            <h3>{cat}</h3>
            {entries.map((e) => {
              const fits = rules.canContain(contextType, e.id)
              return (
                <div
                  key={e.id}
                  className={`item ${fits ? 'fits' : 'dim'}`}
                  draggable
                  onDragStart={(ev) => onDragStart(ev, e)}
                  onDragEnd={() => setDragging(null)}
                  title={e.description ?? e.label}
                >
                  {e.icon && <img src={`/icons/${e.icon}`} alt="" draggable={false} />}
                  <span>{e.label}</span>
                  {e.kind === 'container' && <span className="tag">box</span>}
                </div>
              )
            })}
          </section>
        ))}
    </aside>
  )
}
