import { create } from 'zustand'
import {
  addEdge,
  applyEdgeChanges,
  applyNodeChanges,
  type Connection,
  type EdgeChange,
  type NodeChange,
  type XYPosition,
} from '@xyflow/react'
import { api } from './api'
import { Rules } from './rules'
import { fromDocument, makeEdge, withMarkers, makeNode, newId, toDocument, zIndexFor, type RFEdge, type RFNode } from './convert'
import type { Document } from './types'
import { rfStore } from './rf'
import type { ApplyResult, AttachmentOption, Catalog, ConvertReport, DriftResult, EdgeStyle, Entry, Family, GeneratedSummary, Job, PlanResult, Problem, Step, TemplateItem } from './types'
import { COMMON, ROOT } from './types'

interface State {
  catalog: Catalog | null
  rules: Rules | null
  docName: string
  /** numbered walkthrough text, keyed by step number */
  steps: Step[]
  showLegend: boolean
  /** The reference-architecture gallery (front page). */
  showGallery: boolean
  templates: TemplateItem[] | null
  nodes: RFNode[]
  edges: RFEdge[]
  selectedId: string | null
  problems: Problem[]
  dirty: boolean
  saving: boolean
  error: string | null
  draggingType: string | null
  toast: string | null
  past: Snapshot[]
  future: Snapshot[]
  plan: PlanResult | null
  planStale: boolean
  drift: DriftResult | null
  job: Job | null
  jobLines: string[]
  logOpen: boolean
  confirmApply: boolean
  lastApply: ApplyResult | null
  theme: 'light' | 'dark'
  showLabels: boolean
  showMinimap: boolean
  snapToGrid: boolean
  version: string
  modal: 'shortcuts' | 'about' | null
  /** Which provider's canvas is shown; nodes of other providers are hidden. */
  activeProvider: string
  catalogVersion: number
  generatedByProvider: Record<string, GeneratedSummary[]>
  familiesByProvider: Record<string, Family[]>
  /** Mirror mode: other providers' tabs show a live equivalence of the source tab. */
  mirror: boolean
  projection: { provider: string; nodes: RFNode[]; edges: RFEdge[]; report: ConvertReport } | null
  projecting: boolean
  connectingFrom: string | null
  attachmentsByType: Record<string, AttachmentOption[]>
  contextMenu: { x: number; y: number; flow: XYPosition; target: 'node' | 'edge' | 'pane'; id?: string } | null
  selectedEdgeId: string | null
  hoveredEdgeId: string | null
  /** A node id the canvas should select through React Flow (deep links). */
  pendingSelect: string | null

  load: () => Promise<void>
  save: () => Promise<void>
  validateSoon: () => void
  onNodesChange: (changes: NodeChange<RFNode>[]) => void
  onEdgesChange: (changes: EdgeChange<RFEdge>[]) => void
  addNode: (type: string, parentId: string | null, position: XYPosition) => string | null
  connect: (c: Connection) => void
  updateNode: (id: string, patch: { name?: string; props?: Record<string, unknown>; caption?: string; step?: string }) => void
  /** Edit an edge's label, step or style (informational fields). */
  updateEdge: (id: string, patch: { label?: string; step?: string; style?: EdgeStyle; type?: string; name?: string; props?: Record<string, unknown> }) => void
  setSteps: (steps: Step[]) => void
  setShowLegend: (v: boolean) => void
  setShowGallery: (v: boolean) => void
  loadTemplates: () => Promise<void>
  /** Open a reference architecture in the editor (replaces the unsaved canvas after confirmation). */
  useTemplate: (t: TemplateItem) => Promise<void>
  removeNodes: (ids: string[]) => void
  select: (id: string | null) => void
  requestSelect: (id: string | null) => void
  setDragging: (type: string | null) => void
  showToast: (msg: string) => void
  /** Record the current graph before a structural change (coalesced by key). */
  commit: (key?: string) => void
  undo: () => void
  redo: () => void
  runPlan: () => Promise<void>
  runDestroyPlan: () => Promise<void>
  runApply: () => Promise<void>
  runDrift: () => Promise<void>
  cancelPlan: () => Promise<void>
  clearPlan: () => void
  clearDrift: () => Promise<void>
  setLogOpen: (open: boolean) => void
  setConfirmApply: (open: boolean) => void
  /** Move a node (and its subtree) under a new parent at a position relative to it. */
  reparent: (id: string, parentId: string | null, position: XYPosition) => void
  copySelection: () => void
  cutSelection: () => void
  paste: (at?: XYPosition) => void
  duplicateSelection: () => void
  /** Reorder the selection among its siblings: front, back, or one step. */
  reorder: (how: 'front' | 'back' | 'forward' | 'backward') => void
  setContextMenu: (m: State['contextMenu']) => void
  hasClipboard: () => boolean
  selectedIds: () => string[]
  setTheme: (t: 'light' | 'dark') => void
  setShowLabels: (v: boolean) => void
  setShowMinimap: (v: boolean) => void
  setSnapToGrid: (v: boolean) => void
  setModal: (m: 'shortcuts' | 'about' | null) => void
  /** Replace the canvas with a document (opened file, import); unsaved until Save. */
  loadDocument: (doc: Document) => void
  newDiagram: () => void
  selectAll: () => void
  deselectAll: () => void
  deleteSelection: () => void
  currentDocument: () => Document
  setActiveProvider: (p: string) => void
  ensureEntry: (type: string) => Promise<Entry | undefined>
  loadGenerated: (provider: string) => Promise<void>
  setConnectingFrom: (id: string | null) => void
  setEdgeBinding: (edgeId: string, attr: string, output: string) => void
  /** Create a references edge from a node's attribute to a target node (from the settings panel). */
  linkAttribute: (sourceId: string, attr: string, targetId: string, output: string) => void
  loadAttachments: (type: string) => Promise<void>
  /** Add an attachment element inside parentId, bound to it through attr/output. */
  addAttachment: (parentId: string, opt: AttachmentOption) => Promise<void>
  isAttachment: (type: string) => boolean
  /** Switch a generated element to a sibling resource type of its family (properties reset to defaults). */
  changeNodeType: (id: string, type: string) => Promise<void>
  /** Provider of the real nodes (the source tab); '' when empty. */
  /** Type of the nearest non-transparent ancestor (groups do not count), or ROOT. */
  logicalParentType: (parentId: string | null | undefined) => string
  /** Properties an element placed under parentId inherits from zone boxes (prop -> value), limited to props the type declares. */
  providedProps: (type: string, parentId: string | null | undefined) => Record<string, string>
  /** Re-apply zone-box values to a node and everything inside it. */
  applyProvided: (rootId: string) => void
  /** Absolute rectangle of a node on the canvas (positions are parent-relative). */
  absRect: (id: string) => { x: number; y: number; w: number; h: number } | null
  /** Recompute which containers each spanning band covers and mirror that as reference edges. */
  syncSpans: () => void
  primaryProvider: () => string
  setMirror: (v: boolean) => void
  refreshProjection: () => Promise<void>
  /** Make the projected tab the source: its converted nodes become the document. */
  materialize: () => void
  /** True when the active tab is a projection (not the source). */
  isProjected: () => boolean
  selectEdge: (id: string | null) => void
  hoverEdge: (id: string | null) => void
  removeEdge: (id: string) => void
}

