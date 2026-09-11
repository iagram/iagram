import { useStore } from '../store'

export function ConfirmApply() {
  const open = useStore((s) => s.confirmApply)
  const plan = useStore((s) => s.plan)
  const nodes = useStore((s) => s.nodes)
  const runApply = useStore((s) => s.runApply)
  const setConfirmApply = useStore((s) => s.setConfirmApply)
  if (!open || !plan) return null
  const { add, change, destroy, nodes: perNode, orphans } = plan.summary
  const nameOf = (id: string) => nodes.find((n) => n.id === id)?.data.name ?? id
  const affected = Object.entries(perNode).filter(([, p]) => p.action !== 'no-op')
  return (
    <div className="modal-backdrop" onClick={() => setConfirmApply(false)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Apply this plan?</h2>
        <p>
          OpenTofu will <b className="create">create {add}</b>, <b className="update">change {change}</b> and <b className="delete">destroy {destroy}</b> resource{add + change + destroy === 1 ? '' : 's'} in your cloud account using
          your local credentials.
        </p>
        <ul className="affected">
          {affected.map(([id, p]) => (
            <li key={id} className={p.action}>
              <b>{p.action}</b> {nameOf(id)} <span className="muted">({p.resources.length})</span>
            </li>
          ))}
          {orphans?.map((o) => (
            <li key={o.address} className="delete">
              <b>destroy</b> <span className="mono">{o.address}</span> <span className="muted">(no longer in the diagram)</span>
            </li>
          ))}
        </ul>
        {destroy > 0 && <p className="warn">This plan destroys resources. Destroyed data is not recoverable.</p>}
        <div className="actions">
          <button onClick={() => setConfirmApply(false)}>Cancel</button>
          <button className={destroy > 0 ? 'danger' : 'primary'} onClick={() => void runApply()}>
            {destroy > 0 ? `Apply and destroy ${destroy}` : 'Apply'}
          </button>
        </div>
      </div>
    </div>
  )
}
