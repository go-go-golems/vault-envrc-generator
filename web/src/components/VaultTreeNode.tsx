import React from 'react'
import { useAppDispatch, useAppSelector } from '../hooks/useAppSelector'
import { toggleNode } from '../store/vaultSlice'

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
  const expandedNodes = useAppSelector(state => state.vault.expandedNodes)
  
  const fullPath = path ? `${path}${name}` : name
  const isFolder = data && typeof data === 'object' && !data.__secret__
  const isSecret = data && typeof data === 'object' && data.__secret__
  const hasError = name.endsWith('__error__')
  const isExpanded = expandedNodes.includes(fullPath) || level === 0
  const isSelected = selectedSecret === fullPath

  const handleClick = () => {
    if (isFolder) {
      dispatch(toggleNode(fullPath))
    } else if (isSecret) {
      onSecretSelect(fullPath)
    }
  }

  const getIcon = () => {
    if (hasError) return 'bi-exclamation-triangle-fill text-danger'
    if (isSecret) return 'bi-key-fill text-primary'
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
        <i className={`bi ${getIcon()} me-2`} style={{ fontSize: '0.9rem' }}></i>
        
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
                onSecretSelect(fullPath)
              }}
              title="View secret details"
            >
              <i className="bi bi-eye" style={{ fontSize: '0.75rem' }}></i>
            </button>
          </div>
        )}

        {isFolder && (
          <i 
            className={`bi ${isExpanded ? 'bi-chevron-down' : 'bi-chevron-right'} text-muted ms-2`}
            style={{ fontSize: '0.75rem' }}
          ></i>
        )}
      </div>

      {isExpanded && isFolder && (
        <div>
          {Object.entries(data)
            .sort(([a], [b]) => {
              // Sort folders first, then secrets
              const aIsFolder = typeof data[a] === 'object' && !data[a].__secret__
              const bIsFolder = typeof data[b] === 'object' && !data[b].__secret__
              if (aIsFolder && !bIsFolder) return -1
              if (!aIsFolder && bIsFolder) return 1
              return a.localeCompare(b)
            })
            .map(([childName, childData]) => (
              <VaultTreeNode
                key={childName}
                name={childName}
                data={childData}
                path={`${fullPath}/`}
                level={level + 1}
                onSecretSelect={onSecretSelect}
                selectedSecret={selectedSecret}
              />
            ))}
        </div>
      )}
    </>
  )
}

export default VaultTreeNode
