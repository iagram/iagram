import { useEffect } from 'react'
import { ReactFlowProvider } from '@xyflow/react'
import { useStore } from './store'
import { Palette } from './components/Palette'
import { Canvas } from './components/Canvas'
import { PropertyPanel } from './components/PropertyPanel'
import { Toolbar } from './components/Toolbar'
import { ProblemsBar } from './components/ProblemsBar'

export function App() {
  const load = useStore((s) => s.load)
  const save = useStore((s) => s.save)
  const error = useStore((s) => s.error)
  const toast = useStore((s) => s.toast)
  const catalog = useStore((s) => s.catalog)
  const dirty = useStore((s) => s.dirty)

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') {
        e.preventDefault()
        void save()
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
  }, [save, dirty])

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
        <ProblemsBar />
        {toast && <div className="toast">{toast}</div>}
        {error && <div className="toast error">{error}</div>}
      </div>
    </ReactFlowProvider>
  )
}
