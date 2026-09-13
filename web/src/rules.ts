// Client-side mirror of the catalog rules so the editor can refuse invalid
// drops and connections before they exist. The Go validator remains the
// authority; this only decides what the UI lets you do.
import type { Catalog, Entry, Link, Rule } from './types'
import { ANY_PARENT, COMMON, FLOW_KIND, LINK_KIND, ROOT } from './types'

export class Rules {
  readonly byId: Record<string, Entry>
  private readonly conn: Map<string, Rule>
  readonly links: Link[]

  constructor(catalog: Catalog) {
    this.byId = Object.fromEntries(catalog.entries.map((e) => [e.id, e]))
    this.conn = new Map(catalog.connections.map((r) => [`${r.from}->${r.to}`, r]))
    this.links = catalog.links ?? []
  }

  /** Link resources that can join the two element types, in either orientation. */
  linkCandidates(fromType: string, toType: string): Link[] {
    const p = fromType.split('.')[0]
    if (p !== toType.split('.')[0]) return []
    return this.links.filter((l) => l.provider === p && ((l.from.elements.includes(fromType) && l.to.elements.includes(toType)) || (l.from.elements.includes(toType) && l.to.elements.includes(fromType))))
  }

  linkRule(fromType: string, toType: string, preferred?: string): Rule | undefined {
    const cands = this.linkCandidates(fromType, toType)
    const l = cands.find((c) => c.id === preferred) ?? cands[0]
    return l ? { from: fromType, to: toType, kind: LINK_KIND, type: l.id, label: l.label, style: l.style } : undefined
  }

  entry(type: string): Entry | undefined {
    return this.byId[type]
  }

  canContain(parentType: string, childType: string): boolean {
    const child = this.byId[childType]
    if (!child) return false
    // Attachments live inside the element they configure, container or not.
    if (child.attachment && parentType !== ROOT) {
      const parent = this.byId[parentType]
      return !!parent && parent.provider === child.provider
    }
    if (parentType === ROOT) return child.allowed_parents.includes(ROOT) || child.allowed_parents.includes(ANY_PARENT)
    const parent = this.byId[parentType]
    if (!parent || parent.kind !== 'container') return false
    // Transparent containers (groups) accept anything; the real check is
    // against the group's own parent (see logicalParentType in the store).
    if (parent.transparent) return true
    if (!child.allowed_parents.includes(parentType) && !child.allowed_parents.includes(ANY_PARENT)) return false
    if (parent.allowed_children && parent.allowed_children.length > 0) {
      return parent.allowed_children.includes(childType)
    }
    return true
  }

  connection(fromType: string, toType: string): Rule | undefined {
    const r = this.conn.get(`${fromType}->${toType}`)
    if (r) return r
    // Actors and notes connect to anything with an informational arrow.
    if (fromType.split('.')[0] === COMMON || toType.split('.')[0] === COMMON) {
      return { from: fromType, to: toType, kind: FLOW_KIND, label: '' }
    }
    // Link resources (peering, attachments, VPN) are drawn as lines.
    const link = this.linkRule(fromType, toType)
    if (link) return link
    // A generated element (<provider>.res.<type>) may reference anything of its provider.
    if (fromType.includes('.res.') && fromType.split('.')[0] === toType.split('.')[0]) {
      return { from: fromType, to: toType, kind: 'references', label: 'references', style: { dash: 'dotted' } }
    }
    // Anything else within one provider is a documentation arrow: drawn, never generated.
    if (this.byId[fromType] && this.byId[toType] && fromType.split('.')[0] === toType.split('.')[0]) {
      return { from: fromType, to: toType, kind: FLOW_KIND, label: '' }
    }
    return undefined
  }

  /** Register a generated element fetched from the server. */
  register(e: Entry) {
    this.byId[e.id] = e
  }

  isTransparent(type: string): boolean {
    return !!this.byId[type]?.transparent
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
