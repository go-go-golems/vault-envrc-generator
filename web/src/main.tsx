import React from 'react'
import { createRoot } from 'react-dom/client'
import { Provider } from 'react-redux'
import { store } from './store'
import { VaultTree } from './components/VaultTree'

function App() {
  return (
    <div className="container-fluid">
      <header className="border-bottom pb-2 mb-4">
        <h1 className="h2 mb-1 text-primary">Vault Envrc Generator</h1>
        <small className="text-muted">
          API health: <Health />
        </small>
      </header>
      <main>
        <VaultTree rootPath="secrets/" />
      </main>
    </div>
  )
}

function Health() {
  const [status, setStatus] = React.useState('checking...')
  const [vaultInfo, setVaultInfo] = React.useState<any>(null)
  
  React.useEffect(() => {
    fetch('/api/v1/health')
      .then((r) => r.json())
      .then((j) => setStatus(j.status ?? 'ok'))
      .catch(() => setStatus('unavailable'))
      
    fetch('/api/v1/vault/status')
      .then((r) => r.json())
      .then((j) => setVaultInfo(j))
      .catch(() => setVaultInfo({ error: true }))
  }, [])
  
  return (
    <div className="d-flex gap-3 align-items-center">
      <div>
        API: <code className={status === 'ok' ? 'text-success' : 'text-danger'}>{status}</code>
      </div>
      {vaultInfo && (
        <div className="d-flex gap-2 align-items-center">
          <span className="text-muted">|</span>
          <small className="text-muted">
            Vault: {vaultInfo.error ? (
              <span className="text-danger">disconnected</span>
            ) : (
              <span className="text-success">
                {vaultInfo.token_info?.display_name || 'connected'}
                {vaultInfo.token_info?.policies && (
                  <span className="text-muted"> ({vaultInfo.token_info.policies.join(', ')})</span>
                )}
              </span>
            )}
          </small>
        </div>
      )}
    </div>
  )
}

const el = document.getElementById('root')!
createRoot(el).render(
  <Provider store={store}>
    <App />
  </Provider>
)


