// Client-side mirror of the catalog rules so the editor can refuse invalid
// drops and connections before they exist. The Go validator remains the
// authority; this only decides what the UI lets you do.
import type { Catalog, Entry, Rule } from './types'
import { ROOT } from './types'

export class Rules {
  readonly byId: Record<string, Entry>
  private readonly conn: Map<string, Rule>

  constructor(catalog: Catalog) {
    this.byId = Object.fromEntries(catalog.entries.map((e) => [e.id, e]))
    this.conn = new Map(catalog.connections.map((r) => [`${r.from}->${r.to}`, r]))
  }

  entry(type: string): Entry | undefined {
    return this.byId[type]
  }

  canContain(parentType: string, childType: string): boolean {
    const child = this.byId[childType]
    if (!child || !child.allowed_parents.includes(parentType)) return false
    if (parentType === ROOT) return true
    const parent = this.byId[parentType]
    if (!parent || parent.kind !== 'container') return false
    if (parent.allowed_children && parent.allowed_children.length > 0) {
      return parent.allowed_children.includes(childType)
    }
    return true
  }

  connection(fromType: string, toType: string): Rule | undefined {
    const r = this.conn.get(`${fromType}->${toType}`)
    if (r) return r
    // A generated element (<provider>.res.<type>) may reference anything of its provider.
    if (fromType.includes('.res.') && fromType.split('.')[0] === toType.split('.')[0]) {
      return { from: fromType, to: toType, kind: 'references', label: 'references' }
    }
    return undefined
  }

  /** Register a generated element fetched from the server. */
  register(e: Entry) {
    this.byId[e.id] = e
  }

  isGenerated(type: string): boolean {
    return type.includes('.res.')
  }

  /** Types that are allowed somewhere inside the given container type. */
  childrenOf(parentType: string): Entry[] {
    return Object.values(this.byId).filter((e) => this.canContain(parentType, e.id))
  }

  /** Icon path for an element, honouring icon_variants keyed by true boolean props. */
  iconFor(type: string, props: Record<string, unknown>): string | undefined {
    const e = this.byId[type]
    if (!e) return undefined
    for (const [prop, icon] of Object.entries(e.icon_variants ?? {})) {
      if (props[prop] === true) return icon
    }
    return e.icon
  }

  defaults(type: string): Record<string, unknown> {
    const out: Record<string, unknown> = {}
    const props = this.byId[type]?.props?.properties ?? {}
    for (const [k, schema] of Object.entries(props)) {
      if (schema.default !== undefined) out[k] = schema.default
    }
    return out
  }
}
