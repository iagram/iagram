import { useReactFlow } from '@xyflow/react'
import { useStore } from '../store'
import { DEFAULT_LAYOUT } from '../types'

/** Miro-style vertical rail: panels, pointer mode, quick inserts, history, arrange. */
export function ToolRail() {
  const showPalette = useStore((s) => s.showPalette)
  const setShowPalette = useStore((s) => s.setShowPalette)
  const showInspector = useStore((s) => s.showInspector)
  const setShowInspector = useStore((s) => s.setShowInspector)
  const handMode = useStore((s) => s.handMode)
  const setHandMode = useStore((s) => s.setHandMode)
  const undo = useStore((s) => s.undo)
  const redo = useStore((s) => s.redo)
  const canUndo = useStore((s) => s.past.length > 0)
  const canRedo = useStore((s) => s.future.length > 0)
  const addNode = useStore((s) => s.addNode)
  const setArrangeDialog = useStore((s) => s.setArrangeDialog)
  const nodeCount = useStore((s) => s.nodes.length)
  const { screenToFlowPosition } = useReactFlow()
  const insert = (type: string, w: number, h: number) => {
    const c = screenToFlowPosition({ x: window.innerWidth / 2, y: window.innerHeight / 2 })
    addNode(type, null, { x: c.x - w / 2, y: c.y - h / 2 })
  }
  return (
    <nav className="rail" aria-label="Tools">
      <div className="card">
        <button className={showPalette ? 'on' : ''} title="Elements palette" onClick={() => setShowPalette(!showPalette)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><rect x="2" y="2" width="6" height="6" rx="1.5" fill="currentColor"/><rect x="10" y="2" width="6" height="6" rx="1.5" fill="currentColor"/><rect x="2" y="10" width="6" height="6" rx="1.5" fill="currentColor"/><rect x="10" y="10" width="6" height="6" rx="1.5" fill="currentColor" opacity=".45"/></svg>
        </button>
      </div>
      <div className="card">
        <button className={!handMode ? 'on' : ''} title="Select (V): drag on the background to select" onClick={() => setHandMode(false)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><path d="M4 2 L14 9 L9.5 10 L12 15 L10.2 15.8 L7.8 11 L4 14 Z" fill="currentColor"/></svg>
        </button>
        <button className={handMode ? 'on' : ''} title="Hand (H): drag on the background to pan" onClick={() => setHandMode(true)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><path d="M6 16 L4 10 C3.6 8.8 5.2 8.2 5.8 9.3 L6.6 11 L6.6 4.2 C6.6 3 8.4 3 8.4 4.2 L8.4 8.5 L8.9 3.4 C9 2.2 10.8 2.3 10.7 3.5 L10.4 8.6 L11.2 4.6 C11.4 3.5 13 3.7 12.9 4.9 L12.4 9 L12.9 6.8 C13.2 5.7 14.8 6 14.6 7.2 L13.6 13 C13.2 15 11.8 16 10 16 Z" fill="currentColor"/></svg>
        </button>
        <span className="sep" />
        <button title="Add a note" onClick={() => insert('common.note', 200, 100)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><path d="M3 3 H15 V11 L11 15 H3 Z" fill="currentColor" opacity=".9"/><path d="M11 15 V11 H15" fill="none" stroke="#fff" strokeWidth="1.4"/></svg>
        </button>
        <button title="Add a group box" onClick={() => insert('common.group', 420, 280)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><rect x="2.5" y="2.5" width="13" height="13" rx="2" fill="none" stroke="currentColor" strokeWidth="1.6" strokeDasharray="3 2"/></svg>
        </button>
        <button title="Add users" onClick={() => insert('common.users', 100, 90)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><circle cx="6.5" cy="6" r="2.6" fill="currentColor"/><circle cx="12" cy="6.5" r="2.2" fill="currentColor" opacity=".7"/><path d="M2 15 C2 11.5 11 11.5 11 15 Z" fill="currentColor"/><path d="M10.5 12.2 C13.5 11.6 16 12.8 16 15 L12 15 C12 13.8 11.5 12.9 10.5 12.2 Z" fill="currentColor" opacity=".7"/></svg>
        </button>
      </div>
      <div className="card">
        <button title="Undo (⌘Z)" onClick={undo} disabled={!canUndo}>↶</button>
        <button title="Redo (⌘⇧Z)" onClick={redo} disabled={!canRedo}>↷</button>
        <span className="sep" />
        <button title="Arrange… (⌘⇧L)" onClick={() => setArrangeDialog({ ...DEFAULT_LAYOUT })} disabled={nodeCount === 0}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><rect x="2" y="7" width="4" height="4" rx="1" fill="currentColor"/><rect x="12" y="2" width="4" height="4" rx="1" fill="currentColor"/><rect x="12" y="12" width="4" height="4" rx="1" fill="currentColor"/><path d="M6 9 H9 M9 4 V14 M9 4 H12 M9 14 H12" fill="none" stroke="currentColor" strokeWidth="1.4"/></svg>
        </button>
      </div>
      <div className="card">
        <button className={showInspector ? 'on' : ''} title="Settings panel" onClick={() => setShowInspector(!showInspector)}>
          <svg width="18" height="18" viewBox="0 0 18 18" aria-hidden><rect x="2" y="2.5" width="14" height="13" rx="2" fill="none" stroke="currentColor" strokeWidth="1.6"/><line x1="10.5" y1="2.5" x2="10.5" y2="15.5" stroke="currentColor" strokeWidth="1.6"/></svg>
        </button>
      </div>
    </nav>
  )
}
