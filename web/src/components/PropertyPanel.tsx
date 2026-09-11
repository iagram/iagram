import { useMemo } from 'react'
import { useStore } from '../store'
import type { JSONSchema, Problem } from '../types'

export function PropertyPanel() {
  const selectedId = useStore((s) => s.selectedId)
  const node = useStore((s) => s.nodes.find((n) => n.id === s.selectedId))
  const rules = useStore((s) => s.rules)
  const problems = useStore((s) => s.problems)
  const updateNode = useStore((s) => s.updateNode)
  const removeNodes = useStore((s) => s.removeNodes)
  const docName = useStore((s) => s.docName)
  const nodeCount = useStore((s) => s.nodes.length)
  const edgeCount = useStore((s) => s.edges.length)
  const nodePlan = useStore((s) => (s.selectedId ? s.plan?.summary.nodes[s.selectedId] : undefined))
  const planStale = useStore((s) => s.planStale)
  const nodeDrift = useStore((s) => (s.selectedId ? s.drift?.summary.nodes[s.selectedId] : undefined))

  // Derive from the stable nodes array: a selector returning a new array each
  // render would re-render forever.
  const allNodes = useStore((s) => s.nodes)
  const selectedEdgeId = useStore((s) => s.selectedEdgeId)
  const edge = useStore((s) => s.edges.find((e) => e.id === s.selectedEdgeId))
  const removeEdge = useStore((s) => s.removeEdge)
  const catalog = useStore((s) => s.catalog)
  const selIds = useMemo(() => allNodes.filter((n) => n.selected).map((n) => n.id), [allNodes])
  const selectedCount = selIds.length
  if (selectedEdgeId && edge && rules) {
    const src = allNodes.find((n) => n.id === edge.source)
    const dst = allNodes.find((n) => n.id === edge.target)
    const rule = src && dst ? rules.connection(src.data.type, dst.data.type) : undefined
    const tf = catalog?.connections.find((r) => r.from === src?.data.type && r.to === dst?.data.type)?.terraform
    return (
      <aside className="panel">
        <h2>Connection</h2>
        <p>
          <b>{src?.data.name}</b> <span className="muted">→</span> <b>{dst?.data.name}</b>
        </p>
        <p className="muted">
          {rules.entry(src?.data.type ?? '')?.label} <i>{rule?.label ?? edge.data?.kind}</i> {rules.entry(dst?.data.type ?? '')?.label}
        </p>
        <p className="muted mono">kind: {edge.data?.kind}</p>
        {tf ? (
          <details open>
            <summary>What this arrow does in Terraform</summary>
            <p className="muted">
              Adds <span className="mono">{tf.value}</span> to the <span className="mono">{tf.input}</span> input of the {tf.set === 'from' ? 'source' : 'target'} module.
            </p>
          </details>
        ) : (
          <p className="muted">This arrow is documentation only; it does not change the generated Terraform.</p>
        )}
        <button className="danger" onClick={() => removeEdge(edge.id)}>
          Delete connection
        </button>
      </aside>
    )
  }
  if (selectedCount > 1 && rules) {
    return (
      <aside className="panel">
        <h2>{selectedCount} elements selected</h2>
        <p className="muted">⌘C copy · ⌘V paste · ⌘D duplicate · ⌫ delete. Drag to move them together; drop into another container to re-parent.</p>
        <button className="danger" onClick={() => removeNodes(selIds)}>
          Delete {selectedCount} elements
        </button>
      </aside>
    )
  }
  if (!node || !rules || !selectedId) {
    return (
      <aside className="panel">
        <h2>{docName || 'Diagram'}</h2>
        <p className="muted">
          {nodeCount} elements, {edgeCount} connections.
        </p>
        <p className="muted">Select an element to edit its properties. Drag from the palette to add one; connect elements by dragging from the right handle to the left handle of another.</p>
        <p className="muted">Shift+drag selects several; ⌘-click adds to the selection. Drag an element into another container to move it there.</p>
      </aside>
    )
  }

  const entry = rules.entry(node.data.type)
  const schema: JSONSchema = entry?.props ?? {}
  const required = new Set(schema.required ?? [])
  const mine = problems.filter((p) => p.node === selectedId)
  const byField: Record<string, Problem[]> = {}
  for (const p of mine) if (p.field) (byField[p.field] ??= []).push(p)
  const general = mine.filter((p) => !p.field)

  const setProp = (k: string, v: unknown) => {
    const props = { ...node.data.props }
    if (v === '' || v === undefined) delete props[k]
    else props[k] = v
    updateNode(node.id, { props })
  }

  return (
    <aside className="panel">
      <h2>
        {entry?.icon && <img src={`/icons/${entry.icon}`} alt="" />}
        {entry?.label ?? node.data.type}
      </h2>
      <p className="muted mono">{node.data.type}</p>
      {entry?.description && <p className="muted">{entry.description}</p>}

      <label className="field">
        <span>
          Name <em>*</em>
        </span>
        <input value={node.data.name} onChange={(e) => updateNode(node.id, { name: e.target.value })} />
        <FieldProblems problems={byField.name} />
      </label>

      {groupFields(schema).map(({ title, fields, advanced }) => (
        <details key={title} open={!advanced} className="section">
          <summary>
            {title}
            {fields.some(([k]) => byField[k]?.some((p) => p.level === 'error')) && <span className="badge error">!</span>}
          </summary>
          {fields.map(([k, s]) => (
            <Field key={k} name={k} schema={s} required={required.has(k)} value={node.data.props[k]} onChange={(v) => setProp(k, v)} problems={byField[k]} />
          ))}
        </details>
      ))}

      {general.length > 0 && (
        <ul className="problems">
          {general.map((p, i) => (
            <li key={i} className={p.level}>
              {p.message}
            </li>
          ))}
        </ul>
      )}

      {nodePlan && (
        <details open className={`planlist ${planStale ? 'stale' : ''}`}>
          <summary>
            Plan: <b className={nodePlan.action}>{nodePlan.action}</b> · {nodePlan.resources.length} resource{nodePlan.resources.length === 1 ? '' : 's'}
            {planStale && ' (stale)'}
          </summary>
          <ul>
            {nodePlan.resources.map((r) => (
              <li key={r.address} className={r.action}>
                <span className="mono">{r.type}</span> <span className="muted mono">{r.address.replace(/^module\.[^.]+\./, '')}</span>
              </li>
            ))}
          </ul>
        </details>
      )}

      {nodeDrift && nodeDrift.action !== 'no-op' && (
        <details open className="driftlist">
          <summary>
            <b>Drift</b> · changed outside iagram
          </summary>
          <ul>
            {nodeDrift.resources
              .filter((r) => r.action !== 'no-op')
              .map((r) => (
                <li key={r.address}>
                  <span className="mono">{r.address.replace(/^module\.[^.]+\./, '')}</span>
                  {r.changed && r.changed.length > 0 && <div className="muted mono">{r.changed.join(', ')}</div>}
                </li>
              ))}
          </ul>
          <p className="muted">Run Plan to see how iagram would reconcile it, or update the diagram to match.</p>
        </details>
      )}

      {node.data.outputs && Object.keys(node.data.outputs).length > 0 ? (
        <details open>
          <summary>Outputs</summary>
          <dl className="outputs">
            {Object.entries(node.data.outputs).map(([k, v]) => (
              <div key={k}>
                <dt className="mono">{k}</dt>
                <dd className="mono" title="click to copy" onClick={() => void navigator.clipboard?.writeText(String(v ?? ''))}>
                  {v === null || v === '' ? <span className="muted">(empty)</span> : String(v)}
                </dd>
              </div>
            ))}
          </dl>
        </details>
      ) : (
        entry?.outputs &&
        entry.outputs.length > 0 && (
          <details>
            <summary>Outputs after apply</summary>
            <p className="muted mono">{entry.outputs.join(', ')}</p>
          </details>
        )
      )}

      <button className="danger" onClick={() => removeNodes([node.id])}>
        Delete {entry?.kind === 'container' ? '(and contents)' : ''}
      </button>
    </aside>
  )
}

