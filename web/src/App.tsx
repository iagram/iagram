import { useEffect } from 'react'
import { ReactFlowProvider } from '@xyflow/react'
import { useStore } from './store'
import { Palette } from './components/Palette'
import { Canvas } from './components/Canvas'
import { PropertyPanel } from './components/PropertyPanel'
import { Toolbar } from './components/Toolbar'
import { ProblemsBar } from './components/ProblemsBar'
import { LogDrawer } from './components/LogDrawer'
import { ConfirmApply } from './components/ConfirmApply'

export function App() {
  const load = useStore((s) => s.load)
  const save = useStore((s) => s.save)
  const error = useStore((s) => s.error)
  const toast = useStore((s) => s.toast)
  const catalog = useStore((s) => s.catalog)
  const dirty = useStore((s) => s.dirty)
  const undo = useStore((s) => s.undo)
  const redo = useStore((s) => s.redo)
  const copySelection = useStore((s) => s.copySelection)
  const paste = useStore((s) => s.paste)
  const duplicateSelection = useStore((s) => s.duplicateSelection)

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey
      if (!mod) return
      const k = e.key.toLowerCase()
      const inField = (e.target as HTMLElement)?.closest('input, select, textarea')
      if (k === 's') {
        e.preventDefault()
        void save()
      } else if (k === 'z' && !inField) {
        e.preventDefault()
        if (e.shiftKey) redo()
        else undo()
      } else if (k === 'y' && !inField) {
        e.preventDefault()
        redo()
      } else if (k === 'c' && !inField) {
        copySelection()
      } else if (k === 'v' && !inField) {
        e.preventDefault()
        paste()
      } else if (k === 'd' && !inField) {
        e.preventDefault()
        duplicateSelection()
      }
    }
    const onUnload = (e: BeforeUnloadEvent) => {
      if (dirty) e.preventDefault()
    }
    window.addEventListener('keydown', onKey)
    window.addEventListener('beforeunload', onUnload)
    return () => {
      window.removeEventListener('keydown', onKey)
      window.removeEventListener('beforeunload', onUnload)
    }
  }, [save, dirty, undo, redo, copySelection, paste, duplicateSelection])

  if (error && !catalog) {
    return (
      <div className="fatal">
        <h1>iagram</h1>
        <p>Could not reach the local server: {error}</p>
        <p className="muted">Is `iagram up` running?</p>
      </div>
    )
  }

  return (
    <ReactFlowProvider>
      <div className="app">
        <Toolbar />
        <div className="body">
          <Palette />
          <Canvas />
          <PropertyPanel />
        </div>
        <LogDrawer />
        <ConfirmApply />
        <ProblemsBar />
        {toast && <div className="toast">{toast}</div>}
        {error && <div className="toast error">{error}</div>}
      </div>
    </ReactFlowProvider>
  )
}
