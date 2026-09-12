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

export interface Entry {
  terraform?: { role?: string; resource?: string; module?: string }
  attachment?: boolean
  component?: boolean
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
  terraform?: { set: 'from' | 'to'; input: string; value: string }
  requires_from?: Record<string, unknown>
  requires_to?: Record<string, unknown>
}

export interface Catalog {
  entries: Entry[]
  connections: Rule[]
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
}

export interface DocEdge {
  id: string
  kind: string
  source: string
  target: string
  attr?: string
  output?: string
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
