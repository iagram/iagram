import { useMemo } from 'react'
import { useStore } from '../store'
import type { NodePlan, PlanAction } from '../types'

const GLYPH: Record<PlanAction, string> = { create: '+', update: '~', replace: '±', delete: '−', read: '·', 'no-op': '' }

/** Error/warning count, planned action, drift and deployed markers for a node. */
export function NodeBadge({ id }: { id: string }) {
  const problems = useStore((s) => s.problems)
  const plan = useStore((s) => s.plan)
  const nodes = useStore((s) => s.nodes)
  const rules = useStore((s) => s.rules)
  useStore((s) => s.catalogVersion)
  // Attachments are hidden children; count them and roll their plan into the parent.
  const { attachments, nodePlan } = useMemo(() => {
    let attachments = 0
    const kids: NodePlan[] = []
    for (const n of nodes) {
      if (n.parentId === id && rules?.entry(n.data.type)?.attachment) {
        attachments++
        const k = plan?.summary.nodes[n.id]
        if (k) kids.push(k)
      }
    }
    const own = plan?.summary.nodes[id]
    if (!plan || (!own && kids.length === 0)) return { attachments, nodePlan: undefined as NodePlan | undefined }
    if (kids.length === 0) return { attachments, nodePlan: own }
    const sev: Record<string, number> = { 'no-op': 0, read: 1, create: 2, update: 3, replace: 4, delete: 5 }
    let action: PlanAction = own?.action ?? 'no-op'
    const resources = [...(own?.resources ?? [])]
    for (const k of kids) {
      if (sev[k.action] > sev[action]) action = k.action
      resources.push(...k.resources)
    }
    return { attachments, nodePlan: { action, resources } as NodePlan }
  }, [nodes, rules, plan, id])
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
      {attachments > 0 && (
        <span className="badge attachments" title={`${attachments} attachment(s) configured in the settings panel`}>
          ⚙{attachments}
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
