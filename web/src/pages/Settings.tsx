import React from 'react'
import { useAppDispatch, useAppSelector } from '../hooks/useAppSelector'
import { setDepth, setReveal, setIncludeValues } from '../store/vaultSlice'

const Settings: React.FC = () => {
  const dispatch = useAppDispatch()
  const { depth, reveal, includeValues } = useAppSelector(state => state.vault)
  
  const [vaultStatus, setVaultStatus] = React.useState<any>(null)
  const [connectionLoading, setConnectionLoading] = React.useState(false)
  const [testMessage, setTestMessage] = React.useState<string | null>(null)

  // Local settings state
  const [settings, setSettings] = React.useState({
    censorPrefix: 2,
    censorSuffix: 2,
    autoRefresh: false,
    showBreadcrumbs: true,
    defaultFormat: 'envrc' as 'envrc' | 'json' | 'yaml'
  })

  React.useEffect(() => {
    // Load settings from localStorage
    const savedSettings = localStorage.getItem('vaultSettings')
    if (savedSettings) {
      try {
        const parsed = JSON.parse(savedSettings)
        setSettings(prev => ({ ...prev, ...parsed }))
      } catch (error) {
        console.error('Failed to parse saved settings:', error)
      }
    }

    // Load Vault status
    loadVaultStatus()
  }, [])

  const loadVaultStatus = async () => {
    try {
      const response = await fetch('/api/v1/vault/status')
      const data = await response.json()
      setVaultStatus(data)
    } catch (error) {
      setVaultStatus({ error: 'Failed to load status' })
    }
  }

  const testConnection = async () => {
    setConnectionLoading(true)
    setTestMessage(null)
    
    try {
      const response = await fetch('/api/v1/health')
      if (response.ok) {
        setTestMessage('✅ Connection successful!')
        await loadVaultStatus()
      } else {
        setTestMessage('❌ Connection failed')
      }
    } catch (error) {
      setTestMessage('❌ Connection error')
    } finally {
      setConnectionLoading(false)
    }
    
    // Clear message after 3 seconds
    setTimeout(() => setTestMessage(null), 3000)
  }

  const saveSettings = () => {
    localStorage.setItem('vaultSettings', JSON.stringify(settings))
    
    // Show success message
    setTestMessage('✅ Settings saved!')
    setTimeout(() => setTestMessage(null), 2000)
  }

  const exportSettings = () => {
    const allSettings = {
      vault: { depth, reveal, includeValues },
      ui: settings
    }
    
    const dataStr = JSON.stringify(allSettings, null, 2)
    const dataBlob = new Blob([dataStr], { type: 'application/json' })
    const url = URL.createObjectURL(dataBlob)
    
    const link = document.createElement('a')
    link.href = url
    link.download = 'vault-settings.json'
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  const importSettings = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    if (!file) return

    const reader = new FileReader()
    reader.onload = (e) => {
      try {
        const imported = JSON.parse(e.target?.result as string)
        
        if (imported.vault) {
          if (typeof imported.vault.depth === 'number') dispatch(setDepth(imported.vault.depth))
          if (typeof imported.vault.reveal === 'boolean') dispatch(setReveal(imported.vault.reveal))
          if (typeof imported.vault.includeValues === 'boolean') dispatch(setIncludeValues(imported.vault.includeValues))
        }
        
        if (imported.ui) {
          setSettings(prev => ({ ...prev, ...imported.ui }))
        }
        
        setTestMessage('✅ Settings imported successfully!')
        setTimeout(() => setTestMessage(null), 3000)
      } catch (error) {
        setTestMessage('❌ Failed to import settings')
        setTimeout(() => setTestMessage(null), 3000)
      }
    }
    
    reader.readAsText(file)
    event.target.value = '' // Reset file input
  }

  const resetSettings = () => {
    if (confirm('Are you sure you want to reset all settings to defaults?')) {
      // Reset Redux state
      dispatch(setDepth(0))
      dispatch(setReveal(false))
      dispatch(setIncludeValues(false))
      
      // Reset local settings
      setSettings({
        censorPrefix: 2,
        censorSuffix: 2,
        autoRefresh: false,
        showBreadcrumbs: true,
        defaultFormat: 'envrc'
      })
      
      // Clear localStorage
      localStorage.removeItem('vaultSettings')
      
      setTestMessage('✅ Settings reset to defaults')
      setTimeout(() => setTestMessage(null), 3000)
    }
  }

  return (
    <div className="container-fluid">
      {/* Header */}
      <div className="row mb-4">
        <div className="col-12">
          <div className="d-flex justify-content-between align-items-center">
            <div>
              <h3 className="mb-1">Settings</h3>
              <p className="text-muted mb-0">
                Configure your Vault connection and application preferences
              </p>
            </div>
            {testMessage && (
              <div className="alert alert-info py-2 px-3 mb-0" role="alert">
                {testMessage}
              </div>
            )}
          </div>
        </div>
      </div>

      <div className="row g-4">
        {/* Vault Connection */}
        <div className="col-md-6">
          <div className="card">
            <div className="card-header">
              <h6 className="mb-0">
                <i className="bi bi-shield-lock me-1"></i>
                Vault Connection
              </h6>
            </div>
            <div className="card-body">
              {vaultStatus ? (
                <div className="mb-3">
                  {vaultStatus.error ? (
                    <div className="alert alert-danger">
                      <i className="bi bi-exclamation-triangle me-1"></i>
                      <strong>Connection Error:</strong><br />
                      {typeof vaultStatus.error === 'string' ? vaultStatus.error : 'Unknown error'}
                    </div>
                  ) : (
                    <div className="alert alert-success">
                      <i className="bi bi-check-circle me-1"></i>
                      <strong>Connected Successfully</strong>
                      
                      {vaultStatus.vault_address && (
                        <div className="mt-2">
                          <small className="d-block">
                            <strong>Address:</strong> {vaultStatus.vault_address}
                          </small>
                        </div>
                      )}
                      
                      {vaultStatus.token_info && (
                        <div className="mt-2">
                          <small className="d-block">
                            <strong>User:</strong> {vaultStatus.token_info.display_name || 'Anonymous'}
                          </small>
                          {vaultStatus.token_info.policies && vaultStatus.token_info.policies.length > 0 && (
                            <small className="d-block">
                              <strong>Policies:</strong> {vaultStatus.token_info.policies.join(', ')}
                            </small>
                          )}
                          {vaultStatus.token_info.ttl && (
                            <small className="d-block">
                              <strong>TTL:</strong> {Math.floor(vaultStatus.token_info.ttl / 3600)}h {Math.floor((vaultStatus.token_info.ttl % 3600) / 60)}m
                            </small>
                          )}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              ) : (
                <div className="alert alert-info">
                  <div className="d-flex align-items-center">
                    <div className="spinner-border spinner-border-sm me-2" role="status">
                      <span className="visually-hidden">Loading...</span>
                    </div>
                    Loading connection status...
                  </div>
                </div>
              )}

              <div className="d-flex gap-2">
                <button
                  className="btn btn-primary"
                  onClick={testConnection}
                  disabled={connectionLoading}
                >
                  {connectionLoading ? (
                    <>
                      <div className="spinner-border spinner-border-sm me-1" role="status">
                        <span className="visually-hidden">Loading...</span>
                      </div>
                      Testing...
                    </>
                  ) : (
                    <>
                      <i className="bi bi-arrow-clockwise me-1"></i>
                      Test Connection
                    </>
                  )}
                </button>
                
                <button className="btn btn-outline-secondary" onClick={loadVaultStatus}>
                  <i className="bi bi-info-circle me-1"></i>
                  Refresh Status
                </button>
              </div>

              <hr />

              <div className="row g-3">
                <div className="col-12">
                  <small className="text-muted">
                    <i className="bi bi-info-circle me-1"></i>
                    Connection settings are managed server-side via environment variables and configuration files.
                    In development mode, you can test different connections using the API.
                  </small>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Display Options */}
        <div className="col-md-6">
          <div className="card">
            <div className="card-header">
              <h6 className="mb-0">
                <i className="bi bi-eye me-1"></i>
                Display Options
              </h6>
            </div>
            <div className="card-body">
              <div className="row g-3">
                <div className="col-md-6">
                  <label className="form-label fw-semibold">Default Tree Depth</label>
                  <select
                    className="form-select"
                    value={depth}
                    onChange={(e) => dispatch(setDepth(Number(e.target.value)))}
                  >
                    <option value={0}>Unlimited</option>
                    <option value={1}>1 level</option>
                    <option value={2}>2 levels</option>
                    <option value={3}>3 levels</option>
                    <option value={5}>5 levels</option>
                  </select>
                </div>

                <div className="col-md-6">
                  <label className="form-label fw-semibold">Default Format</label>
                  <select
                    className="form-select"
                    value={settings.defaultFormat}
                    onChange={(e) => setSettings(prev => ({ 
                      ...prev, 
                      defaultFormat: e.target.value as any 
                    }))}
                  >
                    <option value="envrc">.envrc</option>
                    <option value="json">JSON</option>
                    <option value="yaml">YAML</option>
                  </select>
                </div>

                <div className="col-md-6">
                  <label className="form-label fw-semibold">Censor Prefix Length</label>
                  <input
                    type="number"
                    className="form-control"
                    min={0}
                    max={10}
                    value={settings.censorPrefix}
                    onChange={(e) => setSettings(prev => ({ 
                      ...prev, 
                      censorPrefix: Number(e.target.value) 
                    }))}
                  />
                </div>

                <div className="col-md-6">
                  <label className="form-label fw-semibold">Censor Suffix Length</label>
                  <input
                    type="number"
                    className="form-control"
                    min={0}
                    max={10}
                    value={settings.censorSuffix}
                    onChange={(e) => setSettings(prev => ({ 
                      ...prev, 
                      censorSuffix: Number(e.target.value) 
                    }))}
                  />
                </div>

                <div className="col-12">
                  <div className="form-check">
                    <input
                      className="form-check-input"
                      type="checkbox"
                      id="includeValues"
                      checked={includeValues}
                      onChange={(e) => dispatch(setIncludeValues(e.target.checked))}
                    />
                    <label className="form-check-label" htmlFor="includeValues">
                      Load secret values by default
                    </label>
                  </div>
                </div>

                <div className="col-12">
                  <div className="form-check">
                    <input
                      className="form-check-input"
                      type="checkbox"
                      id="reveal"
                      checked={reveal}
                      onChange={(e) => dispatch(setReveal(e.target.checked))}
                    />
                    <label className="form-check-label" htmlFor="reveal">
                      Reveal secret values by default
                    </label>
                  </div>
                </div>

                <div className="col-12">
                  <div className="form-check">
                    <input
                      className="form-check-input"
                      type="checkbox"
                      id="showBreadcrumbs"
                      checked={settings.showBreadcrumbs}
                      onChange={(e) => setSettings(prev => ({ 
                        ...prev, 
                        showBreadcrumbs: e.target.checked 
                      }))}
                    />
                    <label className="form-check-label" htmlFor="showBreadcrumbs">
                      Show path breadcrumbs
                    </label>
                  </div>
                </div>

                <div className="col-12">
                  <div className="form-check">
                    <input
                      className="form-check-input"
                      type="checkbox"
                      id="autoRefresh"
                      checked={settings.autoRefresh}
                      onChange={(e) => setSettings(prev => ({ 
                        ...prev, 
                        autoRefresh: e.target.checked 
                      }))}
                    />
                    <label className="form-check-label" htmlFor="autoRefresh">
                      Auto-refresh tree every 60 seconds
                    </label>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Export/Import */}
        <div className="col-12">
          <div className="card">
            <div className="card-header">
              <h6 className="mb-0">
                <i className="bi bi-gear me-1"></i>
                Settings Management
              </h6>
            </div>
            <div className="card-body">
              <div className="d-flex gap-2 flex-wrap">
                <button className="btn btn-success" onClick={saveSettings}>
                  <i className="bi bi-check me-1"></i>
                  Save Settings
                </button>
                
                <button className="btn btn-outline-primary" onClick={exportSettings}>
                  <i className="bi bi-download me-1"></i>
                  Export Settings
                </button>
                
                <label className="btn btn-outline-secondary">
                  <i className="bi bi-upload me-1"></i>
                  Import Settings
                  <input
                    type="file"
                    accept=".json"
                    onChange={importSettings}
                    className="d-none"
                  />
                </label>
                
                <button className="btn btn-outline-danger" onClick={resetSettings}>
                  <i className="bi bi-arrow-counterclockwise me-1"></i>
                  Reset to Defaults
                </button>
              </div>
              
              <hr className="my-3" />
              
              <small className="text-muted">
                <i className="bi bi-info-circle me-1"></i>
                Settings are automatically saved to your browser's local storage. 
                Use export/import to backup or share your configuration.
              </small>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Settings


