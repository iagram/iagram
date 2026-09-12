import { useStore } from '../store'

const SHORTCUTS: [string, string][] = [
  ['⌘S', 'Save'],
  ['⌘Z / ⌘⇧Z', 'Undo / redo'],
  ['⌘X / ⌘C / ⌘V / ⌘D', 'Cut / copy / paste / duplicate selection'],
  ['⌘] / ⌘[ (+⇧)', 'Bring forward / send backward (to front / to back)'],
  ['Right-click', 'Context menu on elements, connections and the canvas'],
  ['⌫', 'Delete selection'],
  ['⌘A / Esc', 'Select all / deselect'],
  ['Shift + drag', 'Box-select'],
  ['⌘ + click', 'Add to selection'],
  ['Drag into a container', 'Move an element there (refused if not allowed)'],
  ['Double-click', 'Focus the settings panel'],
  ['?', 'This list'],
]

export function InfoModals() {
  const modal = useStore((s) => s.modal)
  const setModal = useStore((s) => s.setModal)
  const version = useStore((s) => s.version)
  if (!modal) return null
  return (
    <div className="modal-backdrop" onClick={() => setModal(null)}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        {modal === 'shortcuts' ? (
          <>
            <h2>Keyboard shortcuts</h2>
            <table className="shortcuts">
              <tbody>
                {SHORTCUTS.map(([k, v]) => (
                  <tr key={k}>
                    <td>
                      <kbd>{k}</kbd>
                    </td>
                    <td>{v}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        ) : (
          <>
            <h2>iagram {version}</h2>
            <p>
              Infrastructure as diagram. Local-only: the canvas, the validator, the Terraform generator and OpenTofu all run in this process on your machine. Your credentials never pass through
              iagram.
            </p>
            <p className="muted">
              Apache-2.0 · <a href="https://github.com/iagram/iagram" target="_blank" rel="noreferrer">github.com/iagram/iagram</a>
            </p>
          </>
        )}
        <div className="actions">
          <button className="primary" onClick={() => setModal(null)}>
            Close
          </button>
        </div>
      </div>
    </div>
  )
}
