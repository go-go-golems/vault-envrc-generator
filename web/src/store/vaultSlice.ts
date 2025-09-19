import { createSlice, PayloadAction } from '@reduxjs/toolkit'

interface VaultState {
  expandedNodes: string[]
  currentPath: string
  includeValues: boolean
  reveal: boolean
  depth: number
  loading: boolean
  error: string | null
  tree: Record<string, any>
}

const initialState: VaultState = {
  expandedNodes: [],
  currentPath: 'secrets/',
  includeValues: false,
  reveal: false,
  depth: 0, // 0 = unlimited depth
  loading: false,
  error: null,
  tree: {},
}

const vaultSlice = createSlice({
  name: 'vault',
  initialState,
  reducers: {
    toggleNode: (state, action: PayloadAction<string>) => {
      const nodePath = action.payload
      const index = state.expandedNodes.indexOf(nodePath)
      if (index >= 0) {
        state.expandedNodes.splice(index, 1)
      } else {
        state.expandedNodes.push(nodePath)
      }
    },
    setPath: (state, action: PayloadAction<string>) => {
      state.currentPath = action.payload
    },
    setIncludeValues: (state, action: PayloadAction<boolean>) => {
      state.includeValues = action.payload
    },
    setReveal: (state, action: PayloadAction<boolean>) => {
      state.reveal = action.payload
    },
    setDepth: (state, action: PayloadAction<number>) => {
      state.depth = action.payload
    },
    setLoading: (state, action: PayloadAction<boolean>) => {
      state.loading = action.payload
    },
    setError: (state, action: PayloadAction<string | null>) => {
      state.error = action.payload
    },
    setTree: (state, action: PayloadAction<Record<string, any>>) => {
      state.tree = action.payload
    },
  },
})

export const {
  toggleNode,
  setPath,
  setIncludeValues,
  setReveal,
  setDepth,
  setLoading,
  setError,
  setTree,
} = vaultSlice.actions

export default vaultSlice.reducer
