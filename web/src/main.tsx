import React from 'react'
import { createRoot } from 'react-dom/client'
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
  React.useEffect(() => {
    fetch('/api/v1/health')
      .then((r) => r.json())
      .then((j) => setStatus(j.status ?? 'ok'))
      .catch(() => setStatus('unavailable'))
  }, [])
  return <code className={status === 'ok' ? 'text-success' : 'text-danger'}>{status}</code>
}

const el = document.getElementById('root')!
createRoot(el).render(<App />)


