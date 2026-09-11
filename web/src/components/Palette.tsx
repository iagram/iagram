import { useEffect, useMemo, useState, type DragEvent } from 'react'
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

  const provider = useStore((s) => s.activeProvider)
  const generated = useStore((s) => s.generatedByProvider[s.activeProvider])
  const loadGenerated = useStore((s) => s.loadGenerated)
  const ensureEntry = useStore((s) => s.ensureEntry)
  const [query, setQuery] = useState('')
  useEffect(() => {
    if (provider) void loadGenerated(provider)
  }, [provider, loadGenerated])
  const q = query.trim().toLowerCase()
  const generatedMatches = useMemo(() => {
    const list = (generated ?? []).filter((g) => g.graphical)
    if (!q) return list
    return list.filter((g) => g.label.toLowerCase().includes(q) || g.resource.includes(q) || g.service.includes(q))
  }, [generated, q])
  const attachmentCount = useMemo(() => (generated ?? []).filter((g) => !g.graphical).length, [generated])

  if (!catalog || !rules) return <aside className="palette" />

  // Context: what can go inside the selected container (or on the canvas)?
  const selected = nodes.find((n) => n.id === selectedId)
  const contextType = selected ? (selected.type === 'container' ? selected.data.type : selected.parentId ? nodes.find((n) => n.id === selected.parentId)?.data.type ?? ROOT : ROOT) : ROOT
  const contextLabel = contextType === ROOT ? 'the canvas' : rules.entry(contextType)?.label ?? contextType

  const groups = new Map<string, Entry[]>()
  for (const e of catalog.entries) {
    if (e.provider !== provider) continue
    if (q && !e.label.toLowerCase().includes(q) && !e.id.includes(q)) continue
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
      <input className="search" type="search" placeholder={`Search ${PROVIDER_LABEL[provider] ?? provider} elements…`} value={query} onChange={(e) => setQuery(e.target.value)} />
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
      <section className="generated">
        <h3>
          All resources <span className="muted">({generated ? generatedMatches.length : '…'})</span>
        </h3>
        <p className="hint">
          Every {PROVIDER_LABEL[provider] ?? provider} Terraform resource with an official icon, settings generated from the provider schema.
          {attachmentCount > 0 && ` ${attachmentCount} more (policies, rules, associations…) attach from an element's settings.`}
        </p>
        {generatedMatches.slice(0, 60).map((g) => {
          const fits = rules.canContain(contextType, g.id) || !rules.entry(g.id)
          return (
            <div
              key={g.id}
              className={`item ${fits ? 'fits' : 'dim'}`}
              draggable
              onMouseEnter={() => void ensureEntry(g.id)}
              onDragStart={(ev) => {
                void ensureEntry(g.id)
                ev.dataTransfer.setData(DND_TYPE, g.id)
                ev.dataTransfer.effectAllowed = 'move'
                setDragging(g.id)
              }}
              onDragEnd={() => setDragging(null)}
              title={g.resource}
            >
              {g.icon && <img src={`/icons/${g.icon}`} alt="" draggable={false} />}
              <span>
                {g.label}
                <small className="mono">{g.resource}</small>
              </span>
            </div>
          )
        })}
        {generatedMatches.length > 60 && <p className="hint">{generatedMatches.length - 60} more; refine the search.</p>}
      </section>
    </aside>
  )
}
