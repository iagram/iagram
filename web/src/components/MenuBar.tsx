import { useEffect, useRef, useState, type ReactNode } from 'react'
import { getNodesBounds, getViewportForBounds, useReactFlow } from '@xyflow/react'
import { toPng, toSvg } from 'html-to-image'
import { api } from '../api'
import { useStore } from '../store'
import type { Document } from '../types'

interface Item {
  label?: string
  shortcut?: string
  onClick?: () => void
  disabled?: boolean
  checked?: boolean
  href?: string
  sep?: boolean
  danger?: boolean
}

export function MenuBar() {
  const [open, setOpen] = useState<string | null>(null)
  const bar = useRef<HTMLDivElement>(null)
  const s = useStore()
  const { zoomIn, zoomOut, fitView, zoomTo, getNodes } = useReactFlow()
  const fileInput = useRef<HTMLInputElement>(null)
  const stateInput = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (!bar.current?.contains(e.target as Node)) setOpen(null)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(null)
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  const running = s.job?.status === 'running'
  const errors = s.problems.filter((p) => p.level === 'error').length
  const hasSelection = s.nodes.some((n) => n.selected) || s.edges.some((e) => e.selected) || !!s.selectedEdgeId
  const hasDeployed = s.nodes.some((n) => n.data.outputs && Object.keys(n.data.outputs).length > 0)
  const canApply = !!s.plan && !s.planStale && s.plan.changes && !running && !s.dirty

  const download = (name: string, data: string | Blob, type = 'application/json') => {
    const blob = data instanceof Blob ? data : new Blob([data], { type })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = name
    a.click()
    setTimeout(() => URL.revokeObjectURL(a.href), 1000)
  }

  const exportImage = async (kind: 'png' | 'svg') => {
    const el = document.querySelector('.react-flow__viewport') as HTMLElement | null
    if (!el) return
    const bounds = getNodesBounds(getNodes())
    const w = Math.min(4096, Math.ceil(bounds.width + 80))
    const h = Math.min(4096, Math.ceil(bounds.height + 80))
    const vp = getViewportForBounds(bounds, w, h, 0.2, 4, 0.05)
    const opts = {
      backgroundColor: s.theme === 'dark' ? '#0f141b' : '#f6f7f9',
      width: w,
      height: h,
      style: { width: `${w}px`, height: `${h}px`, transform: `translate(${vp.x}px, ${vp.y}px) scale(${vp.zoom})` },
    }
    const name = `${s.docName || 'iagram'}.${kind}`
    try {
      const url = kind === 'png' ? await toPng(el, opts) : await toSvg(el, opts)
      const a = document.createElement('a')
      a.href = url
      a.download = name
      a.click()
    } catch (e) {
      s.showToast(`Export failed: ${(e as Error).message}`)
    }
  }

  const openFile = (f: File) => {
    f.text().then((txt) => {
      try {
        const doc = JSON.parse(txt) as Document
        if (!Array.isArray(doc.nodes) || !Array.isArray(doc.edges)) throw new Error('not an iagram document')
        s.loadDocument(doc)
      } catch (e) {
        s.showToast(`Cannot open: ${(e as Error).message}`)
      }
    })
  }

  const importState = (f: File) => {
    f.text().then(async (txt) => {
      try {
        const r = await api.importState(txt, f.name.replace(/\.(tfstate|json)$/, ''))
        s.loadDocument(r.document)
        const notes = [`${r.report.imported} resource(s) imported`]
        if (r.report.skipped?.length) notes.push(`${r.report.skipped.length} type(s) without mapping`)
        s.showToast(notes.join(', ') + '; arrows are not recovered')
      } catch (e) {
        s.showToast(`Import failed: ${(e as Error).message}`)
      }
    })
  }

  const menus: Record<string, Item[]> = {
    File: [
      { label: 'New diagram', onClick: () => (s.nodes.length === 0 || confirm('Clear the canvas? The file is not touched until you save.')) && s.newDiagram() },
      { label: 'Open iagram.json…', onClick: () => fileInput.current?.click() },
      { label: 'Import from Terraform state…', onClick: () => stateInput.current?.click() },
      { sep: true },
      { label: 'Save', shortcut: '⌘S', onClick: () => void s.save(), disabled: !s.dirty || s.saving },
      { label: 'Download iagram.json', onClick: () => download(`${s.docName || 'iagram'}.json`, JSON.stringify(s.currentDocument(), null, 2) + '\n') },
      { sep: true },
      {
        label: 'Export Terraform (main.tf.json)',
        disabled: errors > 0 || s.dirty,
        onClick: () => api.generate().then((r) => download('main.tf.json', JSON.stringify(r.config, null, 2) + '\n')).catch((e) => s.showToast((e as Error).message)),
      },
      { label: 'Export as PNG', onClick: () => void exportImage('png') },
      { label: 'Export as SVG', onClick: () => void exportImage('svg') },
    ],
    Edit: [
      { label: 'Undo', shortcut: '⌘Z', onClick: s.undo, disabled: s.past.length === 0 },
      { label: 'Redo', shortcut: '⌘⇧Z', onClick: s.redo, disabled: s.future.length === 0 },
      { sep: true },
      { label: 'Copy', shortcut: '⌘C', onClick: s.copySelection, disabled: !s.nodes.some((n) => n.selected) },
      { label: 'Paste', shortcut: '⌘V', onClick: s.paste },
      { label: 'Duplicate', shortcut: '⌘D', onClick: s.duplicateSelection, disabled: !s.nodes.some((n) => n.selected) },
      { label: 'Delete', shortcut: '⌫', onClick: s.deleteSelection, disabled: !hasSelection, danger: true },
      { sep: true },
      { label: 'Select all', shortcut: '⌘A', onClick: s.selectAll, disabled: s.nodes.length === 0 },
      { label: 'Deselect', shortcut: 'Esc', onClick: s.deselectAll },
    ],
    View: [
      { label: 'Zoom in', shortcut: '⌘+', onClick: () => void zoomIn() },
      { label: 'Zoom out', shortcut: '⌘−', onClick: () => void zoomOut() },
      { label: 'Zoom to 100%', shortcut: '⌘0', onClick: () => void zoomTo(1) },
      { label: 'Fit diagram', shortcut: '⇧1', onClick: () => void fitView({ padding: 0.1 }) },
      { sep: true },
      { label: 'Connection labels', checked: s.showLabels, onClick: () => s.setShowLabels(!s.showLabels) },
      { label: 'Minimap', checked: s.showMinimap, onClick: () => s.setShowMinimap(!s.showMinimap) },
      { label: 'Snap to grid', checked: s.snapToGrid, onClick: () => s.setSnapToGrid(!s.snapToGrid) },
      { sep: true },
      { label: 'Dark theme', checked: s.theme === 'dark', onClick: () => s.setTheme(s.theme === 'dark' ? 'light' : 'dark') },
    ],
    Infrastructure: [
      { label: 'Plan', onClick: () => void s.runPlan(), disabled: running || errors > 0 },
      { label: s.plan?.destroy ? 'Destroy (apply plan)' : 'Apply', onClick: () => s.setConfirmApply(true), disabled: !canApply },
      { label: 'Check drift', onClick: () => void s.runDrift(), disabled: running || !hasDeployed },
      { sep: true },
      { label: 'Show log', onClick: () => s.setLogOpen(true), disabled: s.jobLines.length === 0 },
      { label: 'Clear plan overlay', onClick: s.clearPlan, disabled: !s.plan },
      { label: 'Clear drift overlay', onClick: () => void s.clearDrift(), disabled: !s.drift },
      { sep: true },
      { label: 'Destroy infrastructure…', onClick: () => void s.runDestroyPlan(), disabled: running || !hasDeployed, danger: true },
    ],
    Help: [
      { label: 'Keyboard shortcuts', shortcut: '?', onClick: () => s.setModal('shortcuts') },
      { label: 'Documentation', href: 'https://github.com/iagram/iagram#readme' },
      { label: 'Catalog specification', href: 'https://github.com/iagram/iagram/blob/main/docs/spec/catalog.md' },
      { label: 'Report an issue', href: 'https://github.com/iagram/iagram/issues/new' },
      { sep: true },
      { label: `About iagram${s.version ? ` ${s.version}` : ''}`, onClick: () => s.setModal('about') },
    ],
  }

  const render = (items: Item[]): ReactNode =>
    items.map((it, i) =>
      it.sep ? (
        <hr key={i} />
      ) : it.href ? (
        <a key={i} role="menuitem" href={it.href} target="_blank" rel="noreferrer" onClick={() => setOpen(null)}>
          {it.label}
        </a>
      ) : (
        <button
          key={i}
          role="menuitem"
          disabled={it.disabled}
          className={`${it.danger ? 'danger-item' : ''} ${it.checked !== undefined ? 'checkable' : ''} ${it.checked ? 'checked' : ''}`}
          onClick={() => {
            setOpen(null)
            it.onClick?.()
          }}
        >
          <span className="check">{it.checked ? '✓' : ''}</span>
          <span className="label">{it.label}</span>
          {it.shortcut && <kbd>{it.shortcut}</kbd>}
        </button>
      ),
    )

  return (
    <div className="menubar" ref={bar} role="menubar">
      {Object.entries(menus).map(([name, items]) => (
        <span key={name} className={`menu-wrap ${open === name ? 'open' : ''}`}>
          <button role="menuitem" aria-haspopup="menu" aria-expanded={open === name} onClick={() => setOpen(open === name ? null : name)} onMouseEnter={() => open && setOpen(name)}>
            {name}
          </button>
          {open === name && (
            <div className="menu" role="menu">
              {render(items)}
            </div>
          )}
        </span>
      ))}
      <input ref={fileInput} type="file" accept=".json,application/json" hidden onChange={(e) => e.target.files?.[0] && (openFile(e.target.files[0]), (e.target.value = ''))} />
      <input ref={stateInput} type="file" accept=".tfstate,.json,application/json" hidden onChange={(e) => e.target.files?.[0] && (importState(e.target.files[0]), (e.target.value = ''))} />
    </div>
  )
}
