import { useStore } from '../store'

/** Error/warning count for a node, from the last validation run. */
export function NodeBadge({ id }: { id: string }) {
  const problems = useStore((s) => s.problems)
  const mine = problems.filter((p) => p.node === id)
  if (mine.length === 0) return null
  const errors = mine.filter((p) => p.level === 'error').length
  return (
    <span className={`badge ${errors ? 'error' : 'warning'}`} title={mine.map((p) => p.message).join('\n')}>
      {mine.length}
    </span>
  )
}
