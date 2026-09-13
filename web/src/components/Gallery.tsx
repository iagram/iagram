import { useEffect, useMemo, useState } from 'react'
import { useStore } from '../store'
import type { Document, TemplateItem } from '../types'
export type { TemplateItem }

const PROVIDER_LABEL: Record<string, string> = { aws: 'AWS', gcp: 'Google Cloud', azure: 'Azure' }
const CATEGORY_LABEL: Record<string, string> = {
  networking: 'Networking',
  web: 'Web applications',
  serverless: 'Serverless',
  containers: 'Containers',
  data: 'Data & analytics',
  ml: 'AI & machine learning',
  resilience: 'Resilience & DR',
  security: 'Security & governance',
  iot: 'IoT',
  devops: 'DevOps',
}
const CATEGORY_ORDER = Object.keys(CATEGORY_LABEL)

/** Front page: the reference architectures, filtered by cloud, category and text; a card opens the diagram in the editor. */
export function Gallery() {
  const show = useStore((s) => s.showGallery)
  const setShow = useStore((s) => s.setShowGallery)
  const templates = useStore((s) => s.templates)
  const loadTemplates = useStore((s) => s.loadTemplates)
  const useTemplate = useStore((s) => s.useTemplate)
  const hasNodes = useStore((s) => s.nodes.length > 0)
  const [provider, setProvider] = useState<string>('all')
  const [category, setCategory] = useState<string>('all')
  const [q, setQ] = useState('')
  useEffect(() => {
    if (show) void loadTemplates()
  }, [show, loadTemplates])
  const list = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return (templates ?? []).filter(
      (t) =>
        (provider === 'all' || t.provider === provider) &&
        (category === 'all' || t.category === category) &&
        (!needle || t.title.toLowerCase().includes(needle) || t.description.toLowerCase().includes(needle) || t.tags.some((x) => x.toLowerCase().includes(needle))),
    )
  }, [templates, provider, category, q])
  const categories = useMemo(() => {
    const seen = new Set((templates ?? []).filter((t) => provider === 'all' || t.provider === provider).map((t) => t.category))
    return CATEGORY_ORDER.filter((c) => seen.has(c))
  }, [templates, provider])
  const counts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const t of templates ?? []) c[t.provider] = (c[t.provider] ?? 0) + 1
    return c
  }, [templates])
  if (!show) return null
  return (
    <div className="gallery" role="dialog" aria-label="Reference architectures">
      <header className="gallery-head">
        <div>
          <h1>Reference architectures</h1>
          <p className="muted">Official reference diagrams from AWS, Google Cloud and Azure, redrawn as editable, deployable diagrams. Pick one to open it in the editor, then adapt it and plan.</p>
        </div>
        <div className="gallery-actions">
          <button
            onClick={() => {
              setShow(false)
            }}
          >
            {hasNodes ? 'Open my diagram' : 'Start from a blank canvas'}
          </button>
        </div>
      </header>
      <div className="gallery-filters">
        <div className="tabs" role="tablist">
          {['all', 'aws', 'gcp', 'azure'].map((p) => (
            <button key={p} role="tab" aria-selected={provider === p} className={provider === p ? 'active' : ''} onClick={() => (setProvider(p), setCategory('all'))}>
              {p === 'all' ? 'All clouds' : PROVIDER_LABEL[p]} <span className="count">{p === 'all' ? (templates?.length ?? 0) : counts[p] ?? 0}</span>
            </button>
          ))}
        </div>
        <input className="search" type="search" placeholder="Search architectures, services, tags…" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      <div className="gallery-chips">
        <button className={category === 'all' ? 'chip active' : 'chip'} onClick={() => setCategory('all')}>
          All categories
        </button>
        {categories.map((c) => (
          <button key={c} className={category === c ? 'chip active' : 'chip'} onClick={() => setCategory(c)}>
            {CATEGORY_LABEL[c] ?? c}
          </button>
        ))}
      </div>
      {!templates && <p className="muted gallery-empty">Loading…</p>}
      {templates && list.length === 0 && <p className="muted gallery-empty">No architecture matches.</p>}
      <div className="gallery-grid">
        {list.map((t) => (
          <article key={t.id} className="card" onClick={() => void useTemplate(t)} title="Open in the editor">
            <Preview doc={t.document} provider={t.provider} types={t.types} />
            <div className="card-body">
              <div className="card-top">
                <span className={`pill ${t.provider}`}>{PROVIDER_LABEL[t.provider] ?? t.provider}</span>
                <span className="muted">{CATEGORY_LABEL[t.category] ?? t.category}</span>
              </div>
              <h3>{t.title}</h3>
              <p>{t.description}</p>
              <div className="card-foot">
                <span className="muted">{t.elements} elements</span>
                <a href={t.source} target="_blank" rel="noreferrer" onClick={(e) => e.stopPropagation()}>
                  Official reference ↗
                </a>
              </div>
            </div>
          </article>
        ))}
      </div>
    </div>
  )
}

