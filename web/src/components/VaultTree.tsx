import React from 'react'

type TreeData = Record<string, any>

interface Props {
  rootPath: string
}

export const VaultTree: React.FC<Props> = ({ rootPath }) => {
  const [path, setPath] = React.useState(rootPath)
  const [tree, setTree] = React.useState<TreeData>({})
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [includeValues, setIncludeValues] = React.useState(false)
  const [reveal, setReveal] = React.useState(false)
  const [depth, setDepth] = React.useState(2)

  const fetchTree = React.useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const params = new URLSearchParams()
      params.set('path', path)
      params.set('depth', String(depth))
      if (includeValues) params.set('include', 'values')
      if (reveal) params.set('reveal', 'true')
      const r = await fetch(`/api/v1/vault/tree?${params.toString()}`)
      if (!r.ok) throw new Error(`${r.status}`)
      const j = await r.json()
      setTree(j.tree ?? {})
    } catch (e: any) {
      setError(e?.message ?? 'error')
    } finally {
      setLoading(false)
    }
  }, [path, includeValues, reveal, depth])

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
                onChange={(e) => setPath(e.target.value)} 
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
                onChange={(e) => setDepth(Number(e.target.value || 0))} 
              />
            </div>
            <div className="col-auto">
              <div className="form-check">
                <input 
                  className="form-check-input" 
                  type="checkbox" 
                  checked={includeValues} 
                  onChange={(e) => setIncludeValues(e.target.checked)} 
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
                  onChange={(e) => setReveal(e.target.checked)} 
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
            <TreeNode name={path.replace(/\/$/, '') || 'root'} data={tree} level={0} />
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
}

const TreeNode: React.FC<NodeProps> = ({ name, data, level }) => {
  const isFolder = data && typeof data === 'object' && !data.__secret__
  const isSecret = data && typeof data === 'object' && data.__secret__
  const [expanded, setExpanded] = React.useState(level < 1)
  
  return (
    <div className={level > 0 ? 'border-start' : ''}>
      <div 
        className={`p-2 d-flex align-items-center gap-2 ${isFolder ? 'text-decoration-none' : ''}`}
        style={{ cursor: isFolder ? 'pointer' : 'default' }}
        onClick={() => isFolder && setExpanded((v) => !v)}
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
      </div>
      {expanded && isFolder && (
        <div className="ms-3">
          {Object.entries(data).map(([k, v]) => (
            <TreeNode key={k} name={k} data={v as any} level={level + 1} />
          ))}
        </div>
      )}
      {isSecret && (
        <div className="ms-4 me-2 mb-2">
          <pre className="bg-light p-2 rounded small text-muted mb-0">
            {JSON.stringify(data, null, 2)}
          </pre>
        </div>
      )}
    </div>
  )
}


