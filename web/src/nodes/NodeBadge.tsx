import { useStore } from '../store'
import type { PlanAction } from '../types'

const GLYPH: Record<PlanAction, string> = { create: '+', update: '~', replace: '±', delete: '−', read: '·', 'no-op': '' }

/** Error/warning count, planned action, drift and deployed markers for a node. */
export function NodeBadge({ id }: { id: string }) {
  const problems = useStore((s) => s.problems)
  const nodePlan = useStore((s) => s.plan?.summary.nodes[id])
  const stale = useStore((s) => s.planStale)
  const drifted = useStore((s) => {
    const d = s.drift?.summary.nodes[id]
    return d && d.action !== 'no-op' ? d : undefined
  })
  const deployed = useStore((s) => {
    const n = s.nodes.find((n) => n.id === id)
    return !!n?.data.outputs && Object.keys(n.data.outputs).length > 0
  })
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
      {drifted && (
        <span className="badge drift" title={`drift: ${drifted.resources.filter((r) => r.action !== 'no-op').length} resource(s) changed outside iagram`}>
          drift
        </span>
      )}
      {deployed && !nodePlan && !drifted && <span className="deployed" title="Deployed: outputs available" />}
      {mine.length > 0 && (
        <span className={`badge ${errors ? 'error' : 'warning'}`} title={mine.map((p) => p.message).join('\n')}>
          {mine.length}
        </span>
      )}
    </>
  )
}

/** CSS classes carrying the planned action and drift state, applied to the node root. */
export function usePlanClass(id: string): string {
  const action = useStore((s) => s.plan?.summary.nodes[id]?.action)
  const stale = useStore((s) => s.planStale)
  const drift = useStore((s) => {
    const d = s.drift?.summary.nodes[id]?.action
    return d && d !== 'no-op'
  })
  const parts: string[] = []
  if (action && action !== 'no-op') parts.push(`plan-${action}`, stale ? 'plan-stale' : '')
  if (drift) parts.push('drifted')
  return parts.join(' ')
}
