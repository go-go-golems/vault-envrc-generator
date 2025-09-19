import React from 'react'
import { useAppSelector, useAppDispatch } from '../hooks/useAppSelector'
import { setPath, setIncludeValues, setReveal, setDepth, setLoading, setError, setTree, toggleNode } from '../store/vaultSlice'

interface Props {
  rootPath: string
}

export const VaultTree: React.FC<Props> = ({ rootPath }) => {
  const dispatch = useAppDispatch()
  const { currentPath: path, tree, loading, error, includeValues, reveal, depth, expandedNodes } = useAppSelector(state => state.vault)
  
  React.useEffect(() => {
    if (path !== rootPath) {
      dispatch(setPath(rootPath))
    }
  }, [rootPath, path, dispatch])

  const fetchTree = React.useCallback(async () => {
    dispatch(setLoading(true))
    dispatch(setError(null))
    try {
      const params = new URLSearchParams()
      params.set('path', path)
      params.set('depth', String(depth))
      if (includeValues) params.set('include', 'values')
      if (reveal) params.set('reveal', 'true')
      const r = await fetch(`/api/v1/vault/tree?${params.toString()}`)
      if (!r.ok) throw new Error(`${r.status}`)
      const j = await r.json()
      dispatch(setTree(j.tree ?? {}))
    } catch (e: any) {
      dispatch(setError(e?.message ?? 'error'))
    } finally {
      dispatch(setLoading(false))
    }
  }, [dispatch, path, includeValues, reveal, depth])

  React.useEffect(() => {
    fetchTree()
  }, [fetchTree])

  return (
    <div>
      <div className="card mb-3">
        <div className="card-body">
          <div className="row g-3 align-items-center">
            <div className="col-auto">
              <label className="form-label fw-semibold mb-0">Path:</label>
            </div>
            <div className="col-auto">
              <input 
                className="form-control form-control-sm"
                style={{ minWidth: 200 }}
                value={path} 
                onChange={(e) => dispatch(setPath(e.target.value))} 
              />
            </div>
            <div className="col-auto">
              <label className="form-label fw-semibold mb-0">Depth:</label>
            </div>
            <div className="col-auto">
              <input 
                type="number" 
                className="form-control form-control-sm"
                style={{ width: 70 }}
                value={depth} 
                onChange={(e) => dispatch(setDepth(Number(e.target.value || 0)))} 
              />
            </div>
            <div className="col-auto">
              <div className="form-check">
                <input 
                  className="form-check-input" 
                  type="checkbox" 
                  checked={includeValues} 
                  onChange={(e) => dispatch(setIncludeValues(e.target.checked))} 
                />
                <label className="form-check-label">Include values</label>
              </div>
            </div>
            <div className="col-auto">
              <div className="form-check">
                <input 
                  className="form-check-input" 
                  type="checkbox" 
                  checked={reveal} 
                  onChange={(e) => dispatch(setReveal(e.target.checked))} 
                />
                <label className="form-check-label">Reveal</label>
              </div>
            </div>
            <div className="col-auto">
              <button className="btn btn-primary btn-sm" onClick={fetchTree}>
                Refresh
              </button>
            </div>
          </div>
        </div>
      </div>
      {loading && <div className="alert alert-info">Loading...</div>}
      {error && <div className="alert alert-danger">Error: {error}</div>}
      {!loading && !error && (
        <div className="card">
          <div className="card-body p-0">
            <TreeNode name={path.replace(/\/$/, '') || 'root'} data={tree} level={0} nodePath="" />
          </div>
        </div>
      )}
    </div>
  )
}

interface NodeProps {
  name: string
  data: any
  level: number
  nodePath: string
}

const TreeNode: React.FC<NodeProps> = ({ name, data, level, nodePath }) => {
  const dispatch = useAppDispatch()
  const expandedNodes = useAppSelector(state => state.vault.expandedNodes)
  const { reveal } = useAppSelector(state => state.vault)
  const isFolder = data && typeof data === 'object' && !data.__secret__
  const isSecret = data && typeof data === 'object' && data.__secret__
  const fullPath = nodePath ? `${nodePath}/${name}` : name
  const expanded = expandedNodes.includes(fullPath) || level < 1
  const [loadingSecret, setLoadingSecret] = React.useState(false)
  const [secretData, setSecretData] = React.useState<any>(null)

  const handleSecretClick = async () => {
    if (isSecret && !secretData && !loadingSecret) {
      setLoadingSecret(true)
      try {
        const secretPath = fullPath.replace(/^secrets\//, '')
        const params = new URLSearchParams()
        if (reveal) params.set('reveal', 'true')
        const r = await fetch(`/api/v1/vault/secrets/${secretPath}?${params.toString()}`)
        if (r.ok) {
          const j = await r.json()
          setSecretData(j.secrets)
        }
      } catch (e) {
        console.error('Failed to load secret:', e)
      } finally {
        setLoadingSecret(false)
      }
    }
  }
  
  return (
    <div className={level > 0 ? 'border-start' : ''}>
      <div 
        className={`p-2 d-flex align-items-center gap-2 ${(isFolder || isSecret) ? 'text-decoration-none' : ''}`}
        style={{ cursor: (isFolder || isSecret) ? 'pointer' : 'default' }}
        onClick={() => {
          if (isFolder) {
            dispatch(toggleNode(fullPath))
          } else if (isSecret) {
            handleSecretClick()
          }
        }}
        onMouseEnter={(e) => e.currentTarget.classList.add('bg-light')}
        onMouseLeave={(e) => e.currentTarget.classList.remove('bg-light')}
      >
        <span style={{ fontSize: 16 }}>
          {isFolder ? (expanded ? '📂' : '📁') : '🔑'}
        </span>
        <span className={`${isFolder ? 'fw-semibold' : ''} ${isSecret ? 'text-primary' : ''}`}>
          {name}
        </span>
        {name.endsWith('__error__') && <span className="text-danger small">⚠️</span>}
        {loadingSecret && <span className="spinner-border spinner-border-sm text-primary" />}
      </div>
      {expanded && isFolder && (
        <div className="ms-3">
          {Object.entries(data).map(([k, v]) => (
            <TreeNode key={k} name={k} data={v as any} level={level + 1} nodePath={fullPath} />
          ))}
        </div>
      )}
      {(isSecret && secretData) && (
        <div className="ms-4 me-2 mb-2">
          <div className="card">
            <div className="card-body p-2">
              <div className="d-flex justify-content-between align-items-center mb-2">
                <small className="text-muted">Secret: {fullPath}</small>
                <button 
                  className="btn btn-outline-secondary btn-sm"
                  onClick={(e) => {
                    e.stopPropagation()
                    setSecretData(null)
                  }}
                >
                  ✕
                </button>
              </div>
              <pre className="bg-light p-2 rounded small mb-0" style={{ fontSize: 11 }}>
                {JSON.stringify(secretData, null, 2)}
              </pre>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}


