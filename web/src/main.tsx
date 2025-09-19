import React from 'react'
import { createRoot } from 'react-dom/client'

function App() {
  return (
    <div style={{ padding: 16 }}>
      <h1>Vault Envrc Generator Web</h1>
      <p>
        API health: <Health />
      </p>
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
  return <code>{status}</code>
}

const el = document.getElementById('root')!
createRoot(el).render(<App />)


