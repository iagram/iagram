import { useReactFlow } from '@xyflow/react'
import { useStore } from '../store'

export function ProblemsBar() {
  const problems = useStore((s) => s.problems)
  const nodes = useStore((s) => s.nodes)
  const edges = useStore((s) => s.edges)
  const requestSelect = useStore((s) => s.requestSelect)
  const selectEdge = useStore((s) => s.selectEdge)
  const setShowGallery = useStore((s) => s.setShowGallery)
  const { fitView } = useReactFlow()
  if (problems.length === 0) return null
  const nameOf = (id?: string) => (id ? nodes.find((n) => n.id === id)?.data.name ?? id : '')
  const focus = (p: { node?: string; edge?: string }) => {
    setShowGallery(false)
    if (p.node) {
      requestSelect(p.node) // the canvas selects it and brings it into view
      return
    }
    if (p.edge) {
      const e = edges.find((x) => x.id === p.edge)
      if (!e) return
      selectEdge(e.id)
      void fitView({ nodes: [{ id: e.source }, { id: e.target }], duration: 350, padding: 0.5, maxZoom: 1.25 })
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
