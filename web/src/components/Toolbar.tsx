import { useState } from 'react'
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
  const setConfirmApply = useStore((s) => s.setConfirmApply)
  const runDrift = useStore((s) => s.runDrift)
  const drift = useStore((s) => s.drift)
  const clearDrift = useStore((s) => s.clearDrift)
  const hasDeployed = useStore((s) => s.nodes.some((n) => n.data.outputs && Object.keys(n.data.outputs).length > 0))
  const running = job?.status === 'running'
  const canApply = !!plan && !planStale && plan.changes && !running && !dirty
  const theme = useStore((s) => s.theme)
  const setTheme = useStore((s) => s.setTheme)
  const showLabels = useStore((s) => s.showLabels)
  const setShowLabels = useStore((s) => s.setShowLabels)
  const runDestroyPlan = useStore((s) => s.runDestroyPlan)
  const [menu, setMenu] = useState(false)
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

      <span className="group">
        <button className={showLabels ? 'on' : ''} onClick={() => setShowLabels(!showLabels)} title={showLabels ? 'Hide connection labels (shown on hover)' : 'Show connection labels'}>
          Aa
        </button>
        <button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} title={theme === 'dark' ? 'Light theme' : 'Dark theme'}>
          {theme === 'dark' ? '☀' : '☾'}
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
      <span className="menu-wrap">
        <button onClick={() => setMenu((m) => !m)} title="More" aria-haspopup="menu" aria-expanded={menu}>
          ⋯
        </button>
        {menu && (
          <div className="menu" role="menu" onMouseLeave={() => setMenu(false)}>
            <button
              role="menuitem"
              className="danger-item"
              disabled={running || !hasDeployed}
              onClick={() => {
                setMenu(false)
                void runDestroyPlan()
              }}
              title={hasDeployed ? 'Plan the teardown of everything this diagram manages' : 'Nothing deployed'}
            >
              Destroy infrastructure…
            </button>
          </div>
        )}
      </span>
      <button className={`apply ${canApply ? 'ready' : ''} ${plan?.destroy ? 'destroy' : ''}`} onClick={() => setConfirmApply(true)} disabled={!canApply} title={!plan ? 'Plan first' : planStale || dirty ? 'Diagram changed; plan again' : !plan.changes ? 'Nothing to apply' : 'Apply this plan'}>
        {running && job?.kind === 'apply' ? (plan?.destroy ? 'Destroying…' : 'Applying…') : plan?.destroy ? 'Destroy' : 'Apply'}
      </button>
    </header>
  )
}