/** Miniature of the diagram: containers as tinted boxes, elements as icons. */
function Preview({ doc, provider, types }: { doc: Document; provider: string; types: TemplateItem['types'] }) {
  const rules = useStore((s) => s.rules)
  const W = 320
  const H = 180
  const { boxes, leaves, vb } = useMemo(() => {
    const byId = new Map(doc.nodes.map((n) => [n.id, n]))
    const abs = (id: string): { x: number; y: number } => {
      let n = byId.get(id)
      let x = 0
      let y = 0
      const seen = new Set<string>()
      while (n && !seen.has(n.id)) {
        seen.add(n.id)
        x += n.layout.x
        y += n.layout.y
        n = n.parent ? byId.get(n.parent) : undefined
      }
      return { x, y }
    }
    const boxes: { x: number; y: number; w: number; h: number; color: string; dash: boolean }[] = []
    const leaves: { x: number; y: number; icon: string }[] = []
    let minX = Infinity
    let minY = Infinity
    let maxX = -Infinity
    let maxY = -Infinity
    for (const n of doc.nodes) {
      const e = rules?.entry(n.type)
      const ti = types?.[n.type]
      const p = abs(n.id)
      const container = ti ? !!ti.container : e ? e.kind === 'container' : !!(n.layout.w && n.layout.w > 200)
      const w = n.layout.w ?? 120
      const h = n.layout.h ?? 90
      minX = Math.min(minX, p.x)
      minY = Math.min(minY, p.y)
      maxX = Math.max(maxX, p.x + w)
      maxY = Math.max(maxY, p.y + h)
      const border = ti?.border ?? e?.style?.border ?? '#9aa4b2'
      const dash = (ti?.dash ?? e?.style?.dash ?? 'dashed') !== 'solid'
      const icon = ti?.icon ?? e?.icon
      if (container) boxes.push({ x: p.x, y: p.y, w, h, color: border, dash })
      else leaves.push({ x: p.x + w / 2, y: p.y + h / 2, icon: icon ? `/icons/${icon}` : `/icons/${n.type.split('.')[0]}/generic.svg` })
    }
    if (!isFinite(minX)) return { boxes, leaves, vb: `0 0 ${W} ${H}` }
    const pad = 40
    const bw = maxX - minX + 2 * pad
    const bh = maxY - minY + 2 * pad
    const scale = Math.max(bw / W, bh / H)
    const vw = W * scale
    const vh = H * scale
    return { boxes, leaves, vb: `${minX - pad - (vw - bw) / 2} ${minY - pad - (vh - bh) / 2} ${vw} ${vh}` }
  }, [doc, rules, types])
  const iconSize = useMemo(() => {
    const parts = vb.split(' ').map(Number)
    return Math.max(28, (parts[2] / W) * 22)
  }, [vb])
  return (
    <svg className="preview" viewBox={vb} width="100%" preserveAspectRatio="xMidYMid meet" data-provider={provider} aria-hidden>
      {boxes.map((b, i) => (
        <rect key={i} x={b.x} y={b.y} width={b.w} height={b.h} rx={12} fill={b.color} fillOpacity={0.06} stroke={b.color} strokeWidth={Math.max(2, iconSize / 14)} strokeDasharray={b.dash ? `${iconSize / 3} ${iconSize / 4}` : undefined} />
      ))}
      {leaves.map((l, i) => (
        <image key={i} href={l.icon} x={l.x - iconSize / 2} y={l.y - iconSize / 2} width={iconSize} height={iconSize} />
      ))}
    </svg>
  )
}
