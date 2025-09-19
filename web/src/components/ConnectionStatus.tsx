import React from 'react'

const ConnectionStatus: React.FC = () => {
  const [healthStatus, setHealthStatus] = React.useState('checking')
  const [vaultStatus, setVaultStatus] = React.useState<any>(null)
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    const checkStatus = async () => {
      try {
        setLoading(true)
        
        // Check API health
        const healthResponse = await fetch('/api/v1/health')
        const healthData = await healthResponse.json()
        setHealthStatus(healthData.status || 'ok')
        
        // Check Vault status
        const vaultResponse = await fetch('/api/v1/vault/status')
        const vaultData = await vaultResponse.json()
        setVaultStatus(vaultData)
      } catch (error) {
        setHealthStatus('error')
        setVaultStatus({ error: 'Connection failed' })
      } finally {
        setLoading(false)
      }
    }

    checkStatus()
    
    // Refresh status every 30 seconds
    const interval = setInterval(checkStatus, 30000)
    return () => clearInterval(interval)
  }, [])

  if (loading) {
    return (
      <div className="text-white-50">
        <div className="d-flex align-items-center mb-2">
          <div className="spinner-border spinner-border-sm me-2" role="status">
            <span className="visually-hidden">Loading...</span>
          </div>
          <small>Checking status...</small>
        </div>
      </div>
    )
  }

  const isHealthy = healthStatus === 'ok'
  const isVaultConnected = vaultStatus && !vaultStatus.error

  return (
    <div className="text-white-50">
      <small className="text-uppercase fw-bold d-block mb-2">Connection Status</small>
      
      {/* API Health */}
      <div className="d-flex align-items-center mb-2">
        <i className={`bi bi-circle-fill me-2 ${isHealthy ? 'text-success' : 'text-danger'}`}></i>
        <small>API: {healthStatus}</small>
      </div>

      {/* Vault Status */}
      <div className="d-flex align-items-center mb-2">
        <i className={`bi bi-circle-fill me-2 ${isVaultConnected ? 'text-success' : 'text-danger'}`}></i>
        <small>
          Vault: {isVaultConnected ? 'connected' : 'disconnected'}
        </small>
      </div>

      {/* Vault Details */}
      {isVaultConnected && vaultStatus.token_info && (
        <div className="mt-2 pt-2 border-top border-secondary">
          <small className="text-white-50 d-block">
            <i className="bi bi-person-circle me-1"></i>
            {vaultStatus.token_info.display_name || 'Anonymous'}
          </small>
          {vaultStatus.token_info.policies && vaultStatus.token_info.policies.length > 0 && (
            <small className="text-white-50 d-block mt-1">
              <i className="bi bi-shield-check me-1"></i>
              {vaultStatus.token_info.policies.slice(0, 2).join(', ')}
              {vaultStatus.token_info.policies.length > 2 && '...'}
            </small>
          )}
          {vaultStatus.vault_address && (
            <small className="text-white-50 d-block mt-1 text-truncate" title={vaultStatus.vault_address}>
              <i className="bi bi-server me-1"></i>
              {new URL(vaultStatus.vault_address).hostname}
            </small>
          )}
        </div>
      )}

      {/* Error Details */}
      {vaultStatus?.error && (
        <div className="mt-2 pt-2 border-top border-secondary">
          <small className="text-danger">
            <i className="bi bi-exclamation-triangle me-1"></i>
            {typeof vaultStatus.error === 'string' ? vaultStatus.error : 'Connection error'}
          </small>
        </div>
      )}
    </div>
  )
}

export default ConnectionStatus
