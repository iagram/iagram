import { useReactFlow, useViewport } from '@xyflow/react'
import { useStore } from '../store'

export function Toolbar() {
  const dirty = useStore((s) => s.dirty)
  const saving = useStore((s) => s.saving)
  const save = useStore((s) => s.save)
  const docName = useStore((s) => s.docName)
  const problems = useStore((s) => s.problems)
  const canUndo = useStore((s) => s.past.length > 0)
  const canRedo = useStore((s) => s.future.length > 0)
  const undo = useStore((s) => s.undo)
  const redo = useStore((s) => s.redo)
  const plan = useStore((s) => s.plan)
  const planStale = useStore((s) => s.planStale)
  const job = useStore((s) => s.job)
  const runPlan = useStore((s) => s.runPlan)
  const clearPlan = useStore((s) => s.clearPlan)
  const setLogOpen = useStore((s) => s.setLogOpen)
  const running = job?.status === 'running'
  const { zoomIn, zoomOut, fitView, zoomTo } = useReactFlow()
  const { zoom } = useViewport()
  const errors = problems.filter((p) => p.level === 'error').length
  const warnings = problems.length - errors

  return (
    <header className="toolbar">
      <span className="brand">iagram</span>
      <span className="doc">
        {docName || 'iagram.json'}
        {dirty && <i title="unsaved changes"> ●</i>}
      </span>

      <span className="group">
        <button onClick={undo} disabled={!canUndo} title="Undo (⌘Z)">
          ↶
        </button>
        <button onClick={redo} disabled={!canRedo} title="Redo (⌘⇧Z)">
          ↷
        </button>
      </span>

      <span className="group">
        <button onClick={() => zoomOut()} title="Zoom out (⌘-)">
          −
        </button>
        <button className="zoom" onClick={() => zoomTo(1)} title="Reset to 100%">
          {Math.round(zoom * 100)}%
        </button>
        <button onClick={() => zoomIn()} title="Zoom in (⌘+)">
          +
        </button>
        <button onClick={() => fitView({ padding: 0.1 })} title="Fit diagram (⌘0)">
          ⛶
        </button>
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
      <button className="primary" onClick={() => void runPlan()} disabled={running || errors > 0} title={errors ? 'Fix validation errors first' : 'Save, generate Terraform and run tofu plan'}>
        {running ? 'Planning…' : 'Plan'}
      </button>
    </header>
  )
}
