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

  if (!node || !rules || !selectedId) {
    return (
      <aside className="panel">
        <h2>{docName || 'Diagram'}</h2>
        <p className="muted">
          {nodeCount} elements, {edgeCount} connections.
        </p>
        <p className="muted">Select an element to edit its properties. Drag from the palette to add one; connect elements by dragging from the right handle to the left handle of another.</p>
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

      {Object.entries(schema.properties ?? {}).map(([k, s]) => (
        <Field key={k} name={k} schema={s} required={required.has(k)} value={node.data.props[k]} onChange={(v) => setProp(k, v)} problems={byField[k]} />
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

      {entry?.outputs && entry.outputs.length > 0 && (
        <details>
          <summary>Outputs after deploy</summary>
          <p className="muted mono">{entry.outputs.join(', ')}</p>
        </details>
      )}

      <button className="danger" onClick={() => removeNodes([node.id])}>
        Delete {entry?.kind === 'container' ? '(and contents)' : ''}
      </button>
    </aside>
  )
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
