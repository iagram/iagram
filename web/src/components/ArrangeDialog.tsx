import { useEffect, useState } from 'react'
import { useStore } from '../store'
import { DEFAULT_LAYOUT, type LayoutOptions } from '../types'

const TITLES: Record<string, string> = { flow: 'Flow', tree: 'Tree', radial: 'Radial tree', organic: 'Organic', orgchart: 'Org chart', circle: 'Circle', grid: 'Grid' }
const HELP: Record<string, string> = {
  flow: 'Layered: elements ordered along the arrows in ranks; Availability Zones as a row of columns with their subnets stacked. The reference-architecture look.',
  tree: 'Tidy tree: roots are the elements nothing points at; parents centred over their children.',
  radial: 'Root at the centre, each tree level on a ring; branches get an angle proportional to their leaves.',
  organic: 'Force-directed: elements repel, arrows pull like springs, until the drawing settles.',
  orgchart: 'Vertical tree with leaf children listed in a column under their parent.',
  circle: 'Siblings on one ring, in reading order.',
  grid: 'Rows of equal cells, containers first.',
}

/** draw.io-style layout dialog: algorithm options, then Apply. */
export function ArrangeDialog() {
  const opts = useStore((s) => s.arrangeDialog)
  const setOpts = useStore((s) => s.setArrangeDialog)
  const autoArrange = useStore((s) => s.autoArrange)
  const selectedContainer = useStore((s) => {
    const sel = s.nodes.filter((n) => n.selected)
    return sel.length === 1 && sel[0].type === 'container' ? sel[0].data.name : null
  })
  const [o, setO] = useState<LayoutOptions>(opts ?? DEFAULT_LAYOUT)
  useEffect(() => {
    if (opts) setO(opts)
  }, [opts])
  if (!opts) return null
  const directional = o.algo === 'flow' || o.algo === 'tree'
  const apply = () => {
    void autoArrange(o)
    setOpts(null)
  }
  return (
    <div className="modal-backdrop" onClick={() => setOpts(null)}>
      <div className="modal arrange" role="dialog" aria-label="Arrange" onClick={(e) => e.stopPropagation()} onKeyDown={(e) => e.key === 'Enter' && apply()}>
        <header>
          <h3>{TITLES[o.algo] ?? 'Arrange'}{directional ? (o.dir === 'TB' ? ', vertical' : ', horizontal') : ''}</h3>
          <button className="icon" onClick={() => setOpts(null)} title="Close">✕</button>
        </header>
        <p className="muted">{HELP[o.algo]}{selectedContainer ? ` Applies to the contents of "${selectedContainer}".` : ' Applies to the whole diagram.'}</p>
        <section>
          <label>
            <span>Algorithm</span>
            <select value={o.algo} onChange={(e) => setO({ ...o, algo: e.target.value as LayoutOptions['algo'] })}>
              {Object.entries(TITLES).map(([k, v]) => (
                <option key={k} value={k}>{v}</option>
              ))}
            </select>
          </label>
          {directional && (
            <label>
              <span>Direction</span>
              <select value={o.dir} onChange={(e) => setO({ ...o, dir: e.target.value as 'LR' | 'TB' })}>
                <option value="LR">Left to right</option>
                <option value="TB">Top to bottom</option>
              </select>
            </label>
          )}
          <label>
            <span>Node spacing</span>
            <input type="number" min={8} max={200} value={o.node_spacing} onChange={(e) => setO({ ...o, node_spacing: Number(e.target.value) })} />
          </label>
          <label>
            <span>Rank spacing</span>
            <input type="number" min={8} max={300} value={o.rank_spacing} onChange={(e) => setO({ ...o, rank_spacing: Number(e.target.value) })} />
          </label>
        </section>
        <section>
          <label className="check">
            <input type="checkbox" checked={o.resize_containers} onChange={(e) => setO({ ...o, resize_containers: e.target.checked })} />
            <span>Resize containers to fit</span>
          </label>
          <label className="check">
            <input type="checkbox" checked={o.preserve_origin} onChange={(e) => setO({ ...o, preserve_origin: e.target.checked })} />
            <span>Preserve origin</span>
          </label>
        </section>
        <footer>
          <button onClick={() => setO({ ...DEFAULT_LAYOUT, algo: o.algo, dir: o.dir })}>Reset</button>
          <button onClick={() => setOpts(null)}>Cancel</button>
          <button className="primary" onClick={apply} autoFocus>Apply</button>
        </footer>
      </div>
    </div>
  )
}
