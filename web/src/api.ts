import type { Catalog, Document, Job, PlanResult, Validation } from './types'

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
  latestPlan: () => fetch('/api/plan/latest').then((r) => json<{ plan: PlanResult | null }>(r)),
  job: (id: string) => fetch(`/api/jobs/${id}`).then((r) => json<Job>(r)),
  cancelJob: (id: string) => fetch(`/api/jobs/${id}/cancel`, { method: 'POST' }),
  generate: () => fetch('/api/generate').then((r) => json<{ path: string; config: unknown; warnings?: string[] }>(r)),
}