function stored<T>(key: string, fallback: T): T {
  try {
    const v = localStorage.getItem(key)
    return v === null ? fallback : (JSON.parse(v) as T)
  } catch {
    return fallback
  }
}
function persist(key: string, v: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(v))
  } catch {
    /* private mode */
  }
}
const systemDark = typeof matchMedia === 'function' && matchMedia('(prefers-color-scheme: dark)').matches
const PROVIDER_NAME: Record<string, string> = { aws: 'AWS', gcp: 'Google Cloud', azure: 'Azure' }

interface Clipboard {
  nodes: RFNode[]
  edges: RFEdge[]
}
let clipboard: Clipboard | null = null
let pasteCount = 0

interface Snapshot {
  nodes: RFNode[]
  edges: RFEdge[]
}

const HISTORY_LIMIT = 100
let lastCommitKey: string | undefined
let lastCommitAt = 0

let validateTimer: ReturnType<typeof setTimeout> | undefined

export const useStore = create<State>((set, get) => ({
  catalog: null,
  rules: null,
  docName: '',
  steps: [],
  showLegend: stored('iagram.legend', false),
  showGallery: false,
  templates: null,
  nodes: [],
  edges: [],
  selectedId: null,
  problems: [],
  dirty: false,
  saving: false,
  error: null,
  draggingType: null,
  toast: null,
  past: [],
  future: [],
  plan: null,
  planStale: false,
  drift: null,
  job: null,
  jobLines: [],
  logOpen: false,
  confirmApply: false,
  lastApply: null,
  theme: stored<'light' | 'dark'>('iagram.theme', systemDark ? 'dark' : 'light'),
  showLabels: stored('iagram.labels', true),
  showMinimap: stored('iagram.minimap', false),
  snapToGrid: stored('iagram.snap', true),
  version: '',
  modal: null,
  activeProvider: stored('iagram.provider', ''),
  catalogVersion: 0,
  generatedByProvider: {},
  familiesByProvider: {},
  mirror: stored('iagram.mirror', true),
  projection: null,
  projecting: false,
  connectingFrom: null,
  attachmentsByType: {},
  contextMenu: null,
  selectedEdgeId: null,
  hoveredEdgeId: null,
  pendingSelect: null,

  async load() {
    try {
      const [catalog, res] = await Promise.all([api.catalog(), api.document()])
      const rules = new Rules(catalog)
      const missing = [...new Set(res.document.nodes.map((n) => n.type).filter((t) => !rules.entry(t)))]
      if (missing.length) {
        const r = await api.resolve(missing).catch(() => ({ entries: {} }))
        for (const e of Object.values(r.entries)) rules.register(e)
      }
      const { nodes, edges } = fromDocument(rules, res.document)
      const providers = [...new Set(catalog.entries.map((e) => e.provider))].filter((p) => p !== COMMON).sort()
      const used = [...new Set(res.document.nodes.map((n) => n.type.split('.')[0]))].filter((p) => p !== COMMON)
      const current = get().activeProvider
      const activeProvider = providers.includes(current) ? current : used.find((p) => providers.includes(p)) ?? providers[0] ?? ''
      set({ catalog, rules, nodes, edges, steps: res.document.steps ?? [], showGallery: !get().catalog && new URLSearchParams(location.search).get('gallery') !== '0', docName: res.document.name ?? '', problems: res.validation.problems, dirty: false, error: null, past: [], future: [], activeProvider, projection: null })
      void get().refreshProjection()
      api.latestPlan().then((r) => set({ ...(r.plan ? { plan: r.plan, planStale: false } : {}), drift: r.drift })).catch(() => undefined)
      api.health().then((h) => set({ version: h.version })).catch(() => undefined)
    } catch (e) {
      set({ error: (e as Error).message })
    }
  },

  async save() {
    const { docName, nodes, edges, steps } = get()
    set({ saving: true })
    try {
      const res = await api.save(toDocument(docName, nodes, edges, steps))
      set({ problems: res.validation.problems, dirty: false, saving: false, error: null })
      get().showToast('Saved iagram.json')
    } catch (e) {
      set({ saving: false, error: (e as Error).message })
    }
  },

  validateSoon() {
    clearTimeout(validateTimer)
    validateTimer = setTimeout(async () => {
      const { docName, nodes, edges, steps } = get()
      try {
        const v = await api.validate(toDocument(docName, nodes, edges, steps))
        set({ problems: v.problems })
      } catch (e) {
        set({ error: (e as Error).message })
      }
    }, 350)
  },

  onNodesChange(changes) {
    if (get().isProjected() && changes.some((c) => c.type !== 'select' && c.type !== 'dimensions')) {
      // Editing a mirrored tab makes it the source first.
      get().materialize()
    }
    // Position/dimension churn is not "dirty" until the drag ends; selection never is.
    // Only user actions dirty the document: a finished drag, a resize that set
    // attributes, add/replace. Initial measurement and selection never do.
    const structural = changes.some(
      (c) => (c.type === 'position' && !c.dragging) || (c.type === 'dimensions' && !!c.setAttributes && !c.resizing) || c.type === 'add' || c.type === 'replace',
    )
    const removed = changes.filter((c) => c.type === 'remove').map((c) => c.id)
    if (removed.length) {
      get().removeNodes(removed) // commits
      const rest = changes.filter((c) => c.type !== 'remove')
      if (rest.length) set({ nodes: applyNodeChanges(rest, get().nodes) })
      return
    }
    set({ nodes: applyNodeChanges(changes, get().nodes), ...(structural ? { dirty: true } : {}) })
    if (structural) get().validateSoon()
  },

  onEdgesChange(changes) {
    if (get().isProjected() && changes.some((c) => c.type !== 'select')) get().materialize()
    const structural = changes.some((c) => c.type !== 'select')
    if (changes.some((c) => c.type === 'remove')) get().commit()
    set({ edges: applyEdgeChanges(changes, get().edges), ...(structural ? { dirty: true } : {}) })
    if (structural) get().validateSoon()
  },

  addNode(type, parentId, position) {
    if (get().isProjected()) get().materialize()
    const { rules, nodes } = get()
    if (!rules) return null
    const entry = rules.entry(type)
    if (!entry) return null
    const parentType = get().logicalParentType(parentId)
    if (!rules.canContain(parentType, type)) {
      const where = parentType === ROOT ? 'directly on the canvas' : `inside a ${rules.entry(parentType)?.label ?? parentType}`
      get().showToast(`${entry.label} cannot be placed ${where}`)
      return null
    }
    get().commit()
    const count = nodes.filter((n) => n.data.type === type).length + 1
    const id = newId(entry)
    const node = makeNode(rules, {
      id,
      type,
      name: `${entry.label.toLowerCase().replace(/[^a-z0-9]+/g, '-')}-${count}`,
      parent: parentId ?? undefined,
      props: { ...rules.defaults(type), ...get().providedProps(type, parentId) },
      layout: { x: Math.round(position.x), y: Math.round(position.y) },
    })
    // A spanning band sits above the containers it covers, below leaves.
    if (entry.span) {
      const siblings = nodes.filter((n) => (n.parentId ?? null) === (parentId ?? null) && n.type === 'container')
      const z = Math.max(0, ...siblings.map((n) => n.data.z ?? 0)) + 1
      node.data.z = z
      node.zIndex = zIndexFor(true, z)
    }
    set({ nodes: [...nodes, node], dirty: true, selectedId: id })
    if (entry.span) get().syncSpans()
    get().validateSoon()
    return id
  },

  connect(c) {
    if (get().isProjected()) get().materialize()
    const { rules, nodes, edges } = get()
    if (!rules || !c.source || !c.target) return
    const src = nodes.find((n) => n.id === c.source)
    const dst = nodes.find((n) => n.id === c.target)
    if (!src || !dst) return
    const rule = rules.connection(src.data.type, dst.data.type)
    if (!rule) return
    if (edges.some((e) => e.source === c.source && e.target === c.target)) return
    get().commit()
    const link = rule.kind === 'link' ? { type: rule.type, name: `${src.data.name}-${dst.data.name}`.replace(/[^A-Za-z0-9_-]+/g, '_'), props: { ...(rules.links.find((l) => l.id === rule.type)?.defaults ?? {}) } } : {}
    const edge = makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: rule.kind, source: c.source, target: c.target, ...link }, rule.label ?? (rule.kind === 'flow' ? '' : rule.kind), rule.style)
    set({ edges: addEdge(edge, edges), dirty: true })
    get().validateSoon()
    // The attachment's configuration opens right away (attribute picker for references).
    rfStore()?.getState().resetSelectedElements()
    set({ selectedId: null, selectedEdgeId: edge.id })
  },

  updateNode(id, patch) {
    if (get().isProjected()) get().materialize()
    // typing in one field coalesces into a single undo step
    get().commit(`update:${id}:${patch.name !== undefined ? 'name' : patch.caption !== undefined ? 'caption' : patch.step !== undefined ? 'step' : Object.keys(patch.props ?? {}).join(',')}`)
    set({
      nodes: get().nodes.map((n) =>
        n.id === id
          ? {
              ...n,
              data: {
                ...n.data,
                ...(patch.name !== undefined ? { name: patch.name } : {}),
                ...(patch.props ? { props: patch.props } : {}),
                ...(patch.caption !== undefined ? { caption: patch.caption || undefined } : {}),
                ...(patch.step !== undefined ? { step: patch.step || undefined } : {}),
              },
            }
          : n,
      ),
      dirty: true,
    })
    get().validateSoon()
  },

  removeNodes(ids) {
    if (get().isProjected()) get().materialize()
    get().commit()
    const { nodes, edges, selectedId } = get()
    const doomed = new Set(ids)
    // cascade to descendants
    let grew = true
    while (grew) {
      grew = false
      for (const n of nodes) {
        if (n.parentId && doomed.has(n.parentId) && !doomed.has(n.id)) {
          doomed.add(n.id)
          grew = true
        }
      }
    }
    set({
      nodes: nodes.filter((n) => !doomed.has(n.id)),
      edges: edges.filter((e) => !doomed.has(e.source) && !doomed.has(e.target)),
      selectedId: selectedId && doomed.has(selectedId) ? null : selectedId,
      dirty: true,
    })
    get().validateSoon()
  },

  select(id) {
    // selectedId mirrors React Flow's selection; the flags on nodes are owned
    // by React Flow's change pipeline (see requestSelect for programmatic use).
    if (id !== get().selectedId) set({ selectedId: id })
  },

  requestSelect(id) {
    set({ pendingSelect: id })
  },

  setDragging(type) {
    set({ draggingType: type })
  },

  showToast(msg) {
    set({ toast: msg })
    setTimeout(() => set((s) => (s.toast === msg ? { toast: null } : {})), 2500)
  },

  commit(key) {
    const now = Date.now()
    if (key && key === lastCommitKey && now - lastCommitAt < 1000) {
      lastCommitAt = now
      return
    }
    lastCommitKey = key
    lastCommitAt = now
    const { nodes, edges, past, plan } = get()
    set({ past: [...past.slice(-(HISTORY_LIMIT - 1)), { nodes, edges }], future: [], ...(plan ? { planStale: true } : {}) })
  },

  undo() {
    const { past, future, nodes, edges } = get()
    const prev = past[past.length - 1]
    if (!prev) return
    lastCommitKey = undefined
    set({ nodes: prev.nodes, edges: prev.edges, past: past.slice(0, -1), future: [{ nodes, edges }, ...future], dirty: true, selectedId: null })
    get().validateSoon()
  },

  async runPlan() {
    const { dirty, save, problems } = get()
    if (problems.some((p) => p.level === 'error')) {
      get().showToast('Fix validation errors before planning')
      return
    }
    if (dirty) await save()
    if (get().dirty) return // save failed
    await streamJob(set, get, api.startPlan, (done) => {
      if (done.status === 'succeeded' && done.result) set({ plan: done.result as PlanResult, planStale: false })
      else if (done.status === 'failed') get().showToast('Plan failed; see log')
    })
  },

  async runDestroyPlan() {
    const { dirty, save } = get()
    if (dirty) await save()
    if (get().dirty) return
    await streamJob(set, get, api.startDestroyPlan, (done) => {
      if (done.status === 'succeeded' && done.result) {
        const p = done.result as PlanResult
        set({ plan: { ...p, destroy: true }, planStale: false })
        if (p.changes) set({ confirmApply: true })
        else get().showToast('Nothing to destroy')
      } else if (done.status === 'failed') get().showToast('Destroy plan failed; see log')
    })
  },

  async runApply() {
    set({ confirmApply: false })
    await streamJob(set, get, api.startApply, (done) => {
      set({ plan: null, planStale: false })
      if (done.status === 'succeeded' && done.result) {
        set({ lastApply: done.result as ApplyResult })
        get().showToast(`Applied; outputs written for ${(done.result as ApplyResult).nodes_updated} node(s)`)
        void get().load() // the file now carries outputs
      } else if (done.status === 'failed') {
        get().showToast('Apply failed; see log')
      }
    })
  },

  async runDrift() {
    const { dirty, save } = get()
    if (dirty) await save()
    if (get().dirty) return
    await streamJob(set, get, api.startDrift, (done) => {
      if (done.status === 'succeeded' && done.result) {
        const d = done.result as DriftResult
        set({ drift: d })
        get().showToast(d.drift ? 'Drift detected' : 'No drift: infrastructure matches state')
      } else if (done.status === 'failed') {
        get().showToast('Drift check failed; see log')
      }
    })
  },

  async cancelPlan() {
    const { job } = get()
    if (job?.status === 'running') await api.cancelJob(job.id)
  },

  clearPlan() {
    set({ plan: null, planStale: false })
  },

  async clearDrift() {
    await api.clearDrift().catch(() => undefined)
    set({ drift: null })
  },

  setConfirmApply(open) {
    set({ confirmApply: open })
  },

  setTheme(t) {
    persist('iagram.theme', t)
    set({ theme: t })
  },

  setShowLabels(v) {
    persist('iagram.labels', v)
    set({ showLabels: v })
  },

  setShowMinimap(v) {
    persist('iagram.minimap', v)
    set({ showMinimap: v })
  },

  setSnapToGrid(v) {
    persist('iagram.snap', v)
    set({ snapToGrid: v })
  },

  setModal(m) {
    set({ modal: m })
  },

  loadDocument(doc) {
    const { rules } = get()
    if (!rules) return
    const missing = [...new Set(doc.nodes.map((n) => n.type).filter((t) => !rules.entry(t)))]
    if (missing.length) {
      void api.resolve(missing).then((r) => {
        for (const e of Object.values(r.entries)) rules.register(e)
        set({ catalogVersion: get().catalogVersion + 1 })
      })
    }
    get().commit()
    const { nodes, edges } = fromDocument(rules, doc)
    set({ nodes, edges, steps: doc.steps ?? [], docName: doc.name ?? get().docName, dirty: true, selectedId: null, selectedEdgeId: null, plan: null, planStale: false })
    get().validateSoon()
    get().showToast(`Loaded ${nodes.length} elements; Save to write iagram.json`)
  },

  newDiagram() {
    get().commit()
    set({ nodes: [], edges: [], dirty: true, selectedId: null, selectedEdgeId: null, plan: null, planStale: false, problems: [] })
  },

  selectAll() {
    rfStore()?.getState().addSelectedNodes(get().nodes.map((n) => n.id))
  },

  deselectAll() {
    rfStore()?.getState().resetSelectedElements()
    set({ selectedId: null, selectedEdgeId: null })
  },

  deleteSelection() {
    const nodeIds = get().selectedIds()
    const edgeIds = get().edges.filter((e) => e.selected).map((e) => e.id)
    if (nodeIds.length) get().removeNodes(nodeIds)
    else if (edgeIds.length) {
      get().commit()
      set({ edges: get().edges.filter((e) => !edgeIds.includes(e.id)), dirty: true, selectedEdgeId: null })
      get().validateSoon()
    } else if (get().selectedEdgeId) get().removeEdge(get().selectedEdgeId!)
  },

  currentDocument() {
    const { docName, nodes, edges, steps } = get()
    return toDocument(docName, nodes, edges, steps)
  },

  setActiveProvider(p) {
    persist('iagram.provider', p)
    rfStore()?.getState().resetSelectedElements()
    set({ activeProvider: p, selectedId: null, selectedEdgeId: null, projection: null })
    void get().refreshProjection()
  },

  providedProps(type, parentId) {
    const { rules, nodes } = get()
    const entry = rules?.entry(type)
    if (!rules || !entry) return {}
    const declared = entry.props?.properties ?? {}
    const chain: RFNode[] = []
    const seen = new Set<string>()
    let cur = parentId ? nodes.find((n) => n.id === parentId) : undefined
    while (cur && !seen.has(cur.id)) {
      seen.add(cur.id)
      chain.push(cur)
      cur = cur.parentId ? nodes.find((n) => n.id === cur!.parentId) : undefined
    }
    const out: Record<string, string> = {}
    chain.forEach((anc, i) => {
      const provides = rules.entry(anc.data.type)?.provides
      if (!provides) return
      for (const [prop, tpl] of Object.entries(provides)) {
        if (!(prop in declared) || prop in out) continue
        let ok = true
        const v = tpl.replace(/\$\{([A-Za-z0-9_]+)\}/g, (_, key: string) => {
          for (const a of chain.slice(i)) {
            const val = a.data.props[key]
            if (val !== undefined && val !== null && val !== '') return String(val)
          }
          ok = false
          return ''
        })
        if (ok) out[prop] = v
      }
    })
    return out
  },

  logicalParentType(parentId) {
    const { rules, nodes } = get()
    let cur = parentId ? nodes.find((n) => n.id === parentId) : undefined
    const seen = new Set<string>()
    while (cur && rules?.isTransparent(cur.data.type) && !seen.has(cur.id)) {
      seen.add(cur.id)
      cur = cur.parentId ? nodes.find((n) => n.id === cur!.parentId) : undefined
    }
    return cur ? cur.data.type : ROOT
  },

  primaryProvider() {
    const counts: Record<string, number> = {}
    for (const n of get().nodes) {
      const p = n.data.type.split('.')[0]
      if (p === COMMON) continue
      counts[p] = (counts[p] ?? 0) + 1
    }
    return Object.entries(counts).sort((a, b) => b[1] - a[1])[0]?.[0] ?? ''
  },

  isProjected() {
    const { mirror, activeProvider, nodes } = get()
    if (!mirror || nodes.length === 0) return false
    const primary = get().primaryProvider()
    // A file that already mixes providers is edited as independent canvases.
    const providers = new Set(nodes.map((n) => n.data.type.split('.')[0]).filter((p) => p !== COMMON))
    if (providers.size > 1) return false
    return activeProvider !== primary
  },

  setMirror(v) {
    persist('iagram.mirror', v)
    set({ mirror: v, projection: null })
    void get().refreshProjection()
  },

  async refreshProjection() {
    const { rules, activeProvider } = get()
    if (!rules || !get().isProjected()) {
      set({ projection: null })
      return
    }
    set({ projecting: true })
    try {
      const r = await api.convert(activeProvider, get().currentDocument())
      if (get().activeProvider !== activeProvider) return
      const missing = [...new Set(r.document.nodes.map((n) => n.type).filter((t) => !rules.entry(t)))]
      if (missing.length) {
        const res = await api.resolve(missing).catch(() => ({ entries: {} }))
        for (const e of Object.values(res.entries)) rules.register(e)
      }
      const { nodes, edges } = fromDocument(rules, r.document)
      set({ projection: { provider: activeProvider, nodes, edges, report: r.report }, projecting: false })
    } catch (e) {
      set({ projecting: false, error: (e as Error).message })
    }
  },

  materialize() {
    const { projection, activeProvider } = get()
    if (!projection || projection.provider !== activeProvider) return
    get().commit()
    set({ nodes: projection.nodes, edges: projection.edges, projection: null, dirty: true, plan: null, planStale: false })
    get().showToast(`${PROVIDER_NAME[activeProvider] ?? activeProvider} is now the source; other tabs mirror it`)
    get().validateSoon()
  },

  async ensureEntry(type) {
    const { rules } = get()
    if (!rules) return undefined
    const have = rules.entry(type)
    if (have) return have
    try {
      const e = await api.entry(type)
      rules.register(e)
      set({ catalogVersion: get().catalogVersion + 1 })
      return e
    } catch {
      return undefined
    }
  },

  async loadGenerated(provider) {
    if (get().generatedByProvider[provider]) return
    try {
      const r = await api.generated(provider)
      set({ generatedByProvider: { ...get().generatedByProvider, [provider]: r.elements }, familiesByProvider: { ...get().familiesByProvider, [provider]: r.families } })
    } catch {
      set({ generatedByProvider: { ...get().generatedByProvider, [provider]: [] }, familiesByProvider: { ...get().familiesByProvider, [provider]: [] } })
    }
  },

  setConnectingFrom(id) {
    set({ connectingFrom: id })
  },

  isAttachment(type) {
    return !!get().rules?.entry(type)?.attachment
  },

  async changeNodeType(id, type) {
    const entry = await get().ensureEntry(type)
    const { rules, nodes } = get()
    if (!entry || !rules) return
    const node = nodes.find((n) => n.id === id)
    if (!node || node.data.type === type) return
    get().commit()
    // Attachments and references bound to the old type no longer apply.
    const attached = nodes.filter((n) => n.parentId === id && !!rules.entry(n.data.type)?.attachment).map((n) => n.id)
    const drop = new Set([id, ...attached])
    set({
      nodes: nodes.filter((n) => !attached.includes(n.id)).map((n) => (n.id === id ? { ...n, data: { ...n.data, type, props: rules.defaults(type), outputs: undefined } } : n)),
      edges: get().edges.filter((e) => !(drop.has(e.source) || attached.includes(e.target))),
      dirty: true,
      catalogVersion: get().catalogVersion + 1,
    })
    get().validateSoon()
  },

  async loadAttachments(type) {
    if (get().attachmentsByType[type]) return
    try {
      const r = await api.attachments(type)
      set({ attachmentsByType: { ...get().attachmentsByType, [type]: r.attachments } })
    } catch {
      set({ attachmentsByType: { ...get().attachmentsByType, [type]: [] } })
    }
  },

  async addAttachment(parentId, opt) {
    const entry = await get().ensureEntry(opt.id)
    const { rules, nodes } = get()
    if (!entry || !rules) return
    const parent = nodes.find((n) => n.id === parentId)
    if (!parent) return
    get().commit()
    const count = nodes.filter((n) => n.data.type === opt.id && n.parentId === parentId).length + 1
    const id = newId(entry)
    // Components are drawn inside the parent box: tile them left to right.
    const siblings = nodes.filter((n) => n.parentId === parentId && !!rules.entry(n.data.type)?.component).length
    const node = makeNode(rules, {
      id,
      type: opt.id,
      name: `${parent.data.name}-${opt.resource.split('_').slice(-1)[0]}${count > 1 ? `-${count}` : ''}`,
      parent: parentId,
      props: rules.defaults(opt.id),
      layout: entry.component ? { x: 20 + (siblings % 3) * 140, y: 50 + Math.floor(siblings / 3) * 110 } : { x: 0, y: 0 },
    })
    const edge = makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: 'references', source: id, target: parentId, attr: opt.attr, output: opt.output }, opt.attr)
    set({ nodes: [...get().nodes, node], edges: [...get().edges, edge], dirty: true })
    get().validateSoon()
  },

  setEdgeBinding(edgeId, attr, output) {
    get().commit(`edge:${edgeId}`)
    set({
      edges: get().edges.map((e) => (e.id === edgeId ? { ...e, data: { ...(e.data ?? { kind: 'references', label: 'references', ruleLabel: 'references' }), attr, output, ruleLabel: attr || 'references', label: e.data?.userLabel || attr || 'references' } } : e)),
      dirty: true,
    })
    get().validateSoon()
  },

  updateEdge(id, patch) {
    if (get().isProjected()) get().materialize()
    get().commit(`edge:${id}:${Object.keys(patch).join(',')}`)
    set({
      edges: get().edges.map((e) => {
        if (e.id !== id || !e.data) return e
        const style = patch.style !== undefined ? Object.fromEntries(Object.entries({ ...(e.data.style ?? {}), ...patch.style }).filter(([, v]) => v !== undefined && v !== '' && v !== false)) : e.data.style
        const userLabel = patch.label !== undefined ? patch.label || undefined : e.data.userLabel
        let ruleLabel = e.data.ruleLabel
        let ruleStyle = e.data.ruleStyle
        if (patch.type !== undefined && patch.type !== e.data.type) {
          // Another link resource for the same pair: relabel, restyle, reset its attributes.
          const src = get().nodes.find((n) => n.id === e.source)
          const dst = get().nodes.find((n) => n.id === e.target)
          const r = src && dst ? get().rules?.linkRule(src.data.type, dst.data.type, patch.type) : undefined
          if (r) (ruleLabel = r.label ?? ruleLabel), (ruleStyle = r.style)
        }
        const data = {
          ...e.data,
          userLabel,
          ruleLabel,
          ruleStyle,
          label: userLabel || ruleLabel,
          ...(patch.step !== undefined ? { step: patch.step || undefined } : {}),
          ...(patch.type !== undefined ? { type: patch.type, props: patch.type !== e.data.type ? {} : e.data.props } : {}),
          ...(patch.name !== undefined ? { name: patch.name || undefined } : {}),
          ...(patch.props !== undefined ? { props: patch.props } : {}),
          style,
        }
        return withMarkers({ ...e, data })
      }),
      dirty: true,
    })
    if (patch.type !== undefined || patch.props !== undefined || patch.name !== undefined) get().validateSoon()
  },

  setSteps(steps) {
    if (get().isProjected()) get().materialize()
    get().commit('steps')
    set({ steps, dirty: true })
  },

  setShowGallery(v) {
    set({ showGallery: v })
  },

  async loadTemplates() {
    if (get().templates) return
    try {
      const r = await api.templates()
      set({ templates: r.templates })
    } catch (e) {
      set({ templates: [] })
      get().showToast((e as Error).message)
    }
  },

  async useTemplate(t) {
    if (get().nodes.length > 0 && !confirm(`Replace the current canvas with "${t.title}"? Your file is not touched until you save.`)) return
    // Generated element types need their schemas before the canvas can draw them.
    const missing = [...new Set(t.document.nodes.map((n) => n.type).filter((x) => !get().rules?.entry(x)))]
    if (missing.length && get().rules) {
      const r = await api.resolve(missing).catch(() => ({ entries: {} }))
      for (const e of Object.values(r.entries)) get().rules!.register(e)
    }
    get().loadDocument(t.document)
    set({ showGallery: false, activeProvider: t.provider, docName: get().docName || t.title })
    void get().refreshProjection()
    get().showToast(`Opened "${t.title}". Adapt it, then Save.`)
  },

  setShowLegend(v) {
    persist('iagram.legend', v)
    set({ showLegend: v })
  },

  linkAttribute(sourceId, attr, targetId, output) {
    const { rules, nodes, edges } = get()
    const src = nodes.find((n) => n.id === sourceId)
    const dst = nodes.find((n) => n.id === targetId)
    if (!rules || !src || !dst) return
    const rule = rules.connection(src.data.type, dst.data.type)
    if (!rule) {
      get().showToast('These elements cannot be linked')
      return
    }
    get().commit()
    const edge = makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: rule.kind, source: sourceId, target: targetId, attr, output }, attr, rule.style)
    set({ edges: [...edges, edge], dirty: true, selectedEdgeId: edge.id, selectedId: null })
    rfStore()?.getState().resetSelectedElements()
    get().validateSoon()
  },

  selectEdge(id) {
    set({ selectedEdgeId: id, ...(id ? { selectedId: null } : {}) })
  },

  hoverEdge(id) {
    set({ hoveredEdgeId: id })
  },

  removeEdge(id) {
    get().commit()
    set({ edges: get().edges.filter((e) => e.id !== id), selectedEdgeId: null, dirty: true })
    get().validateSoon()
  },

  reparent(id, parentId, position) {
    // The drag already committed a snapshot at drag start; do not commit again.
    set({
      nodes: get().nodes.map((n) => (n.id === id ? { ...n, parentId: parentId ?? undefined, position } : n)),
      dirty: true,
    })
    get().applyProvided(id)
    get().validateSoon()
  },

  absRect(id) {
    const nodes = get().nodes
    let cur = nodes.find((n) => n.id === id)
    if (!cur) return null
    const w = Number(cur.style?.width ?? cur.measured?.width ?? 120)
    const h = Number(cur.style?.height ?? cur.measured?.height ?? 90)
    let x = cur.position.x
    let y = cur.position.y
    const seen = new Set<string>()
    while (cur?.parentId && !seen.has(cur.id)) {
      seen.add(cur.id)
      cur = nodes.find((n) => n.id === cur!.parentId)
      if (!cur) break
      x += cur.position.x
      y += cur.position.y
    }
    return { x, y, w, h }
  },

  syncSpans() {
    const { rules, nodes, edges } = get()
    if (!rules) return
    const spans = nodes.filter((n) => rules.entry(n.data.type)?.span)
    if (spans.length === 0) return
    let next = edges
    let changed = false
    for (const band of spans) {
      const span = rules.entry(band.data.type)!.span!
      const r = get().absRect(band.id)
      if (!r) continue
      const covered = new Set<string>()
      for (const n of nodes) {
        if (n.id === band.id || !span.elements.includes(n.data.type)) continue
        const c = get().absRect(n.id)
        if (!c) continue
        // Covered when the band overlaps at least 40% of the container's area.
        const ix = Math.max(0, Math.min(r.x + r.w, c.x + c.w) - Math.max(r.x, c.x))
        const iy = Math.max(0, Math.min(r.y + r.h, c.y + c.h) - Math.max(r.y, c.y))
        if (ix * iy >= 0.4 * c.w * c.h) covered.add(n.id)
      }
      const mine = next.filter((e) => e.source === band.id && e.data?.kind === 'references' && e.data.attr === span.attr)
      const have = new Set(mine.map((e) => e.target))
      const stale = mine.filter((e) => !covered.has(e.target)).map((e) => e.id)
      if (stale.length) (next = next.filter((e) => !stale.includes(e.id))), (changed = true)
      for (const target of covered) {
        if (have.has(target)) continue
        const t = nodes.find((n) => n.id === target)!
        next = [...next, makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: 'references', source: band.id, target, attr: span.attr, output: span.outputs[t.data.type] ?? 'id' }, span.attr, { dash: 'dotted' })]
        changed = true
      }
    }
    if (changed) {
      set({ edges: next, dirty: true })
      get().validateSoon()
    }
  },

  applyProvided(rootId) {
    // Zone boxes give their value to everything drawn inside: refresh the
    // moved node and its descendants.
    const nodes = get().nodes
    const inside = new Set<string>([rootId])
    let grew = true
    while (grew) {
      grew = false
      for (const n of nodes) if (n.parentId && inside.has(n.parentId) && !inside.has(n.id)) (inside.add(n.id), (grew = true))
    }
    let changed = false
    const next = nodes.map((n) => {
      if (!inside.has(n.id)) return n
      const given = get().providedProps(n.data.type, n.parentId)
      const patch: Record<string, unknown> = {}
      for (const [k, v] of Object.entries(given)) if (n.data.props[k] !== v) patch[k] = v
      if (Object.keys(patch).length === 0) return n
      changed = true
      return { ...n, data: { ...n.data, props: { ...n.data.props, ...patch } } }
    })
    if (changed) set({ nodes: next, dirty: true })
  },

  selectedIds() {
    return get().nodes.filter((n) => n.selected).map((n) => n.id)
  },

  copySelection() {
    const { nodes, edges } = get()
    const ids = new Set(get().selectedIds())
    if (ids.size === 0) return
    // Include everything inside selected containers.
    let grew = true
    while (grew) {
      grew = false
      for (const n of nodes) if (n.parentId && ids.has(n.parentId) && !ids.has(n.id)) (ids.add(n.id), (grew = true))
    }
    clipboard = {
      nodes: nodes.filter((n) => ids.has(n.id)).map((n) => ({ ...n, selected: false, data: { ...n.data, props: { ...n.data.props }, outputs: undefined } })),
      edges: edges.filter((e) => ids.has(e.source) && ids.has(e.target)).map((e) => ({ ...e, selected: false })),
    }
    pasteCount = 0
    get().showToast(`Copied ${clipboard.nodes.length} element${clipboard.nodes.length === 1 ? '' : 's'}`)
  },

  paste(at) {
    const { rules } = get()
    if (!clipboard || !rules) return
    pasteCount++
    const offset = 40 * pasteCount
    // Paste at a point: shift the copied top-level nodes so their bounding box starts there.
    const tops = clipboard.nodes.filter((n) => !n.parentId || !clipboard!.nodes.some((m) => m.id === n.parentId))
    const minX = Math.min(...tops.map((n) => n.position.x))
    const minY = Math.min(...tops.map((n) => n.position.y))
    const shift = at ? { x: at.x - minX, y: at.y - minY } : { x: offset, y: offset }
    const idMap = new Map<string, string>()
    for (const n of clipboard.nodes) idMap.set(n.id, newId(rules.entry(n.data.type)!))
    const existingNames = new Set(get().nodes.map((n) => n.data.name))
    get().commit()
    const nodes: RFNode[] = clipboard.nodes.map((n) => {
      const parentInside = n.parentId && idMap.has(n.parentId)
      let name = n.data.name
      if (!parentInside || !n.parentId) {
        // top-level pasted nodes get a fresh name; nested ones keep theirs (they live in a new parent)
        name = uniqueName(n.data.name, existingNames)
        existingNames.add(name)
      }
      return {
        ...n,
        id: idMap.get(n.id)!,
        parentId: parentInside ? idMap.get(n.parentId!) : n.parentId,
        position: parentInside ? n.position : { x: n.position.x + shift.x, y: n.position.y + shift.y },
        selected: true,
        data: { ...n.data, name, props: { ...n.data.props } },
      }
    })
    const edges: RFEdge[] = clipboard.edges.map((e) => ({ ...e, id: `e-${Math.random().toString(36).slice(2, 8)}`, source: idMap.get(e.source)!, target: idMap.get(e.target)! }))
    set({ nodes: [...get().nodes.map((n) => ({ ...n, selected: false })), ...nodes], edges: [...get().edges, ...edges], dirty: true, selectedId: nodes.length === 1 ? nodes[0].id : null })
    get().validateSoon()
  },

  duplicateSelection() {
    get().copySelection()
    if (clipboard) get().paste()
  },

  cutSelection() {
    const ids = get().selectedIds()
    if (ids.length === 0) return
    get().copySelection()
    get().removeNodes(ids)
    get().showToast(`Cut ${ids.length} element${ids.length === 1 ? '' : 's'}`)
  },

  hasClipboard() {
    return !!clipboard
  },

  reorder(how) {
    const ids = new Set(get().selectedIds())
    if (ids.size === 0) return
    get().commit()
    const nodes = get().nodes
    const zOf = (n: RFNode) => n.data.z ?? 0
    const next = nodes.map((n) => {
      if (!ids.has(n.id)) return n
      const siblings = nodes.filter((m) => m.parentId === n.parentId && m.type === n.type && m.id !== n.id)
      let z = zOf(n)
      switch (how) {
        case 'front':
          z = Math.max(0, ...siblings.map(zOf)) + 1
          break
        case 'back':
          z = Math.min(0, ...siblings.map(zOf)) - 1
          break
        case 'forward':
          z = zOf(n) + 1
          break
        case 'backward':
          z = zOf(n) - 1
          break
      }
      return { ...n, data: { ...n.data, z }, zIndex: zIndexFor(n.type === 'container', z) }
    })
    set({ nodes: next, dirty: true })
  },

  setContextMenu(m) {
    set({ contextMenu: m })
  },

  setLogOpen(open) {
    set({ logOpen: open })
  },

  redo() {
    const { past, future, nodes, edges } = get()
    const next = future[0]
    if (!next) return
    lastCommitKey = undefined
    set({ nodes: next.nodes, edges: next.edges, past: [...past, { nodes, edges }], future: future.slice(1), dirty: true, selectedId: null })
    get().validateSoon()
  },
}))

