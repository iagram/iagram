// Mirrors internal/catalog and internal/document on the Go side.

export type Kind = 'leaf' | 'container'

export interface JSONSchema {
  type?: string
  title?: string
  description?: string
  default?: unknown
  enum?: unknown[]
  format?: string
  minimum?: number
  maximum?: number
  pattern?: string
  required?: string[]
  properties?: Record<string, JSONSchema>
  /** iagram annotations: panel section and collapsed-by-default flag */
  group?: string
  advanced?: boolean
  /** value edited as raw JSON (complex Terraform types, nested blocks) */
  'x-json'?: boolean
  items?: JSONSchema
}

export interface BoxStyle {
  border?: string
  fill?: string
  dash?: 'solid' | 'dashed' | 'dotted'
  label?: 'left' | 'center'
}

export interface Entry {
  style?: BoxStyle
  style_variants?: Record<string, BoxStyle>
  caption_prop?: string
  /** child property -> template ("${zone}", "${region}${zone}") given to elements drawn inside */
  provides?: Record<string, string>
  terraform?: { role?: string; resource?: string; module?: string }
  attachment?: boolean
  component?: boolean
  link?: boolean
  /** spanning group: a band across containers; covered containers fill span.attr */
  span?: Span
  transparent?: boolean
  id: string
  label: string
  description?: string
  provider: string
  category: string
  icon?: string
  icon_variants?: Record<string, string>
  kind: Kind
  allowed_parents: string[]
  allowed_children?: string[]
  props?: JSONSchema
  outputs?: string[]
  size?: { w: number; h: number }
}

export interface Rule {
  from: string
  to: string
  kind: string
  label?: string
  style?: Omit<EdgeStyle, 'inactive'>
  /** link edges: the link element this rule creates */
  type?: string
  terraform?: { set: 'from' | 'to'; input: string; value: string }
  requires_from?: Record<string, unknown>
  requires_to?: Record<string, unknown>
}

export interface LinkEnd {
  attr: string
  output?: string
  types?: string[]
  /** catalog element ids accepted at this end */
  elements: string[]
}

/** A Terraform resource drawn as a line between two elements (peering, attachment, VPN). */
export interface Link {
  id: string
  resource: string
  provider: string
  label: string
  from: LinkEnd
  to: LinkEnd
  style?: Omit<EdgeStyle, 'inactive'>
  defaults?: Record<string, unknown>
}

export interface Span {
  id: string
  resource: string
  provider: string
  label: string
  attr: string
  over: string[]
  elements: string[]
  outputs: Record<string, string>
  ghost?: { count: string; icon: string }
  icon?: string
}

export interface Catalog {
  entries: Entry[]
  connections: Rule[]
  links?: Link[]
}

export interface Layout {
  x: number
  y: number
  w?: number
  h?: number
  z?: number
}

export interface DocNode {
  id: string
  type: string
  name: string
  parent?: string
  props: Record<string, unknown>
  layout: Layout
  outputs?: Record<string, unknown>
  /** free second line under the name; informational */
  caption?: string
  /** callout number of the walkthrough ("1", "9a"); informational */
  step?: string
}

export interface EdgeStyle {
  direction?: 'one' | 'both' | 'none'
  dash?: 'solid' | 'dashed' | 'dotted'
  color?: string
  width?: number
  curve?: 'step' | 'straight' | 'bezier'
  inactive?: boolean
}

export interface DocEdge {
  id: string
  kind: string
  source: string
  target: string
  attr?: string
  output?: string
  /** overrides the kind's label; informational */
  label?: string
  step?: string
  style?: EdgeStyle
  /** link edges: link element, resource name and attributes besides the ends */
  type?: string
  name?: string
  props?: Record<string, unknown>
  /** anchors: t | r | b | l (default r -> l) */
  from_port?: string
  to_port?: string
}

export interface Step {
  n: string
  text: string
}

export interface GeneratedSummary {
  id: string
  label: string
  resource: string
  service: string
  category: string
  icon?: string
  graphical: boolean
  attachment: boolean
}

export interface Family {
  key: string
  label: string
  icon: string
  category: string
  types: string[]
  default: string
  curated?: string
}

export interface AttachmentOption {
  id: string
  label: string
  resource: string
  attr: string
  output: string
  component?: boolean
}

export interface ConvertReport {
  converted: number
  dropped?: string[]
  edges?: string[]
  notes?: string[]
}

export interface Document {
  version: number
  name?: string
  nodes: DocNode[]
  edges: DocEdge[]
  steps?: Step[]
}

export interface Problem {
  level: 'error' | 'warning'
  node?: string
  edge?: string
  field?: string
  message: string
}

export interface Validation {
  problems: Problem[]
}

export const ROOT = 'root'
/** Provider of the cloud-neutral vocabulary (groups, actors, notes). */
export const COMMON = 'common'
export const ANY_PARENT = '*'
/** Informational edge to or from an actor or note. */
export const FLOW_KIND = 'flow'
/** A Terraform link resource drawn as a line. */
export const LINK_KIND = 'link'

export type PlanAction = 'no-op' | 'read' | 'create' | 'update' | 'replace' | 'delete'

export interface ResourceChange {
  address: string
  type: string
  actions: string[]
  action: PlanAction
  changed?: string[]
}

export interface NodePlan {
  action: PlanAction
  resources: ResourceChange[]
}

export interface PlanSummary {
  add: number
  change: number
  destroy: number
  nodes: Record<string, NodePlan>
  orphans?: ResourceChange[]
}

export interface PlanResult {
  destroy?: boolean
  changes: boolean
  summary: PlanSummary
  config_path: string
  tofu_version: string
  duration_s: number
  warnings?: string[]
}

export interface ApplyResult {
  outputs: Record<string, Record<string, unknown>>
  nodes_updated: number
  duration_s: number
  tofu_version: string
}

export interface DriftResult {
  drift: boolean
  summary: PlanSummary
  checked_at: string
  duration_s: number
  tofu_version: string
}

export interface Job {
  id: string
  kind: 'plan' | 'apply' | 'drift'
  status: 'running' | 'succeeded' | 'failed' | 'cancelled'
  error?: string
  result?: PlanResult | ApplyResult | DriftResult
}

export interface ImportReport {
  imported: number
  skipped?: string[]
  unplaced?: string[]
  providers?: string[]
}

/** A shipped reference architecture (GET /api/templates). */
export interface TemplateItem {
  id: string
  provider: string
  slug: string
  title: string
  category: string
  tags: string[]
  source: string
  /** original diagram on the source page; loaded from the vendor site */
  image?: string
  description: string
  elements: number
  document: Document
  types: Record<string, { icon?: string; container?: boolean; border?: string; dash?: string }>
}
