import { useMemo, useState } from 'react'
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
  const setEdgeBinding = useStore((s) => s.setEdgeBinding)
  const linkAttribute = useStore((s) => s.linkAttribute)
  useStore((s) => s.catalogVersion)
  const selIds = useMemo(() => allNodes.filter((n) => n.selected).map((n) => n.id), [allNodes])
  const selectedCount = selIds.length
  if (selectedEdgeId && edge && rules) {
    const src = allNodes.find((n) => n.id === edge.source)
    const dst = allNodes.find((n) => n.id === edge.target)
    const rule = src && dst ? rules.connection(src.data.type, dst.data.type) : undefined
    const fullRule = catalog?.connections.find((r) => r.from === src?.data.type && r.to === dst?.data.type)
    const tf = fullRule?.terraform
    const reqs = [
      ...Object.entries(fullRule?.requires_from ?? {}).map(([k, v]) => `${src?.data.name}.${k} = ${String(v)}`),
      ...Object.entries(fullRule?.requires_to ?? {}).map(([k, v]) => `${dst?.data.name}.${k} = ${String(v)}`),
    ]
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
        {edge.data?.kind === 'references' && src && dst && (
          <ReferenceBinding
            sourceType={src.data.type}
            targetType={dst.data.type}
            attr={edge.data.attr ?? ''}
            output={edge.data.output ?? 'id'}
            onChange={(attr, output) => setEdgeBinding(edge.id, attr, output)}
          />
        )}
        {edge.data?.kind !== 'references' && tf ? (
          <details open>
            <summary>What this arrow does in Terraform</summary>
            <p className="muted">
              Adds <span className="mono">{tf.value}</span> to the <span className="mono">{tf.input}</span> input of the {tf.set === 'from' ? 'source' : 'target'} module.
            </p>
          </details>
        ) : edge.data?.kind !== 'references' ? (
          <p className="muted">This arrow is documentation only; it does not change the generated Terraform.</p>
        ) : null}
        {reqs.length > 0 && (
          <p className="muted">
            Requires: <span className="mono">{reqs.join(', ')}</span>
          </p>
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

      {rules.isGenerated(node.data.type) && (
        <p className="muted">
          Generated from the provider schema for <span className="mono">{entry?.terraform?.resource}</span>. Use 🔗 on an attribute to reference another element, or draw an arrow.
        </p>
      )}
      {groupFields(schema).map(({ title, fields, advanced }) => (
        <details key={title} open={!advanced} className="section">
          <summary>
            {title}
            {fields.some(([k]) => byField[k]?.some((p) => p.level === 'error')) && <span className="badge error">!</span>}
          </summary>
          {fields.map(([k, s]) => (
            <Field
              key={k}
              name={k}
              schema={s}
              required={required.has(k)}
              value={node.data.props[k]}
              onChange={(v) => setProp(k, v)}
              problems={byField[k]}
              linkable={rules.isGenerated(node.data.type) && (s.type === 'string' || s.type === 'array') && !s['x-json']}
              onLink={(targetId, output) => linkAttribute(node.id, k, targetId, output)}
              nodeId={node.id}
            />
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

function Field({
  name,
  schema,
  required,
  value,
  onChange,
  problems,
  linkable,
  onLink,
  nodeId,
}: {
  name: string
  schema: JSONSchema
  required: boolean
  value: unknown
  onChange: (v: unknown) => void
  problems?: Problem[]
  linkable?: boolean
  onLink?: (targetId: string, output: string) => void
  nodeId?: string
}) {
  const title = schema.title ?? name
  const [linking, setLinking] = useState(false)
  const [jsonText, setJsonText] = useState<string | null>(null)
  const [jsonError, setJsonError] = useState<string | null>(null)
  let control: JSX.Element
  if (schema['x-json']) {
    const shown = jsonText ?? (value === undefined ? '' : JSON.stringify(value, null, 2))
    control = (
      <textarea
        className="mono"
        rows={Math.min(12, Math.max(3, shown.split('\n').length))}
        value={shown}
        placeholder={schema.type === 'array' ? '[ { … } ]' : '{ … }'}
        onChange={(e) => setJsonText(e.target.value)}
        onBlur={() => {
          if (jsonText === null) return
          if (jsonText.trim() === '') {
            onChange(undefined)
          } else {
            try {
              onChange(JSON.parse(jsonText))
              setJsonError(null)
            } catch (err) {
              setJsonError((err as Error).message)
              return
            }
          }
          setJsonText(null)
        }}
      />
    )
  } else if (schema.type === 'boolean') {
    control = <input type="checkbox" checked={Boolean(value ?? schema.default ?? false)} onChange={(e) => onChange(e.target.checked)} />
  } else if (schema.type === 'array') {
    const list = Array.isArray(value) ? (value as unknown[]).map(String) : []
    control = (
      <textarea
        rows={Math.min(6, Math.max(2, list.length + 1))}
        value={list.join('\n')}
        placeholder="one value per line"
        onChange={(e) => {
          const items = e.target.value.split('\n').map((x) => x.trim()).filter(Boolean)
          onChange(items.length ? (schema.items?.type === 'number' ? items.map(Number) : items) : undefined)
        }}
      />
    )
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
        {linkable && (
          <button type="button" className="link-btn" title="Reference another element with this attribute" onClick={(e) => (e.preventDefault(), setLinking((v) => !v))}>
            🔗
          </button>
        )}
      </span>
      {control}
      {linking && nodeId && onLink && (
        <LinkPicker
          sourceId={nodeId}
          onPick={(targetId, output) => {
            setLinking(false)
            onLink(targetId, output)
          }}
          onCancel={() => setLinking(false)}
        />
      )}
      {schema.description && <small>{schema.description}</small>}
      {jsonError && <small className="err">Invalid JSON: {jsonError}</small>}
      <FieldProblems problems={problems} />
    </label>
  )
}

/** Attribute/output picker for a references edge. */
function ReferenceBinding({ sourceType, targetType, attr, output, onChange }: { sourceType: string; targetType: string; attr: string; output: string; onChange: (attr: string, output: string) => void }) {
  const rules = useStore((s) => s.rules)
  const src = rules?.entry(sourceType)
  const dst = rules?.entry(targetType)
  const attrs = Object.entries(src?.props?.properties ?? {})
    .filter(([, s]) => (s.type === 'string' || s.type === 'array') && !s['x-json'])
    .map(([k]) => k)
    .sort((a, b) => score(b) - score(a) || a.localeCompare(b))
  const outputs = ['id', ...(dst?.outputs ?? []).filter((o) => o !== 'id')]
  return (
    <div className="binding">
      <label className="field">
        <span>Attribute of the source that receives the reference</span>
        <select value={attr} onChange={(e) => onChange(e.target.value, output)}>
          <option value="">Choose…</option>
          {attrs.map((a) => (
            <option key={a} value={a}>
              {a}
            </option>
          ))}
        </select>
      </label>
      <label className="field">
        <span>Attribute of the target being referenced</span>
        <select value={output} onChange={(e) => onChange(attr, e.target.value)}>
          {outputs.map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </select>
      </label>
      {attr && (
        <p className="muted mono">
          {attr} = {'${'}
          {dst?.terraform?.resource ? `${dst.terraform.resource}.<name>` : 'module.<id>'}.{output}
          {'}'}
        </p>
      )}
    </div>
  )
}

function score(a: string): number {
  if (/_ids?$/.test(a)) return 3
  if (/_arns?$/.test(a) || /_names?$/.test(a)) return 2
  if (/(vpc|subnet|network|group|role|key|bucket|topic|queue|zone)/.test(a)) return 1
  return 0
}

/** Target chooser for linking from the settings panel. */
function LinkPicker({ sourceId, onPick, onCancel }: { sourceId: string; onPick: (targetId: string, output: string) => void; onCancel: () => void }) {
  const nodes = useStore((s) => s.nodes)
  const rules = useStore((s) => s.rules)
  const src = nodes.find((n) => n.id === sourceId)
  const [target, setTarget] = useState('')
  const candidates = nodes.filter((n) => n.id !== sourceId && rules && src && rules.connection(src.data.type, n.data.type))
  const dst = nodes.find((n) => n.id === target)
  const outputs = ['id', ...((dst && rules?.entry(dst.data.type)?.outputs) ?? []).filter((o) => o !== 'id')]
  const [output, setOutput] = useState('id')
  return (
    <div className="binding">
      <select value={target} onChange={(e) => setTarget(e.target.value)}>
        <option value="">Link to element…</option>
        {candidates.map((n) => (
          <option key={n.id} value={n.id}>
            {n.data.name} ({rules?.entry(n.data.type)?.label ?? n.data.type})
          </option>
        ))}
      </select>
      {target && (
        <select value={output} onChange={(e) => setOutput(e.target.value)}>
          {outputs.map((o) => (
            <option key={o} value={o}>
              {o}
            </option>
          ))}
        </select>
      )}
      <div className="actions">
        <button type="button" onClick={onCancel}>
          Cancel
        </button>
        <button type="button" className="primary" disabled={!target} onClick={() => onPick(target, output)}>
          Link
        </button>
      </div>
    </div>
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
