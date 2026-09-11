import { useStore } from '../store'

export function Toolbar() {
  const dirty = useStore((s) => s.dirty)
  const saving = useStore((s) => s.saving)
  const save = useStore((s) => s.save)
  const docName = useStore((s) => s.docName)
  const problems = useStore((s) => s.problems)
  const errors = problems.filter((p) => p.level === 'error').length
  const warnings = problems.length - errors

  return (
    <header className="toolbar">
      <span className="brand">iagram</span>
      <span className="doc">
        {docName || 'iagram.json'}
        {dirty && <i title="unsaved changes"> ●</i>}
      </span>
      <span className="spacer" />
      <span className={`status ${errors ? 'error' : warnings ? 'warning' : 'ok'}`}>
        {errors ? `${errors} error${errors > 1 ? 's' : ''}` : warnings ? `${warnings} warning${warnings > 1 ? 's' : ''}` : 'valid'}
      </span>
      <button className="primary" onClick={() => void save()} disabled={saving || !dirty} title="⌘S / Ctrl+S">
        {saving ? 'Saving…' : 'Save'}
      </button>
      <button disabled title="Phase 2: generate Terraform and run a plan">
        Plan
      </button>
    </header>
  )
}
