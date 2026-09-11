import { useEffect, useRef } from 'react'
import { useStore } from '../store'

export function LogDrawer() {
  const open = useStore((s) => s.logOpen)
  const lines = useStore((s) => s.jobLines)
  const job = useStore((s) => s.job)
  const plan = useStore((s) => s.plan)
  const setLogOpen = useStore((s) => s.setLogOpen)
  const cancelPlan = useStore((s) => s.cancelPlan)
  const end = useRef<HTMLDivElement>(null)

  useEffect(() => {
    end.current?.scrollIntoView({ block: 'end' })
  }, [lines.length])

  if (!open) return null
  const running = job?.status === 'running'
  return (
    <section className="logdrawer">
      <header>
        <b>{job ? `${job.kind} · ${job.status}` : 'plan'}</b>
        {plan && !running && (
          <span className="muted">
            {plan.tofu_version} · {plan.duration_s.toFixed(1)}s · {plan.config_path}
          </span>
        )}
        <span className="spacer" />
        {running && <button onClick={() => void cancelPlan()}>Cancel</button>}
        <button onClick={() => setLogOpen(false)}>Close</button>
      </header>
      <pre>
        {lines.join('\n')}
        {job?.status === 'failed' && `\n\nerror: ${job.error}`}
        <div ref={end} />
      </pre>
      {plan?.summary.orphans && plan.summary.orphans.length > 0 && !running && (
        <footer>
          <b>Not in the diagram, still in state (will be destroyed):</b>
          <ul>
            {plan.summary.orphans.map((o) => (
              <li key={o.address} className="mono">
                {o.address}
              </li>
            ))}
          </ul>
        </footer>
      )}
    </section>
  )
}
