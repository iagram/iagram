import { useState } from 'react'
import { useStore } from '../store'

const NAME: Record<string, string> = { aws: 'AWS', gcp: 'Google Cloud', azure: 'Azure' }

/** Shown on a mirrored tab: this canvas is the live equivalence of the source tab. */
export function ProjectionBanner() {
  const projection = useStore((s) => s.projection)
  const primary = useStore((s) => s.primaryProvider())
  const materialize = useStore((s) => s.materialize)
  const [open, setOpen] = useState(false)
  if (!projection) return null
  const r = projection.report
  const issues = (r.dropped?.length ?? 0) + (r.edges?.length ?? 0) + (r.notes?.length ?? 0)
  return (
    <div className="projection">
      <span>
        <b>Mirror of {NAME[primary] ?? primary}</b> through the equivalence table ({r.converted} element{r.converted === 1 ? '' : 's'}).
        {issues > 0 && (
          <button className="link" onClick={() => setOpen((v) => !v)}>
            {issues} note{issues === 1 ? '' : 's'}
          </button>
        )}
        {' '}Edit here to make {NAME[projection.provider] ?? projection.provider} the source.
      </span>
      <button onClick={materialize}>Make source</button>
      {open && (
        <ul>
          {r.dropped?.map((d) => <li key={d}>dropped: {d}</li>)}
          {r.edges?.map((d) => <li key={d}>connection: {d}</li>)}
          {r.notes?.map((d) => <li key={d}>{d}</li>)}
        </ul>
      )}
    </div>
  )
}
