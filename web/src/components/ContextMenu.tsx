import { useEffect } from 'react'
import { useStore } from '../store'

/** Right-click menu for nodes, edges and the empty canvas. */
export function ContextMenu() {
  const menu = useStore((s) => s.contextMenu)
  const close = useStore((s) => s.setContextMenu)
  const s = useStore()
  useEffect(() => {
    if (!menu) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && close(null)
    const onDown = (e: MouseEvent) => {
      if (!(e.target as HTMLElement).closest('.ctx-menu')) close(null)
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener('mousedown', onDown)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('mousedown', onDown)
    }
  }, [menu, close])
  if (!menu) return null
  const run = (fn: () => void) => () => {
    close(null)
    fn()
  }
  const selected = s.nodes.some((n) => n.selected)
  const target = menu.target === 'node' && menu.id ? s.nodes.find((n) => n.id === menu.id) : undefined
  const insideCount = target ? s.nodes.filter((n) => n.parentId === target.id).length : 0
  type Item = { label?: string; kbd?: string; onClick?: () => void; disabled?: boolean; danger?: boolean; sep?: boolean }
  const items: Item[] =
    menu.target === 'pane'
      ? [
          { label: 'Paste here', kbd: '⌘V', onClick: run(() => s.paste(menu.flow)), disabled: !s.hasClipboard() },
          { sep: true },
          { label: 'Select all', kbd: '⌘A', onClick: run(s.selectAll), disabled: s.nodes.length === 0 },
          { label: 'Fit diagram', onClick: run(() => document.querySelector<HTMLButtonElement>('.canvas-controls button[title^="Fit"]')?.click()) },
        ]
      : menu.target === 'edge'
        ? [{ label: 'Delete connection', kbd: '⌫', onClick: run(() => menu.id && s.removeEdge(menu.id)), danger: true }]
        : [
            { label: 'Cut', kbd: '⌘X', onClick: run(s.cutSelection), disabled: !selected },
            { label: 'Copy', kbd: '⌘C', onClick: run(s.copySelection), disabled: !selected },
            { label: 'Paste', kbd: '⌘V', onClick: run(() => s.paste()), disabled: !s.hasClipboard() },
            { label: 'Duplicate', kbd: '⌘D', onClick: run(s.duplicateSelection), disabled: !selected },
            ...(target?.type === 'container' && !s.rules?.entry(target.data.type)?.span
              ? [
                  { sep: true },
                  target.data.view === 'icon' || (!target.data.view && s.rules?.entry(target.data.type)?.composite && !s.nodes.some((n) => n.parentId === target.id))
                    ? { label: 'Expand to box', onClick: run(() => s.updateNode(target.id, { view: 'box' })) }
                    : { label: `Collapse to icon${insideCount ? ` (hides ${insideCount})` : ''}`, onClick: run(() => s.updateNode(target.id, { view: 'icon' })) },
                ]
              : []),
            { sep: true },
            { label: 'Bring to front', kbd: '⌘⇧]', onClick: run(() => s.reorder('front')) },
            { label: 'Bring forward', kbd: '⌘]', onClick: run(() => s.reorder('forward')) },
            { label: 'Send backward', kbd: '⌘[', onClick: run(() => s.reorder('backward')) },
            { label: 'Send to back', kbd: '⌘⇧[', onClick: run(() => s.reorder('back')) },
            { sep: true },
            { label: 'Delete', kbd: '⌫', onClick: run(s.deleteSelection), danger: true },
          ]
  return (
    <div className="ctx-menu" style={{ left: menu.x, top: menu.y }} role="menu">
      {items.map((it, i) =>
        it.sep ? (
          <hr key={i} />
        ) : (
          <button key={i} role="menuitem" disabled={it.disabled} className={it.danger ? 'danger-item' : ''} onClick={it.onClick}>
            <span>{it.label}</span>
            {it.kbd && <kbd>{it.kbd}</kbd>}
          </button>
        ),
      )}
    </div>
  )
}
