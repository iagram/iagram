import { useReactFlow, useViewport } from '@xyflow/react'
import { useStore } from '../store'

/** Floating undo/redo, zoom, labels and theme controls at the bottom of the canvas. */
export function CanvasControls() {
  const canUndo = useStore((s) => s.past.length > 0)
  const canRedo = useStore((s) => s.future.length > 0)
  const undo = useStore((s) => s.undo)
  const redo = useStore((s) => s.redo)
  const theme = useStore((s) => s.theme)
  const setTheme = useStore((s) => s.setTheme)
  const showLabels = useStore((s) => s.showLabels)
  const setShowLabels = useStore((s) => s.setShowLabels)
  const { zoomIn, zoomOut, fitView, zoomTo } = useReactFlow()
  const { zoom } = useViewport()
  return (
    <div className="canvas-controls">
      <span className="group">
        <button onClick={undo} disabled={!canUndo} title="Undo (⌘Z)">
          ↶
        </button>
        <button onClick={redo} disabled={!canRedo} title="Redo (⌘⇧Z)">
          ↷
        </button>
      </span>
      <span className="group">
        <button onClick={() => void zoomOut()} title="Zoom out (⌘−)">
          −
        </button>
        <button className="zoom" onClick={() => void zoomTo(1)} title="Reset to 100%">
          {Math.round(zoom * 100)}%
        </button>
        <button onClick={() => void zoomIn()} title="Zoom in (⌘+)">
          +
        </button>
        <button onClick={() => void fitView({ padding: 0.1 })} title="Fit diagram">
          ⛶
        </button>
      </span>
      <span className="group">
        <button className={showLabels ? 'on' : ''} onClick={() => setShowLabels(!showLabels)} title={showLabels ? 'Hide connection labels (shown on hover)' : 'Show connection labels'}>
          Aa
        </button>
        <button onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')} title={theme === 'dark' ? 'Light theme' : 'Dark theme'}>
          {theme === 'dark' ? '☀' : '☾'}
        </button>
      </span>
    </div>
  )
}
