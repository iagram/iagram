import type { StoreApi } from 'zustand'
import type { ReactFlowState } from '@xyflow/react'

// Set by Canvas once mounted; menus use it for selection operations that
// must go through React Flow (see store.select for why).
let api: StoreApi<ReactFlowState> | null = null
export const setRfStore = (s: StoreApi<ReactFlowState> | null) => {
  api = s
}
export const rfStore = () => api
