import { useStore } from '../store'

export function ProblemsBar() {
  const problems = useStore((s) => s.problems)
  const nodes = useStore((s) => s.nodes)
  const select = useStore((s) => s.select)
  if (problems.length === 0) return null
  const nameOf = (id?: string) => (id ? nodes.find((n) => n.id === id)?.data.name ?? id : '')
  return (
    <footer className="problemsbar">
      {problems.slice(0, 50).map((p, i) => (
        <button key={i} className={p.level} onClick={() => p.node && select(p.node)}>
          <b>{p.level}</b> {p.node && <span className="mono">{nameOf(p.node)}</span>} {p.message}
        </button>
      ))}
    </footer>
  )
}