function uniqueName(base: string, taken: Set<string>): string {
  const stem = base.replace(/-copy(-\d+)?$/, '')
  let candidate = `${stem}-copy`
  for (let i = 2; taken.has(candidate); i++) candidate = `${stem}-copy-${i}`
  return candidate
}

/** Start a job and stream its log into the store; onDone runs with the final job. */
async function streamJob(set: (p: Partial<State>) => void, get: () => State, start: () => Promise<Job>, onDone: (j: Job) => void) {
  set({ jobLines: [], logOpen: true, error: null })
  let job: Job
  try {
    job = await start()
  } catch (e) {
    set({ error: (e as Error).message })
    return
  }
  set({ job })
  const es = new EventSource(`/api/jobs/${job.id}/stream`)
  es.addEventListener('line', (ev) => {
    const line = JSON.parse((ev as MessageEvent).data) as string
    set({ jobLines: [...get().jobLines, line] })
  })
  es.addEventListener('done', (ev) => {
    es.close()
    const done = JSON.parse((ev as MessageEvent).data) as Job
    set({ job: done })
    onDone(done)
  })
  es.onerror = () => {
    es.close()
    void api.job(job.id).then((j) => {
      set({ job: j })
      if (j.status !== 'running') onDone(j)
    })
  }
}

/** Problems indexed by node id and edge id, computed once per render. */
export function indexProblems(problems: Problem[]) {
  const byNode: Record<string, Problem[]> = {}
  const byEdge: Record<string, Problem[]> = {}
  for (const p of problems) {
    if (p.node) (byNode[p.node] ??= []).push(p)
    if (p.edge) (byEdge[p.edge] ??= []).push(p)
  }
  return { byNode, byEdge }
}
