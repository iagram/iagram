import { useStore } from '../store'

/** While a connection is being dragged, tells a node whether it can be its target. */
export function useTargetState(id: string, type: string): '' | 'target-ok' | 'target-no' {
  return useStore((s) => {
    if (!s.connectingFrom || s.connectingFrom === id || !s.rules) return ''
    const src = s.nodes.find((n) => n.id === s.connectingFrom)
    if (!src) return ''
    if (s.edges.some((e) => e.source === src.id && e.target === id)) return 'target-no'
    return s.rules.connection(src.data.type, type) ? 'target-ok' : 'target-no'
  })
}
