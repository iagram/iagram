import type { Catalog, Document, DriftResult, Job, PlanResult, Validation } from './types'
import type { ImportReport } from './types'

// Browser fetch failures surface as an opaque TypeError; say what it means here.
const rawFetch = globalThis.fetch.bind(globalThis)
const fetch: typeof globalThis.fetch = (input, init) =>
  rawFetch(input, init).catch(() => {
    throw new Error('iagram server not reachable. Is `iagram up` still running? Restart it and reload this page.')
  })

async function json<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let msg = res.statusText
    try {
      msg = ((await res.json()) as { error?: string }).error ?? msg
    } catch {
      /* not json */
    }
    throw new Error(msg)
  }
  return (await res.json()) as T
}

export const api = {
  catalog: () => fetch('/api/catalog').then((r) => json<Catalog>(r)),
  document: () => fetch('/api/document').then((r) => json<{ document: Document; validation: Validation }>(r)),
  save: (doc: Document) =>
    fetch('/api/document', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(doc),
    }).then((r) => json<{ document: Document; validation: Validation }>(r)),
  validate: (doc: Document) =>
    fetch('/api/validate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(doc),
    }).then((r) => json<Validation>(r)),
  startPlan: () => fetch('/api/plan', { method: 'POST' }).then((r) => json<Job>(r)),
  startDestroyPlan: () => fetch('/api/plan/destroy', { method: 'POST' }).then((r) => json<Job>(r)),
  latestPlan: () => fetch('/api/plan/latest').then((r) => json<{ plan: PlanResult | null; drift: DriftResult | null }>(r)),
  startApply: () => fetch('/api/apply', { method: 'POST' }).then((r) => json<Job>(r)),
  startDrift: () => fetch('/api/drift', { method: 'POST' }).then((r) => json<Job>(r)),
  clearDrift: () => fetch('/api/drift', { method: 'DELETE' }),
  job: (id: string) => fetch(`/api/jobs/${id}`).then((r) => json<Job>(r)),
  cancelJob: (id: string) => fetch(`/api/jobs/${id}/cancel`, { method: 'POST' }),
  generate: () => fetch('/api/generate').then((r) => json<{ path: string; config: unknown; warnings?: string[] }>(r)),
  importState: (raw: string, name: string) =>
    fetch(`/api/import?name=${encodeURIComponent(name)}`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: raw }).then((r) =>
      json<{ document: Document; report: ImportReport; validation: Validation }>(r),
    ),
  health: () => fetch('/api/health').then((r) => json<{ ok: boolean; version: string; document: string }>(r)),
}
