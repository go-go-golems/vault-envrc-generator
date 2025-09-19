import React from 'react'
import { useAppDispatch, useAppSelector } from '../hooks/useAppSelector'
import { toggleNode, setNodeChildren, setNodeLoading, setNodeError } from '../store/vaultSlice'

interface VaultTreeNodeProps {
  name: string
  data: any
  path: string
  level: number
  onSecretSelect: (path: string) => void
  selectedSecret: string | null
}

const VaultTreeNode: React.FC<VaultTreeNodeProps> = ({ 
  name, 
  data, 
  path, 
  level, 
  onSecretSelect, 
  selectedSecret 
}) => {
  const dispatch = useAppDispatch()
  const { expandedNodes, depth, reveal } = useAppSelector(state => state.vault)
  
  const fullPath = path ? `${path}${name}/` : `${name}/`
  const isFolder = !data.isSecret
  const isSecret = data.isSecret
  const hasError = data.error || name.endsWith('__error__')
  const isExpanded = expandedNodes.includes(fullPath)
  const isSelected = selectedSecret === fullPath.replace(/\/$/, '')
  const isLoading = data.loading || false
  const hasChildren = data.children && Object.keys(data.children).length > 0
  const isLoaded = data.loaded || false

  const fetchChildren = async (nodePath: string) => {
    if (isLoaded || isLoading) return

    dispatch(setNodeLoading({ nodePath, loading: true }))
    
    try {
      // Convert full path to API path (remove secrets/ prefix and trailing slash)
      let apiPath = nodePath.replace(/^secrets\//, '').replace(/\/$/, '')
      // Backend now accepts empty path for root listing
      const response = await fetch(`/api/v1/vault/list/${apiPath}`)
      
      if (!response.ok) {
        throw new Error(`Failed to fetch children: ${response.status}`)
      }
      
      const responseData = await response.json()
      const children: Record<string, any> = {}
      
      if (responseData.keys && Array.isArray(responseData.keys)) {
        for (const key of responseData.keys) {
          const isChildFolder = key.endsWith('/')
          const cleanKey = key.replace(/\/$/, '')
          
          children[cleanKey] = {
            isSecret: !isChildFolder,
            children: isChildFolder ? {} : undefined,
            loaded: false
          }
        }
      }
      
      dispatch(setNodeChildren({ nodePath, children }))
    } catch (error: any) {
      dispatch(setNodeError({ nodePath, error: error?.message || 'Failed to load' }))
    }
  }

  const handleClick = async () => {
    if (isFolder) {
      const wasExpanded = isExpanded
      dispatch(toggleNode(fullPath))
      
      // If we're expanding and haven't loaded children yet, fetch them
      if (!wasExpanded && !isLoaded) {
        await fetchChildren(fullPath)
      }
    } else if (isSecret) {
      onSecretSelect(fullPath.replace(/\/$/, ''))
    }
  }

  // Auto-expand based on depth setting (only for newly loaded nodes)
  React.useEffect(() => {
    if (isFolder && !isExpanded && level < depth && !isLoaded) {
      dispatch(toggleNode(fullPath))
      fetchChildren(fullPath)
    }
  }, [isFolder, isExpanded, level, depth, isLoaded, fullPath, dispatch])

  const getIcon = () => {
    if (hasError) return 'bi-exclamation-triangle-fill text-danger'
    if (isSecret) return 'bi-key-fill text-primary'
    if (isLoading) return 'spinner-border spinner-border-sm text-warning'
    return isExpanded ? 'bi-folder2-open text-warning' : 'bi-folder-fill text-warning'
  }

  const getIndentStyle = () => ({
    paddingLeft: `${level * 1.5 + 0.75}rem`
  })

  return (
    <>
      <div
        className={`
          d-flex align-items-center py-2 px-3 border-bottom border-light cursor-pointer
          ${isSelected ? 'bg-primary text-white' : 'hover-bg-light'}
          ${hasError ? 'bg-danger-subtle' : ''}
        `}
        style={getIndentStyle()}
        onClick={handleClick}
        role="button"
        tabIndex={0}
      >
        <div className="me-2 d-flex align-items-center" style={{ width: '16px', height: '16px' }}>
          {isLoading ? (
            <div className="spinner-border spinner-border-sm text-primary" style={{ fontSize: '0.6rem', width: '12px', height: '12px' }} role="status">
              <span className="visually-hidden">Loading...</span>
            </div>
          ) : (
            <i className={`bi ${getIcon()}`} style={{ fontSize: '0.9rem' }}></i>
          )}
        </div>
        
        <span className={`flex-grow-1 ${isSecret ? 'fw-medium' : ''}`}>
          {hasError ? name.replace('__error__', '') : name}
        </span>

        {hasError && (
          <span className="badge bg-danger ms-2">Error</span>
        )}

        {isSecret && (
          <div className="d-flex align-items-center gap-1 ms-2">
            <button
              className={`btn btn-sm ${isSelected ? 'btn-light' : 'btn-outline-primary'}`}
              onClick={(e) => {
                e.stopPropagation()
                onSecretSelect(fullPath.replace(/\/$/, ''))
              }}
              title="View secret details"
            >
              <i className="bi bi-eye" style={{ fontSize: '0.75rem' }}></i>
            </button>
          </div>
        )}

        {isFolder && (
          <div className="d-flex align-items-center ms-2">
            {(hasChildren || isLoaded) && (
              <small className="text-muted me-2">
                {Object.keys(data.children || {}).length}
              </small>
            )}
            <i 
              className={`bi ${isExpanded ? 'bi-chevron-down' : 'bi-chevron-right'} text-muted`}
              style={{ fontSize: '0.75rem' }}
            ></i>
          </div>
        )}
      </div>

      {isExpanded && isFolder && hasChildren && (
        <div>
          {Object.entries(data.children || {})
            .sort(([a, aData], [b, bData]) => {
              // Sort folders first, then secrets
              const aIsFolder = !aData.isSecret
              const bIsFolder = !bData.isSecret
              if (aIsFolder && !bIsFolder) return -1
              if (!aIsFolder && bIsFolder) return 1
              return a.localeCompare(b)
            })
            .map(([childName, childData]) => (
              <VaultTreeNode
                key={childName}
                name={childName}
                data={childData}
                path={fullPath}
                level={level + 1}
                onSecretSelect={onSecretSelect}
                selectedSecret={selectedSecret}
              />
            ))}
        </div>
      )}

      {isExpanded && isFolder && data.error && (
        <div style={getIndentStyle()}>
          <div className="py-2 px-3 text-danger small">
            <i className="bi bi-exclamation-triangle me-1"></i>
            Error loading children: {data.error}
            <button 
              className="btn btn-sm btn-outline-danger ms-2"
              onClick={(e) => {
                e.stopPropagation()
                fetchChildren(fullPath)
              }}
            >
              <i className="bi bi-arrow-clockwise me-1"></i>
              Retry
            </button>
          </div>
        </div>
      )}
    </>
  )
}

export default VaultTreeNode