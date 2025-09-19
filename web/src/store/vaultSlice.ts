import { createSlice, PayloadAction } from '@reduxjs/toolkit'

interface NodeData {
  children?: Record<string, NodeData>
  isSecret?: boolean
  secretData?: any
  loading?: boolean
  error?: string
  loaded?: boolean
}

interface VaultState {
  expandedNodes: string[]
  currentPath: string
  includeValues: boolean
  reveal: boolean
  depth: number
  loading: boolean
  error: string | null
  tree: Record<string, NodeData>
}

const initialState: VaultState = {
  expandedNodes: [],
  currentPath: 'secrets/',
  includeValues: false,
  reveal: false,
  depth: 2, // Default to 2 levels instead of unlimited
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
      // Reset tree when path changes
      state.tree = {}
      state.expandedNodes = []
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
    setTree: (state, action: PayloadAction<Record<string, NodeData>>) => {
      state.tree = action.payload
    },
    setNodeChildren: (state, action: PayloadAction<{ nodePath: string; children: Record<string, NodeData> }>) => {
      const { nodePath, children } = action.payload
      const pathParts = nodePath.split('/').filter(Boolean)
      
      let current = state.tree
      for (let i = 0; i < pathParts.length; i++) {
        const part = pathParts[i]
        if (!current[part]) {
          current[part] = {}
        }
        if (i === pathParts.length - 1) {
          current[part].children = children
          current[part].loaded = true
          current[part].loading = false
        } else {
          if (!current[part].children) {
            current[part].children = {}
          }
          current = current[part].children!
        }
      }
    },
    setNodeLoading: (state, action: PayloadAction<{ nodePath: string; loading: boolean }>) => {
      const { nodePath, loading } = action.payload
      const pathParts = nodePath.split('/').filter(Boolean)
      
      let current = state.tree
      for (let i = 0; i < pathParts.length; i++) {
        const part = pathParts[i]
        if (!current[part]) {
          current[part] = {}
        }
        if (i === pathParts.length - 1) {
          current[part].loading = loading
        } else {
          if (!current[part].children) {
            current[part].children = {}
          }
          current = current[part].children!
        }
      }
    },
    setNodeError: (state, action: PayloadAction<{ nodePath: string; error: string }>) => {
      const { nodePath, error } = action.payload
      const pathParts = nodePath.split('/').filter(Boolean)
      
      let current = state.tree
      for (let i = 0; i < pathParts.length; i++) {
        const part = pathParts[i]
        if (!current[part]) {
          current[part] = {}
        }
        if (i === pathParts.length - 1) {
          current[part].error = error
          current[part].loading = false
        } else {
          if (!current[part].children) {
            current[part].children = {}
          }
          current = current[part].children!
        }
      }
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
  setNodeChildren,
  setNodeLoading,
  setNodeError,
} = vaultSlice.actions

export default vaultSlice.reducer