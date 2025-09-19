import React from 'react'
import { Link } from 'react-router-dom'
import { useAppSelector } from '../hooks/useAppSelector'

interface SecretViewerProps {
  secretPath: string
  onClose: () => void
}

const SecretViewer: React.FC<SecretViewerProps> = ({ secretPath, onClose }) => {
  const { reveal } = useAppSelector(state => state.vault)
  const [secret, setSecret] = React.useState<any>(null)
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [copySuccess, setCopySuccess] = React.useState<string | null>(null)

  React.useEffect(() => {
    const fetchSecret = async () => {
      setLoading(true)
      setError(null)
      
      try {
        // Convert full path to relative path for API
        const relativePath = secretPath.replace(/^secrets\//, '')
        const params = new URLSearchParams()
        if (reveal) params.set('reveal', 'true')
        
        const response = await fetch(`/api/v1/vault/secrets/${relativePath}?${params.toString()}`)
        
        if (!response.ok) {
          throw new Error(`Failed to fetch secret: ${response.status}`)
        }
        
        const data = await response.json()
        setSecret(data.secrets || {})
      } catch (error: any) {
        setError(error?.message || 'Failed to load secret')
      } finally {
        setLoading(false)
      }
    }

    fetchSecret()
  }, [secretPath, reveal])

  const copyToClipboard = async (text: string, key?: string) => {
    try {
      await navigator.clipboard.writeText(text)
      setCopySuccess(key || 'all')
      setTimeout(() => setCopySuccess(null), 2000)
    } catch (error) {
      console.error('Failed to copy to clipboard:', error)
    }
  }

  const copyAllAsEnv = () => {
    if (!secret) return
    
    const envVars = Object.entries(secret)
      .map(([key, value]) => `${key.toUpperCase()}="${value}"`)
      .join('\n')
    
    copyToClipboard(envVars, 'env')
  }

  const copyAllAsJSON = () => {
    if (!secret) return
    copyToClipboard(JSON.stringify(secret, null, 2), 'json')
  }

  if (loading) {
    return (
      <div className="card h-100">
        <div className="card-header d-flex justify-content-between align-items-center">
          <h6 className="mb-0">
            <i className="bi bi-key me-1"></i>
            Loading Secret...
          </h6>
          <button className="btn btn-sm btn-outline-secondary" onClick={onClose}>
            <i className="bi bi-x-lg"></i>
          </button>
        </div>
        <div className="card-body d-flex align-items-center justify-content-center">
          <div className="text-center">
            <div className="spinner-border text-primary mb-2" role="status">
              <span className="visually-hidden">Loading...</span>
            </div>
            <p className="text-muted">Loading secret details...</p>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="card h-100">
      <div className="card-header d-flex justify-content-between align-items-center">
        <h6 className="mb-0">
          <i className="bi bi-key me-1"></i>
          Secret Details
        </h6>
        <button className="btn btn-sm btn-outline-secondary" onClick={onClose}>
          <i className="bi bi-x-lg"></i>
        </button>
      </div>
      
      <div className="card-body p-0 d-flex flex-column" style={{ overflowY: 'auto' }}>
        {/* Path */}
        <div className="border-bottom p-3">
          <small className="text-muted d-block mb-1">Path:</small>
          <code className="text-primary">{secretPath}</code>
        </div>

        {/* Error */}
        {error && (
          <div className="alert alert-danger m-3 mb-0" role="alert">
            <i className="bi bi-exclamation-triangle me-1"></i>
            {error}
          </div>
        )}

        {/* Secret Content */}
        {secret && (
          <>
            {/* Action Buttons */}
            <div className="border-bottom p-3">
              <div className="d-flex gap-2 flex-wrap">
                <button
                  className="btn btn-sm btn-outline-primary"
                  onClick={copyAllAsEnv}
                  disabled={Object.keys(secret).length === 0}
                >
                  <i className="bi bi-clipboard me-1"></i>
                  {copySuccess === 'env' ? 'Copied!' : 'Copy as ENV'}
                </button>
                
                <button
                  className="btn btn-sm btn-outline-secondary"
                  onClick={copyAllAsJSON}
                  disabled={Object.keys(secret).length === 0}
                >
                  <i className="bi bi-filetype-json me-1"></i>
                  {copySuccess === 'json' ? 'Copied!' : 'Copy as JSON'}
                </button>
                
                <Link
                  to={`/generator?path=${encodeURIComponent(secretPath)}`}
                  className="btn btn-sm btn-success"
                >
                  <i className="bi bi-file-earmark-code me-1"></i>
                  Generate File
                </Link>
              </div>
            </div>

            {/* Key-Value Pairs */}
            <div className="flex-grow-1">
              {Object.keys(secret).length === 0 ? (
                <div className="text-center p-4 text-muted">
                  <i className="bi bi-inbox" style={{ fontSize: '2rem' }}></i>
                  <p className="mt-2 mb-0">No keys found</p>
                  <small>This secret appears to be empty</small>
                </div>
              ) : (
                <div className="list-group list-group-flush">
                  {Object.entries(secret).map(([key, value]) => (
                    <div key={key} className="list-group-item">
                      <div className="d-flex justify-content-between align-items-start">
                        <div className="flex-grow-1 me-2">
                          <div className="d-flex align-items-center mb-1">
                            <strong className="text-dark">{key}</strong>
                            <button
                              className="btn btn-sm btn-outline-secondary ms-2"
                              onClick={() => copyToClipboard(String(value), key)}
                              title={`Copy ${key} value`}
                            >
                              <i className="bi bi-clipboard" style={{ fontSize: '0.75rem' }}></i>
                              {copySuccess === key && (
                                <span className="ms-1" style={{ fontSize: '0.75rem' }}>✓</span>
                              )}
                            </button>
                          </div>
                          <div className="text-break">
                            {String(value).length > 100 ? (
                              <details>
                                <summary className="text-muted cursor-pointer">
                                  <code className="text-dark">
                                    {String(value).substring(0, 50)}...
                                  </code>
                                  <small className="ms-2">Click to expand</small>
                                </summary>
                                <pre className="mt-2 mb-0 bg-light p-2 rounded small">
                                  {String(value)}
                                </pre>
                              </details>
                            ) : (
                              <code className="text-dark">{String(value)}</code>
                            )}
                          </div>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>

      {/* Footer */}
      <div className="card-footer bg-light">
        <small className="text-muted">
          <i className="bi bi-info-circle me-1"></i>
          {reveal ? 'Values are revealed' : 'Values are censored'}
          {!reveal && (
            <span className="ms-2">
              - Toggle "Reveal secrets" to see full values
            </span>
          )}
        </small>
      </div>
    </div>
  )
}

export default SecretViewer
