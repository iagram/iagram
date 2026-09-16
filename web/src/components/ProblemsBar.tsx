import { useReactFlow, useStoreApi } from '@xyflow/react'
import { useStore } from '../store'

export function ProblemsBar() {
  const problems = useStore((s) => s.problems)
  const nodes = useStore((s) => s.nodes)
  const edges = useStore((s) => s.edges)
  const requestSelect = useStore((s) => s.requestSelect)
  const select = useStore((s) => s.select)
  const selectEdge = useStore((s) => s.selectEdge)
  const setShowGallery = useStore((s) => s.setShowGallery)
  const activeProvider = useStore((s) => s.activeProvider)
  const setActiveProvider = useStore((s) => s.setActiveProvider)
  const { fitView } = useReactFlow()
  const rf = useStoreApi()
  if (problems.length === 0) return null
  const nameOf = (id?: string) => (id ? nodes.find((n) => n.id === id)?.data.name ?? id : '')
  const focus = (p: { node?: string; edge?: string }) => {
    setShowGallery(false)
    const goto = (id: string) => {
      const n = nodes.find((x) => x.id === id)
      const prov = n?.data.type.split('.')[0]
      if (prov && prov !== 'common' && prov !== activeProvider) setActiveProvider(prov)
    }
    if (p.node) {
      goto(p.node)
      select(p.node) // inspector
      requestSelect(p.node) // React Flow selection + centre, once the tab shows it
      return
    }
    if (p.edge) {
      const e = edges.find((x) => x.id === p.edge)
      if (!e) return
      goto(e.source)
      selectEdge(e.id)
      requestAnimationFrame(() => {
        rf.getState().addSelectedEdges([e.id])
        void fitView({ nodes: [{ id: e.source }, { id: e.target }], duration: 350, padding: 0.5, maxZoom: 1.25 })
      })
    }
  }
  return (
    <footer className="problemsbar">
      {problems.slice(0, 50).map((p, i) => (
        <button key={i} className={p.level} onClick={() => focus(p)} title="Select and centre the element">
          <b>{p.level}</b> {p.node && <span className="mono">{nameOf(p.node)}</span>} {p.message}
        </button>
      ))}
    </footer>
  )
}
