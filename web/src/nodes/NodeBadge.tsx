import { useStore } from '../store'
import type { PlanAction } from '../types'

const GLYPH: Record<PlanAction, string> = { create: '+', update: '~', replace: '±', delete: '−', read: '·', 'no-op': '' }

/** Error/warning count and planned action for a node. */
export function NodeBadge({ id }: { id: string }) {
  const problems = useStore((s) => s.problems)
  const nodePlan = useStore((s) => s.plan?.summary.nodes[id])
  const stale = useStore((s) => s.planStale)
  const mine = problems.filter((p) => p.node === id)
  const errors = mine.filter((p) => p.level === 'error').length
  return (
    <>
      {nodePlan && nodePlan.action !== 'no-op' && (
        <span className={`badge plan ${nodePlan.action} ${stale ? 'stale' : ''}`} title={`${nodePlan.action}: ${nodePlan.resources.length} resource(s)`}>
          {GLYPH[nodePlan.action]}
          {nodePlan.resources.length}
        </span>
      )}
      {mine.length > 0 && (
        <span className={`badge ${errors ? 'error' : 'warning'}`} title={mine.map((p) => p.message).join('\n')}>
          {mine.length}
        </span>
      )}
    </>
  )
}

/** CSS class carrying the planned action, applied to the node root. */
export function usePlanClass(id: string): string {
  const action = useStore((s) => s.plan?.summary.nodes[id]?.action)
  const stale = useStore((s) => s.planStale)
  if (!action || action === 'no-op') return ''
  return `plan-${action}${stale ? ' plan-stale' : ''}`
}
