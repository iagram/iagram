import { useStore } from '../store'
import { MenuBar } from './MenuBar'

export function Toolbar() {
  const dirty = useStore((s) => s.dirty)
  const saving = useStore((s) => s.saving)
  const save = useStore((s) => s.save)
  const docName = useStore((s) => s.docName)
  const problems = useStore((s) => s.problems)
  const plan = useStore((s) => s.plan)
  const planStale = useStore((s) => s.planStale)
  const job = useStore((s) => s.job)
  const runPlan = useStore((s) => s.runPlan)
  const clearPlan = useStore((s) => s.clearPlan)
  const setLogOpen = useStore((s) => s.setLogOpen)
  const setConfirmApply = useStore((s) => s.setConfirmApply)
  const runDrift = useStore((s) => s.runDrift)
  const drift = useStore((s) => s.drift)
  const clearDrift = useStore((s) => s.clearDrift)
  const hasDeployed = useStore((s) => s.nodes.some((n) => n.data.outputs && Object.keys(n.data.outputs).length > 0))
  const running = job?.status === 'running'
  const canApply = !!plan && !planStale && plan.changes && !running && !dirty

  const errors = problems.filter((p) => p.level === 'error').length
  const warnings = problems.length - errors

  return (
    <header className="toolbar">
      <span className="brand">iagram</span>
      <MenuBar />
      <span className="doc">
        {docName || 'iagram.iad'}
        {dirty && <i title="unsaved changes"> ●</i>}
      </span>

      <span className="spacer" />
      <span className={`status ${errors ? 'error' : warnings ? 'warning' : 'ok'}`}>
        {errors ? `${errors} error${errors > 1 ? 's' : ''}` : warnings ? `${warnings} warning${warnings > 1 ? 's' : ''}` : 'valid'}
      </span>
      <button className="primary" onClick={() => void save()} disabled={saving || !dirty} title="Save (⌘S)">
        {saving ? 'Saving…' : 'Save'}
      </button>
      {plan && (
        <button className={`plan-summary ${planStale ? 'stale' : ''}`} onClick={() => setLogOpen(true)} title={planStale ? 'Diagram changed since this plan' : `OpenTofu plan (${plan.tofu_version})`}>
          <span className="create">+{plan.summary.add}</span> <span className="update">~{plan.summary.change}</span> <span className="delete">−{plan.summary.destroy}</span>
          {planStale && ' stale'}
        </button>
      )}
      {plan && !running && (
        <button onClick={clearPlan} title="Remove the plan overlay">
          Clear
        </button>
      )}
      {drift && (
        <button className={`drift-summary ${drift.drift ? 'bad' : 'ok'}`} onClick={() => (drift.drift ? setLogOpen(true) : void clearDrift())} title={`Checked ${new Date(drift.checked_at).toLocaleString()}`}>
          {drift.drift ? `drift: ${Object.values(drift.summary.nodes).filter((n) => n.action !== 'no-op').length} node(s)` : 'no drift ✓'}
        </button>
      )}
      <button onClick={() => void runDrift()} disabled={running || !hasDeployed} title={hasDeployed ? 'Refresh-only plan: find infrastructure changed outside iagram' : 'Apply something first'}>
        Check drift
      </button>
      <button className="primary" onClick={() => void runPlan()} disabled={running || errors > 0} title={errors ? 'Fix validation errors first' : 'Save, generate Terraform and run tofu plan'}>
        {running && job?.kind === 'plan' ? 'Planning…' : 'Plan'}
      </button>
      <button className={`apply ${canApply ? 'ready' : ''} ${plan?.destroy ? 'destroy' : ''}`} onClick={() => setConfirmApply(true)} disabled={!canApply} title={!plan ? 'Plan first' : planStale || dirty ? 'Diagram changed; plan again' : !plan.changes ? 'Nothing to apply' : 'Apply this plan'}>
        {running && job?.kind === 'apply' ? (plan?.destroy ? 'Destroying…' : 'Applying…') : plan?.destroy ? 'Destroy' : 'Apply'}
      </button>
    </header>
  )
}
