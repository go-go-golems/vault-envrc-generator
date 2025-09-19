import React from 'react'
import { useSearchParams } from 'react-router-dom'
import { useAppSelector, useAppDispatch } from '../hooks/useAppSelector'
import { setPath, setIncludeValues, setReveal, setDepth, setLoading, setError, setTree, setNodeChildren } from '../store/vaultSlice'
import VaultTreeNode from '../components/VaultTreeNode'
import SecretViewer from '../components/SecretViewer'
import PathBreadcrumb from '../components/PathBreadcrumb'

const Explorer: React.FC = () => {
  const dispatch = useAppDispatch()
  const [searchParams, setSearchParams] = useSearchParams()
  const { currentPath, tree, loading, error, includeValues, reveal, depth } = useAppSelector(state => state.vault)
  const [selectedSecret, setSelectedSecret] = React.useState<string | null>(null)
  const [searchTerm, setSearchTerm] = React.useState('')

  // Initialize path from URL params
  React.useEffect(() => {
    const pathParam = searchParams.get('path')
    if (pathParam && pathParam !== currentPath) {
      dispatch(setPath(pathParam))
    }
  }, [searchParams, currentPath, dispatch])

  const fetchRootLevel = React.useCallback(async () => {
    if (!currentPath) return

    dispatch(setLoading(true))
    dispatch(setError(null))
    
    try {
      // Use the list endpoint to get immediate children only
      let cleanPath = currentPath.replace(/^secrets\//, '').replace(/\/$/, '')
      // Backend now accepts empty path for root listing
      const response = await fetch(`/api/v1/vault/list/${cleanPath}`)
      
      if (!response.ok) {
        throw new Error(`Failed to fetch directory: ${response.status} ${response.statusText}`)
      }
      
      const data = await response.json()
      
      // Convert list response to tree structure
      const rootTree: Record<string, any> = {}
      
      if (data.keys && Array.isArray(data.keys)) {
        for (const key of data.keys) {
          const isFolder = key.endsWith('/')
          const cleanKey = key.replace(/\/$/, '')
          
          rootTree[cleanKey] = {
            isSecret: !isFolder,
            children: isFolder ? {} : undefined,
            loaded: false
          }
        }
      }
      
      dispatch(setTree(rootTree))
      
      // Update recent paths
      const recentPaths = JSON.parse(localStorage.getItem('recentPaths') || '[]')
      const updatedPaths = [currentPath, ...recentPaths.filter(p => p !== currentPath)].slice(0, 10)
      localStorage.setItem('recentPaths', JSON.stringify(updatedPaths))
      
    } catch (error: any) {
      dispatch(setError(error?.message || 'Failed to load directory'))
    } finally {
      dispatch(setLoading(false))
    }
  }, [dispatch, currentPath])

  React.useEffect(() => {
    fetchRootLevel()
  }, [fetchRootLevel])

  const handlePathChange = (newPath: string) => {
    dispatch(setPath(newPath))
    setSearchParams({ path: newPath })
  }

  const handleSecretSelect = (secretPath: string) => {
    setSelectedSecret(secretPath)
  }

  const handleNodeExpand = async (nodePath: string) => {
    // This will be handled by the VaultTreeNode component itself
    console.log('Node expand requested:', nodePath)
  }

  const filteredTree = React.useMemo(() => {
    if (!searchTerm) return tree
    
    const filterNode = (node: any, path: string): any => {
      if (typeof node !== 'object' || !node) return node
      
      if (node.isSecret) {
        return path.toLowerCase().includes(searchTerm.toLowerCase()) ? node : null
      }
      
      const filtered: any = {}
      const children = node.children || {}
      for (const [key, value] of Object.entries(children)) {
        const fullPath = path ? `${path}/${key}` : key
        if (key.toLowerCase().includes(searchTerm.toLowerCase())) {
          filtered[key] = value
        } else {
          const filteredChild = filterNode(value, fullPath)
          if (filteredChild !== null) {
            filtered[key] = filteredChild
          }
        }
      }
      
      return Object.keys(filtered).length > 0 ? { ...node, children: filtered } : null
    }
    
    const result: Record<string, any> = {}
    for (const [key, value] of Object.entries(tree)) {
      const filteredNode = filterNode(value, key)
      if (filteredNode !== null) {
        result[key] = filteredNode
      }
    }
    
    return result
  }, [tree, searchTerm])

  return (
    <div className="container-fluid h-100">
      <div className="row h-100">
        {/* Tree Panel */}
        <div className={`col-md-${selectedSecret ? '8' : '12'} h-100 d-flex flex-column`}>
          {/* Controls */}
          <div className="card mb-3">
            <div className="card-body">
              <div className="row g-3 align-items-center">
                <div className="col-md-6">
                  <label className="form-label fw-semibold">Path:</label>
                  <div className="input-group">
                    <input
                      type="text"
                      className="form-control"
                      value={currentPath}
                      onChange={(e) => handlePathChange(e.target.value)}
                      placeholder="e.g., secrets/"
                    />
                    <button 
                      className="btn btn-outline-secondary"
                      onClick={() => handlePathChange('secrets/')}
                      title="Reset to root"
                    >
                      <i className="bi bi-house"></i>
                    </button>
                  </div>
                </div>
                
                <div className="col-md-2">
                  <label className="form-label fw-semibold">Auto-expand Depth:</label>
                  <select 
                    className="form-select"
                    value={depth}
                    onChange={(e) => dispatch(setDepth(Number(e.target.value)))}
                    title="How many levels to auto-expand when clicking folders"
                  >
                    <option value={1}>1 level</option>
                    <option value={2}>2 levels</option>
                    <option value={3}>3 levels</option>
                    <option value={5}>5 levels</option>
                  </select>
                </div>

                <div className="col-md-4">
                  <label className="form-label fw-semibold">Options:</label>
                  <div className="d-flex gap-3">
                    <div className="form-check">
                      <input
                        className="form-check-input"
                        type="checkbox"
                        id="includeValues"
                        checked={includeValues}
                        onChange={(e) => dispatch(setIncludeValues(e.target.checked))}
                      />
                      <label className="form-check-label" htmlFor="includeValues">
                        Load values
                      </label>
                    </div>
                    <div className="form-check">
                      <input
                        className="form-check-input"
                        type="checkbox"
                        id="reveal"
                        checked={reveal}
                        onChange={(e) => dispatch(setReveal(e.target.checked))}
                      />
                      <label className="form-check-label" htmlFor="reveal">
                        Reveal secrets
                      </label>
                    </div>
                  </div>
                </div>
              </div>

              <div className="row g-3 align-items-center mt-2">
                <div className="col-md-6">
                  <div className="input-group">
                    <span className="input-group-text">
                      <i className="bi bi-search"></i>
                    </span>
                    <input
                      type="text"
                      className="form-control"
                      placeholder="Search secrets..."
                      value={searchTerm}
                      onChange={(e) => setSearchTerm(e.target.value)}
                    />
                    {searchTerm && (
                      <button 
                        className="btn btn-outline-secondary"
                        onClick={() => setSearchTerm('')}
                      >
                        <i className="bi bi-x"></i>
                      </button>
                    )}
                  </div>
                </div>
                
                <div className="col-md-6">
                  <div className="d-flex gap-2 justify-content-end">
                    <button 
                      className="btn btn-primary"
                      onClick={fetchRootLevel}
                      disabled={loading}
                    >
                      {loading ? (
                        <>
                          <div className="spinner-border spinner-border-sm me-1" role="status">
                            <span className="visually-hidden">Loading...</span>
                          </div>
                          Loading...
                        </>
                      ) : (
                        <>
                          <i className="bi bi-arrow-clockwise me-1"></i>
                          Refresh
                        </>
                      )}
                    </button>
                    
                    {selectedSecret && (
                      <button 
                        className="btn btn-outline-secondary"
                        onClick={() => setSelectedSecret(null)}
                      >
                        <i className="bi bi-x-lg me-1"></i>
                        Close Viewer
                      </button>
                    )}
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Breadcrumb */}
          <PathBreadcrumb path={currentPath} onPathChange={handlePathChange} />

          {/* Error Alert */}
          {error && (
            <div className="alert alert-danger d-flex align-items-center" role="alert">
              <i className="bi bi-exclamation-triangle-fill me-2"></i>
              <div>
                <strong>Error:</strong> {error}
              </div>
            </div>
          )}

          {/* Tree */}
          <div className="card flex-grow-1">
            <div className="card-body p-0" style={{ overflowY: 'auto' }}>
              {Object.keys(filteredTree).length > 0 ? (
                <div className="tree-container">
                  {Object.entries(filteredTree)
                    .sort(([a, aData], [b, bData]) => {
                      // Sort folders first, then secrets
                      const aIsFolder = !aData.isSecret
                      const bIsFolder = !bData.isSecret
                      if (aIsFolder && !bIsFolder) return -1
                      if (!aIsFolder && bIsFolder) return 1
                      return a.localeCompare(b)
                    })
                    .map(([name, data]) => (
                      <VaultTreeNode
                        key={name}
                        name={name}
                        data={data}
                        path={currentPath}
                        level={0}
                        onSecretSelect={handleSecretSelect}
                        selectedSecret={selectedSecret}
                      />
                    ))}
                </div>
              ) : !loading && (
                <div className="text-center py-5 text-muted">
                  <i className="bi bi-folder2-open" style={{ fontSize: '3rem' }}></i>
                  <p className="mt-3 mb-1">
                    {searchTerm ? 'No matching secrets found' : 'No secrets found at this path'}
                  </p>
                  <small>
                    {searchTerm 
                      ? 'Try adjusting your search terms' 
                      : 'Check your path or try a different location'
                    }
                  </small>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Secret Viewer Panel */}
        {selectedSecret && (
          <div className="col-md-4 h-100">
            <SecretViewer 
              secretPath={selectedSecret} 
              onClose={() => setSelectedSecret(null)} 
            />
          </div>
        )}
      </div>
    </div>
  )
}

export default Explorer