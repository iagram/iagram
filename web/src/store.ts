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
import { fromDocument, makeEdge, makeNode, newId, toDocument, type RFEdge, type RFNode } from './convert'
import type { Document } from './types'
import { rfStore } from './rf'
import type { ApplyResult, Catalog, DriftResult, Entry, GeneratedSummary, Job, PlanResult, Problem } from './types'
import { ROOT } from './types'

interface State {
  catalog: Catalog | null
  rules: Rules | null
  docName: string
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
  connectingFrom: string | null
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
  updateNode: (id: string, patch: { name?: string; props?: Record<string, unknown> }) => void
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
  paste: () => void
  duplicateSelection: () => void
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
  connectingFrom: null,
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
      const providers = [...new Set(catalog.entries.map((e) => e.provider))].sort()
      const used = [...new Set(res.document.nodes.map((n) => n.type.split('.')[0]))]
      const current = get().activeProvider
      const activeProvider = providers.includes(current) ? current : used.find((p) => providers.includes(p)) ?? providers[0] ?? ''
      set({ catalog, rules, nodes, edges, docName: res.document.name ?? '', problems: res.validation.problems, dirty: false, error: null, past: [], future: [], activeProvider })
      api.latestPlan().then((r) => set({ ...(r.plan ? { plan: r.plan, planStale: false } : {}), drift: r.drift })).catch(() => undefined)
      api.health().then((h) => set({ version: h.version })).catch(() => undefined)
    } catch (e) {
      set({ error: (e as Error).message })
    }
  },

  async save() {
    const { docName, nodes, edges } = get()
    set({ saving: true })
    try {
      const res = await api.save(toDocument(docName, nodes, edges))
      set({ problems: res.validation.problems, dirty: false, saving: false, error: null })
      get().showToast('Saved iagram.json')
    } catch (e) {
      set({ saving: false, error: (e as Error).message })
    }
  },

  validateSoon() {
    clearTimeout(validateTimer)
    validateTimer = setTimeout(async () => {
      const { docName, nodes, edges } = get()
      try {
        const v = await api.validate(toDocument(docName, nodes, edges))
        set({ problems: v.problems })
      } catch (e) {
        set({ error: (e as Error).message })
      }
    }, 350)
  },

  onNodesChange(changes) {
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
    const structural = changes.some((c) => c.type !== 'select')
    if (changes.some((c) => c.type === 'remove')) get().commit()
    set({ edges: applyEdgeChanges(changes, get().edges), ...(structural ? { dirty: true } : {}) })
    if (structural) get().validateSoon()
  },

  addNode(type, parentId, position) {
    const { rules, nodes } = get()
    if (!rules) return null
    const entry = rules.entry(type)
    if (!entry) return null
    const parentType = parentId ? nodes.find((n) => n.id === parentId)?.data.type ?? '' : ROOT
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
      props: rules.defaults(type),
      layout: { x: Math.round(position.x), y: Math.round(position.y) },
    })
    set({ nodes: [...nodes, node], dirty: true, selectedId: id })
    get().validateSoon()
    return id
  },

  connect(c) {
    const { rules, nodes, edges } = get()
    if (!rules || !c.source || !c.target) return
    const src = nodes.find((n) => n.id === c.source)
    const dst = nodes.find((n) => n.id === c.target)
    if (!src || !dst) return
    const rule = rules.connection(src.data.type, dst.data.type)
    if (!rule) return
    if (edges.some((e) => e.source === c.source && e.target === c.target)) return
    get().commit()
    const edge = makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: rule.kind, source: c.source, target: c.target }, rule.label ?? rule.kind)
    set({ edges: addEdge(edge, edges), dirty: true })
    get().validateSoon()
    // The attachment's configuration opens right away (attribute picker for references).
    rfStore()?.getState().resetSelectedElements()
    set({ selectedId: null, selectedEdgeId: edge.id })
  },

  updateNode(id, patch) {
    // typing in one field coalesces into a single undo step
    get().commit(`update:${id}:${patch.name !== undefined ? 'name' : Object.keys(patch.props ?? {}).join(',')}`)
    set({
      nodes: get().nodes.map((n) =>
        n.id === id ? { ...n, data: { ...n.data, ...(patch.name !== undefined ? { name: patch.name } : {}), ...(patch.props ? { props: patch.props } : {}) } } : n,
      ),
      dirty: true,
    })
    get().validateSoon()
  },

  removeNodes(ids) {
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
    set({ nodes, edges, docName: doc.name ?? get().docName, dirty: true, selectedId: null, selectedEdgeId: null, plan: null, planStale: false })
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
    const { docName, nodes, edges } = get()
    return toDocument(docName, nodes, edges)
  },

  setActiveProvider(p) {
    persist('iagram.provider', p)
    rfStore()?.getState().resetSelectedElements()
    set({ activeProvider: p, selectedId: null, selectedEdgeId: null })
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
      set({ generatedByProvider: { ...get().generatedByProvider, [provider]: r.elements } })
    } catch {
      set({ generatedByProvider: { ...get().generatedByProvider, [provider]: [] } })
    }
  },

  setConnectingFrom(id) {
    set({ connectingFrom: id })
  },

  setEdgeBinding(edgeId, attr, output) {
    get().commit(`edge:${edgeId}`)
    set({
      edges: get().edges.map((e) => (e.id === edgeId ? { ...e, label: attr || e.data?.label, data: { ...(e.data ?? { kind: 'references', label: 'references' }), attr, output } } : e)),
      dirty: true,
    })
    get().validateSoon()
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
    const edge = makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: rule.kind, source: sourceId, target: targetId, attr, output }, attr)
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
    get().validateSoon()
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

  paste() {
    const { rules } = get()
    if (!clipboard || !rules) return
    pasteCount++
    const offset = 40 * pasteCount
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
        position: parentInside ? n.position : { x: n.position.x + offset, y: n.position.y + offset },
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
