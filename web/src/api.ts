import type { Catalog, Document, Validation } from './types'

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
}
