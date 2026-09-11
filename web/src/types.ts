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
}

export interface Entry {
  id: string
  label: string
  description?: string
  provider: string
  category: string
  icon?: string
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
}

export interface DocNode {
  id: string
  type: string
  name: string
  parent?: string
  props: Record<string, unknown>
  layout: Layout
}

export interface DocEdge {
  id: string
  kind: string
  source: string
  target: string
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
