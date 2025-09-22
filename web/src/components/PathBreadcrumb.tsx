import React from 'react'

interface PathBreadcrumbProps {
  path: string
  onPathChange: (path: string) => void
}

const PathBreadcrumb: React.FC<PathBreadcrumbProps> = ({ path, onPathChange }) => {
  const segments = React.useMemo(() => {
    if (!path) return []
    
    const parts = path.split('/').filter(Boolean)
    const segments = []
    let currentPath = ''
    
    // Add root
    segments.push({ name: 'root', path: '' })
    
    // Add each segment
    for (const part of parts) {
      currentPath += part + '/'
      segments.push({ name: part, path: currentPath })
    }
    
    return segments
  }, [path])

  if (segments.length <= 1) {
    return (
      <nav aria-label="breadcrumb" className="mb-3">
        <ol className="breadcrumb mb-0 bg-white rounded p-2 shadow-sm">
          <li className="breadcrumb-item active">
            <i className="bi bi-house me-1"></i>
            Root
          </li>
        </ol>
      </nav>
    )
  }

  return (
    <nav aria-label="breadcrumb" className="mb-3">
      <ol className="breadcrumb mb-0 bg-white rounded p-2 shadow-sm">
        {segments.map((segment, index) => {
          const isLast = index === segments.length - 1
          
          if (isLast) {
            return (
              <li key={index} className="breadcrumb-item active" aria-current="page">
                {index === 0 ? (
                  <i className="bi bi-house me-1"></i>
                ) : (
                  <i className="bi bi-folder me-1"></i>
                )}
                {segment.name || 'root'}
              </li>
            )
          }
          
          return (
            <li key={index} className="breadcrumb-item">
              <button
                className="btn btn-link p-0 text-decoration-none"
                onClick={() => onPathChange(segment.path)}
                title={`Navigate to ${segment.path || 'root'}`}
              >
                {index === 0 ? (
                  <i className="bi bi-house me-1"></i>
                ) : (
                  <i className="bi bi-folder me-1"></i>
                )}
                {segment.name || 'root'}
              </button>
            </li>
          )
        })}
        
        {/* Quick navigation shortcuts */}
        <li className="breadcrumb-item ms-auto">
          <div className="btn-group" role="group">
            <button
              className="btn btn-outline-secondary btn-sm"
              onClick={() => onPathChange('secrets/')}
              title="Go to secrets root"
            >
              <i className="bi bi-house"></i>
            </button>
            
            <button
              className="btn btn-outline-secondary btn-sm"
              onClick={() => {
                // Go up one level
                const parentPath = path.replace(/[^/]*\/$/, '')
                onPathChange(parentPath)
              }}
              title="Go up one level"
              disabled={segments.length <= 2}
            >
              <i className="bi bi-arrow-up"></i>
            </button>
            
            <button
              className="btn btn-outline-secondary btn-sm"
              onClick={() => {
                const newPath = prompt('Enter path:', path)
                if (newPath !== null) {
                  onPathChange(newPath)
                }
              }}
              title="Go to specific path"
            >
              <i className="bi bi-pencil"></i>
            </button>
          </div>
        </li>
      </ol>
    </nav>
  )
}

export default PathBreadcrumb


