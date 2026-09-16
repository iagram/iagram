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
  migration: 'Migration',
  media: 'Media',
  'analytics-bi': 'Analytics & BI',
  observability: 'Observability',
  identity: 'Identity',
  storage: 'Storage',
  database: 'Databases',
  compute: 'Compute',
  edge: 'Edge',
  industry: 'Industry solutions',
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
  const nodeCount = useStore((s) => s.nodes.length)
  const docName = useStore((s) => s.docName)
  const dirty = useStore((s) => s.dirty)
  const newDiagram = useStore((s) => s.newDiagram)
  const newIad = () => {
    if (hasNodes && dirty && !confirm('Start a new IaD? The current canvas is discarded (the file on disk is not touched until you save).')) return
    if (hasNodes) newDiagram()
    setShow(false)
  }
  const [provider, setProvider] = useState<string>('all')
  const [category, setCategory] = useState<string>('all')
  const [q, setQ] = useState('')
  const [refImages, setRefImages] = useState<boolean>(() => {
    try {
      return localStorage.getItem('iagram.refimages') !== '0'
    } catch {
      return true
    }
  })
  const toggleRefImages = (v: boolean) => {
    setRefImages(v)
    try {
      localStorage.setItem('iagram.refimages', v ? '1' : '0')
    } catch {
      /* private mode */
    }
  }
  const PAGE = 48
  const [limit, setLimit] = useState(PAGE)
  useEffect(() => {
    if (show) void loadTemplates()
  }, [show, loadTemplates])
  useEffect(() => setLimit(PAGE), [provider, category, q])
  const list = useMemo(() => {
    const needle = q.trim().toLowerCase()
    return (templates ?? []).filter(
      (t) =>
        (provider === 'all' || t.provider === provider) &&
        (category === 'all' || t.category === category) &&
        (!needle || t.title.toLowerCase().includes(needle) || t.description.toLowerCase().includes(needle) || t.tags.some((x) => x.toLowerCase().includes(needle))),
    )
  }, [templates, provider, category, q])
  const categoryCounts = useMemo(() => {
    const c: Record<string, number> = {}
    for (const t of templates ?? []) if (provider === 'all' || t.provider === provider) c[t.category] = (c[t.category] ?? 0) + 1
    return c
  }, [templates, provider])
  const categories = useMemo(() => [...CATEGORY_ORDER.filter((c) => categoryCounts[c]), ...Object.keys(categoryCounts).filter((c) => !CATEGORY_ORDER.includes(c)).sort()], [categoryCounts])
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
          <h1>iagram</h1>
          <p className="muted">Infrastructure as diagram. Start a new IaD, or open one of the official reference architectures from AWS, Google Cloud and Azure, redrawn as editable, deployable diagrams. A card always opens a copy: the references stay pristine.</p>
        </div>
        <div className="gallery-actions">
          {hasNodes && (
            <button onClick={() => setShow(false)} title={`Continue editing ${docName || 'the current diagram'}`}>
              Continue {docName || 'my diagram'} <span className="muted">({nodeCount} elements)</span>
            </button>
          )}
          <button className="primary" onClick={newIad}>
            + New IaD
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
        <label className="field check refimages" title="Show the vendor's original diagram (bundled with iagram, credited on the card) instead of the generated miniature.">
          <input type="checkbox" checked={refImages} onChange={(e) => toggleRefImages(e.target.checked)} />
          <span>Original diagrams</span>
        </label>
      </div>
      <div className="gallery-chips">
        <button className={category === 'all' ? 'chip active' : 'chip'} onClick={() => setCategory('all')}>
          All categories
        </button>
        {categories.map((c) => (
          <button key={c} className={category === c ? 'chip active' : 'chip'} onClick={() => setCategory(c)}>
            {CATEGORY_LABEL[c] ?? c} <span className="count">{categoryCounts[c]}</span>
          </button>
        ))}
      </div>
      {!templates && <p className="muted gallery-empty">Loading…</p>}
      {templates && list.length === 0 && <p className="muted gallery-empty">No architecture matches.</p>}
      <div className="gallery-grid">
        {!q && category === 'all' && (
          <article className="card new" onClick={newIad} title="Start from a blank canvas">
            <div className="preview plus">+</div>
            <div className="card-body">
              <h3>New IaD</h3>
              <p>Start from a blank canvas: pick a cloud tab, drag elements from the palette, connect them, plan.</p>
            </div>
          </article>
        )}
        {list.slice(0, limit).map((t) => (
          <article key={t.id} className="card" onClick={() => void useTemplate(t)} title="Open a copy in the editor (the reference architecture itself is never modified)">
            <CardImage t={t} useRef={refImages} />
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
      {list.length > limit && (
        <div className="gallery-more">
          <button onClick={() => setLimit((l) => l + PAGE)}>
            Show more ({list.length - limit} remaining)
          </button>
        </div>
      )}
    </div>
  )
}

/** The original vendor diagram when available and enabled, else the generated miniature. */
function CardImage({ t, useRef }: { t: TemplateItem; useRef: boolean }) {
  const [failed, setFailed] = useState(false)
  if (useRef && t.image && !failed) {
    let host = ''
    try {
      host = new URL(t.image_source ?? t.source).hostname
    } catch {
      /* keep empty */
    }
    return (
      <div className="preview ref">
        <img src={t.image} alt={`Reference diagram: ${t.title}`} loading="lazy" onError={() => setFailed(true)} />
        {host && <span className="credit">Diagram from {host}</span>}
      </div>
    )
  }
  return <Preview doc={t.document} provider={t.provider} types={t.types} />
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