/** Order fields into catalog-declared sections; advanced ones collapse under "Advanced". */
function groupFields(schema: JSONSchema): { title: string; fields: [string, JSONSchema][]; advanced: boolean }[] {
  const sections = new Map<string, [string, JSONSchema][]>()
  const advanced: [string, JSONSchema][] = []
  for (const entry of Object.entries(schema.properties ?? {})) {
    const [, s] = entry
    if (s.advanced) {
      advanced.push(entry)
      continue
    }
    const g = s.group ?? 'Settings'
    ;(sections.get(g) ?? sections.set(g, []).get(g)!).push(entry)
  }
  const out = [...sections.entries()].map(([title, fields]) => ({ title, fields, advanced: false }))
  if (advanced.length) out.push({ title: 'Advanced', fields: advanced, advanced: true })
  return out
}

function Field({ name, schema, required, value, onChange, problems }: { name: string; schema: JSONSchema; required: boolean; value: unknown; onChange: (v: unknown) => void; problems?: Problem[] }) {
  const title = schema.title ?? name
  let control: JSX.Element
  if (schema.type === 'boolean') {
    control = <input type="checkbox" checked={Boolean(value ?? schema.default ?? false)} onChange={(e) => onChange(e.target.checked)} />
  } else if (schema.enum) {
    control = (
      <select value={String(value ?? '')} onChange={(e) => onChange(e.target.value)}>
        <option value="">{required ? 'Select…' : '(default)'}</option>
        {schema.enum.map((o) => (
          <option key={String(o)} value={String(o)}>
            {String(o)}
          </option>
        ))}
      </select>
    )
  } else if (schema.type === 'integer' || schema.type === 'number') {
    control = (
      <input
        type="number"
        value={value === undefined ? '' : String(value)}
        min={schema.minimum}
        max={schema.maximum}
        step={schema.type === 'integer' ? 1 : 'any'}
        placeholder={schema.default !== undefined ? String(schema.default) : ''}
        onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
      />
    )
  } else {
    control = <input value={value === undefined ? '' : String(value)} placeholder={schema.default !== undefined ? String(schema.default) : ''} onChange={(e) => onChange(e.target.value)} />
  }
  return (
    <label className={`field ${schema.type === 'boolean' ? 'inline' : ''}`}>
      <span>
        {title} {required && <em>*</em>}
      </span>
      {control}
      {schema.description && <small>{schema.description}</small>}
      <FieldProblems problems={problems} />
    </label>
  )
}

function FieldProblems({ problems }: { problems?: Problem[] }) {
  if (!problems?.length) return null
  return (
    <ul className="problems">
      {problems.map((p, i) => (
        <li key={i} className={p.level}>
          {p.message}
        </li>
      ))}
    </ul>
  )
}
