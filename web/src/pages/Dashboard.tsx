import React from 'react'
import { Link } from 'react-router-dom'

const Dashboard: React.FC = () => {
  const [stats, setStats] = React.useState({
    totalSecrets: 0,
    totalFolders: 0,
    lastActivity: null,
    recentPaths: [] as string[]
  })

  React.useEffect(() => {
    // Load recent activity from localStorage or API
    const recentPaths = JSON.parse(localStorage.getItem('recentPaths') || '[]')
    setStats(prev => ({
      ...prev,
      recentPaths: recentPaths.slice(0, 5)
    }))
  }, [])

  return (
    <div className="container-fluid">
      <div className="row">
        <div className="col-12">
          <div className="d-flex justify-content-between align-items-start mb-4">
            <div>
              <h3 className="mb-1">Welcome to Vault Envrc Generator</h3>
              <p className="text-muted mb-0">
                Explore your Vault secrets and generate configuration files with ease.
              </p>
            </div>
            <div className="d-flex gap-2">
              <Link to="/explorer" className="btn btn-primary">
                <i className="bi bi-tree me-1"></i>
                Browse Secrets
              </Link>
              <Link to="/generator" className="btn btn-outline-primary">
                <i className="bi bi-file-earmark-code me-1"></i>
                Generate Files
              </Link>
            </div>
          </div>
        </div>
      </div>

      <div className="row g-4 mb-4">
        {/* Quick Stats */}
        <div className="col-md-3">
          <div className="card h-100">
            <div className="card-body text-center">
              <i className="bi bi-key-fill text-primary mb-2" style={{ fontSize: '2rem' }}></i>
              <h5 className="card-title">Secrets</h5>
              <p className="card-text display-6 text-primary mb-0">{stats.totalSecrets}</p>
              <small className="text-muted">accessible secrets</small>
            </div>
          </div>
        </div>

        <div className="col-md-3">
          <div className="card h-100">
            <div className="card-body text-center">
              <i className="bi bi-folder-fill text-warning mb-2" style={{ fontSize: '2rem' }}></i>
              <h5 className="card-title">Folders</h5>
              <p className="card-text display-6 text-warning mb-0">{stats.totalFolders}</p>
              <small className="text-muted">directories found</small>
            </div>
          </div>
        </div>

        <div className="col-md-3">
          <div className="card h-100">
            <div className="card-body text-center">
              <i className="bi bi-clock-fill text-info mb-2" style={{ fontSize: '2rem' }}></i>
              <h5 className="card-title">Last Activity</h5>
              <p className="card-text h6 text-info mb-0">
                {stats.lastActivity || 'Just now'}
              </p>
              <small className="text-muted">last exploration</small>
            </div>
          </div>
        </div>

        <div className="col-md-3">
          <div className="card h-100">
            <div className="card-body text-center">
              <i className="bi bi-lightning-fill text-success mb-2" style={{ fontSize: '2rem' }}></i>
              <h5 className="card-title">Quick Actions</h5>
              <div className="d-flex flex-column gap-1">
                <Link to="/explorer?path=secrets/" className="btn btn-sm btn-outline-success">
                  Browse Root
                </Link>
                <Link to="/generator" className="btn btn-sm btn-outline-success">
                  Generate
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div className="row g-4">
        {/* Recent Paths */}
        <div className="col-md-6">
          <div className="card">
            <div className="card-header d-flex justify-content-between align-items-center">
              <h6 className="mb-0">
                <i className="bi bi-clock-history me-1"></i>
                Recent Paths
              </h6>
              <button className="btn btn-sm btn-outline-secondary">
                <i className="bi bi-arrow-clockwise"></i>
              </button>
            </div>
            <div className="card-body">
              {stats.recentPaths.length > 0 ? (
                <div className="list-group list-group-flush">
                  {stats.recentPaths.map((path, index) => (
                    <Link
                      key={index}
                      to={`/explorer?path=${encodeURIComponent(path)}`}
                      className="list-group-item list-group-item-action d-flex align-items-center"
                    >
                      <i className="bi bi-folder me-2 text-muted"></i>
                      <code className="text-primary">{path}</code>
                    </Link>
                  ))}
                </div>
              ) : (
                <div className="text-center text-muted py-4">
                  <i className="bi bi-folder2-open" style={{ fontSize: '2rem' }}></i>
                  <p className="mt-2 mb-0">No recent paths</p>
                  <small>Start exploring to see your recent activity</small>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Getting Started */}
        <div className="col-md-6">
          <div className="card">
            <div className="card-header">
              <h6 className="mb-0">
                <i className="bi bi-info-circle me-1"></i>
                Getting Started
              </h6>
            </div>
            <div className="card-body">
              <div className="list-group list-group-flush">
                <div className="list-group-item d-flex align-items-start">
                  <div className="badge bg-primary rounded-pill me-3">1</div>
                  <div>
                    <h6 className="mb-1">Explore Your Secrets</h6>
                    <p className="mb-1 text-muted">
                      Use the <Link to="/explorer" className="text-decoration-none">Explorer</Link> to 
                      browse your Vault secret structure and view individual secrets.
                    </p>
                  </div>
                </div>
                
                <div className="list-group-item d-flex align-items-start">
                  <div className="badge bg-primary rounded-pill me-3">2</div>
                  <div>
                    <h6 className="mb-1">Generate Configuration</h6>
                    <p className="mb-1 text-muted">
                      Use the <Link to="/generator" className="text-decoration-none">Generator</Link> to 
                      create .envrc, JSON, or YAML files from your secrets.
                    </p>
                  </div>
                </div>
                
                <div className="list-group-item d-flex align-items-start">
                  <div className="badge bg-primary rounded-pill me-3">3</div>
                  <div>
                    <h6 className="mb-1">Configure Settings</h6>
                    <p className="mb-1 text-muted">
                      Visit <Link to="/settings" className="text-decoration-none">Settings</Link> to 
                      configure your preferences and connection details.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Dashboard


