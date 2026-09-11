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
import type { Catalog, Problem } from './types'
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
  setDragging: (type: string | null) => void
  showToast: (msg: string) => void
}

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

  async load() {
    try {
      const [catalog, res] = await Promise.all([api.catalog(), api.document()])
      const rules = new Rules(catalog)
      const { nodes, edges } = fromDocument(rules, res.document)
      set({ catalog, rules, nodes, edges, docName: res.document.name ?? '', problems: res.validation.problems, dirty: false, error: null })
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
      get().removeNodes(removed)
      const rest = changes.filter((c) => c.type !== 'remove')
      if (rest.length) set({ nodes: applyNodeChanges(rest, get().nodes) })
      return
    }
    set({ nodes: applyNodeChanges(changes, get().nodes), ...(structural ? { dirty: true } : {}) })
    if (structural) get().validateSoon()
  },

  onEdgesChange(changes) {
    const structural = changes.some((c) => c.type !== 'select')
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
    const edge = makeEdge({ id: `e-${Math.random().toString(36).slice(2, 8)}`, kind: rule.kind, source: c.source, target: c.target }, rule.label ?? rule.kind)
    set({ edges: addEdge(edge, edges), dirty: true })
    get().validateSoon()
  },

  updateNode(id, patch) {
    set({
      nodes: get().nodes.map((n) =>
        n.id === id ? { ...n, data: { ...n.data, ...(patch.name !== undefined ? { name: patch.name } : {}), ...(patch.props ? { props: patch.props } : {}) } } : n,
      ),
      dirty: true,
    })
    get().validateSoon()
  },

  removeNodes(ids) {
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
    set({ selectedId: id })
  },

  setDragging(type) {
    set({ draggingType: type })
  },

  showToast(msg) {
    set({ toast: msg })
    setTimeout(() => set((s) => (s.toast === msg ? { toast: null } : {})), 2500)
  },
}))

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
