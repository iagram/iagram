import { useEffect } from 'react'
import { ReactFlowProvider } from '@xyflow/react'
import { useStore } from './store'
import { Palette } from './components/Palette'
import { Canvas } from './components/Canvas'
import { PropertyPanel } from './components/PropertyPanel'
import { Toolbar } from './components/Toolbar'
import { CanvasTabs } from './components/CanvasTabs'
import { ProblemsBar } from './components/ProblemsBar'
import { LogDrawer } from './components/LogDrawer'
import { ConfirmApply } from './components/ConfirmApply'
import { InfoModals } from './components/InfoModals'

export function App() {
  const load = useStore((s) => s.load)
  const save = useStore((s) => s.save)
  const error = useStore((s) => s.error)
  const toast = useStore((s) => s.toast)
  const catalog = useStore((s) => s.catalog)
  const dirty = useStore((s) => s.dirty)
  const theme = useStore((s) => s.theme)
  useEffect(() => {
    document.documentElement.dataset.theme = theme
  }, [theme])
  const undo = useStore((s) => s.undo)
  const redo = useStore((s) => s.redo)
  const copySelection = useStore((s) => s.copySelection)
  const paste = useStore((s) => s.paste)
  const duplicateSelection = useStore((s) => s.duplicateSelection)
  const cutSelection = useStore((s) => s.cutSelection)
  const reorder = useStore((s) => s.reorder)
  const selectAll = useStore((s) => s.selectAll)
  const deselectAll = useStore((s) => s.deselectAll)
  const setModal = useStore((s) => s.setModal)

  useEffect(() => {
    // Deep links: #select=<node id> focuses an element, ?theme=dark|light forces a theme.
    void load().then(() => {
      const params = new URLSearchParams(location.search)
      const t = params.get('theme')
      if (t === 'dark' || t === 'light') useStore.getState().setTheme(t)
      const prov = params.get('provider')
      if (prov) useStore.getState().setActiveProvider(prov)
      const m = /select=([^&]+)/.exec(location.hash)
      if (m) useStore.getState().requestSelect(decodeURIComponent(m[1]))
    })
  }, [load])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey
      const k = e.key.toLowerCase()
      const inField = (e.target as HTMLElement)?.closest('input, select, textarea')
      if (!mod) {
        if (e.key === 'Escape') {
          deselectAll()
          setModal(null)
        } else if (e.key === '?' && !inField) setModal('shortcuts')
        return
      }
      if (k === 'a' && !inField) {
        e.preventDefault()
        selectAll()
        return
      }
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
      } else if (k === 'x' && !inField) {
        e.preventDefault()
        cutSelection()
      } else if (e.key === ']' && !inField) {
        e.preventDefault()
        reorder(e.shiftKey ? 'front' : 'forward')
      } else if (e.key === '[' && !inField) {
        e.preventDefault()
        reorder(e.shiftKey ? 'back' : 'backward')
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
  }, [save, dirty, undo, redo, copySelection, paste, duplicateSelection, cutSelection, reorder, selectAll, deselectAll, setModal])

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
          <div className="center">
            <CanvasTabs />
            <Canvas />
          </div>
          <PropertyPanel />
        </div>
        <LogDrawer />
        <ConfirmApply />
        <InfoModals />
        <ProblemsBar />
        {toast && <div className="toast">{toast}</div>}
        {error && <div className="toast error">{error}</div>}
      </div>
    </ReactFlowProvider>
  )
}
