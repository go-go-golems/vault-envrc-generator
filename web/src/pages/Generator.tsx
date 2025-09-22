import React from 'react'
import { useSearchParams } from 'react-router-dom'

interface GenerateFormData {
  path: string
  format: 'envrc' | 'json' | 'yaml'
  prefix: string
  transformKeys: boolean
  sortKeys: boolean
  includeKeys: string[]
  excludeKeys: string[]
}

const Generator: React.FC = () => {
  const [searchParams] = useSearchParams()
  const [formData, setFormData] = React.useState<GenerateFormData>({
    path: searchParams.get('path') || 'secrets/',
    format: 'envrc',
    prefix: '',
    transformKeys: true,
    sortKeys: true,
    includeKeys: [],
    excludeKeys: []
  })
  
  const [generatedContent, setGeneratedContent] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const [batchConfig, setBatchConfig] = React.useState('')
  const [activeTab, setActiveTab] = React.useState<'single' | 'batch'>('single')

  const handleGenerate = async () => {
    setLoading(true)
    setError(null)
    setGeneratedContent(null)
    
    try {
      const payload = {
        path: formData.path,
        prefix: formData.prefix,
        transform_keys: formData.transformKeys,
        sort_keys: formData.sortKeys,
        include_keys: formData.includeKeys.filter(k => k.trim()),
        exclude_keys: formData.excludeKeys.filter(k => k.trim())
      }
      
      const response = await fetch(`/api/v1/generate/${formData.format}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      
      if (!response.ok) {
        throw new Error(`Generation failed: ${response.status} ${response.statusText}`)
      }
      
      const data = await response.json()
      setGeneratedContent(data.content || '')
      
    } catch (error: any) {
      setError(error?.message || 'Failed to generate content')
    } finally {
      setLoading(false)
    }
  }

  const handleBatchProcess = async () => {
    if (!batchConfig.trim()) {
      setError('Please provide batch configuration')
      return
    }
    
    setLoading(true)
    setError(null)
    setGeneratedContent(null)
    
    try {
      const response = await fetch('/api/v1/batch/process', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          config: batchConfig,
          dry_run: true
        })
      })
      
      if (!response.ok) {
        throw new Error(`Batch processing failed: ${response.status} ${response.statusText}`)
      }
      
      const data = await response.json()
      
      // Format batch results for display
      if (data.results) {
        const formatted = Object.entries(data.results)
          .map(([path, content]) => `# ${path}\n${content}`)
          .join('\n\n' + '='.repeat(50) + '\n\n')
        setGeneratedContent(formatted)
      } else {
        setGeneratedContent(JSON.stringify(data, null, 2))
      }
      
    } catch (error: any) {
      setError(error?.message || 'Failed to process batch configuration')
    } finally {
      setLoading(false)
    }
  }

  const copyToClipboard = async () => {
    if (generatedContent) {
      await navigator.clipboard.writeText(generatedContent)
    }
  }

  const downloadFile = () => {
    if (!generatedContent) return
    
    const extension = activeTab === 'batch' ? 'txt' : formData.format === 'envrc' ? 'envrc' : formData.format
    const filename = activeTab === 'batch' 
      ? 'batch-results.txt' 
      : `generated.${extension}`
    
    const blob = new Blob([generatedContent], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  return (
    <div className="container-fluid">
      {/* Header */}
      <div className="row mb-4">
        <div className="col-12">
          <div className="d-flex justify-content-between align-items-center">
            <div>
              <h3 className="mb-1">File Generator</h3>
              <p className="text-muted mb-0">
                Generate configuration files from your Vault secrets
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="row mb-4">
        <div className="col-12">
          <ul className="nav nav-tabs">
            <li className="nav-item">
              <button
                className={`nav-link ${activeTab === 'single' ? 'active' : ''}`}
                onClick={() => setActiveTab('single')}
              >
                <i className="bi bi-file-earmark-code me-1"></i>
                Single Path Generation
              </button>
            </li>
            <li className="nav-item">
              <button
                className={`nav-link ${activeTab === 'batch' ? 'active' : ''}`}
                onClick={() => setActiveTab('batch')}
              >
                <i className="bi bi-files me-1"></i>
                Batch Processing
              </button>
            </li>
          </ul>
        </div>
      </div>

      <div className="row">
        {/* Configuration Panel */}
        <div className="col-md-6">
          <div className="card h-100">
            <div className="card-header">
              <h6 className="mb-0">
                <i className="bi bi-gear me-1"></i>
                {activeTab === 'single' ? 'Generation Configuration' : 'Batch Configuration'}
              </h6>
            </div>
            <div className="card-body">
              {activeTab === 'single' ? (
                <div className="row g-3">
                  {/* Path */}
                  <div className="col-12">
                    <label className="form-label fw-semibold">Vault Path</label>
                    <input
                      type="text"
                      className="form-control"
                      value={formData.path}
                      onChange={(e) => setFormData(prev => ({ ...prev, path: e.target.value }))}
                      placeholder="e.g., secrets/app/production"
                    />
                  </div>

                  {/* Format */}
                  <div className="col-md-6">
                    <label className="form-label fw-semibold">Output Format</label>
                    <select
                      className="form-select"
                      value={formData.format}
                      onChange={(e) => setFormData(prev => ({ 
                        ...prev, 
                        format: e.target.value as GenerateFormData['format'] 
                      }))}
                    >
                      <option value="envrc">.envrc (Environment)</option>
                      <option value="json">JSON</option>
                      <option value="yaml">YAML</option>
                    </select>
                  </div>

                  {/* Prefix */}
                  <div className="col-md-6">
                    <label className="form-label fw-semibold">Variable Prefix</label>
                    <input
                      type="text"
                      className="form-control"
                      value={formData.prefix}
                      onChange={(e) => setFormData(prev => ({ ...prev, prefix: e.target.value }))}
                      placeholder="e.g., DB_"
                    />
                  </div>

                  {/* Options */}
                  <div className="col-12">
                    <label className="form-label fw-semibold">Options</label>
                    <div className="form-check">
                      <input
                        className="form-check-input"
                        type="checkbox"
                        id="transformKeys"
                        checked={formData.transformKeys}
                        onChange={(e) => setFormData(prev => ({ ...prev, transformKeys: e.target.checked }))}
                      />
                      <label className="form-check-label" htmlFor="transformKeys">
                        Transform keys to UPPER_CASE
                      </label>
                    </div>
                    <div className="form-check">
                      <input
                        className="form-check-input"
                        type="checkbox"
                        id="sortKeys"
                        checked={formData.sortKeys}
                        onChange={(e) => setFormData(prev => ({ ...prev, sortKeys: e.target.checked }))}
                      />
                      <label className="form-check-label" htmlFor="sortKeys">
                        Sort keys alphabetically
                      </label>
                    </div>
                  </div>

                  {/* Include/Exclude Keys */}
                  <div className="col-md-6">
                    <label className="form-label fw-semibold">Include Keys</label>
                    <textarea
                      className="form-control"
                      rows={3}
                      value={formData.includeKeys.join('\n')}
                      onChange={(e) => setFormData(prev => ({ 
                        ...prev, 
                        includeKeys: e.target.value.split('\n').map(k => k.trim()).filter(Boolean)
                      }))}
                      placeholder="One key per line (leave empty to include all)"
                    />
                  </div>

                  <div className="col-md-6">
                    <label className="form-label fw-semibold">Exclude Keys</label>
                    <textarea
                      className="form-control"
                      rows={3}
                      value={formData.excludeKeys.join('\n')}
                      onChange={(e) => setFormData(prev => ({ 
                        ...prev, 
                        excludeKeys: e.target.value.split('\n').map(k => k.trim()).filter(Boolean)
                      }))}
                      placeholder="One key per line"
                    />
                  </div>

                  {/* Generate Button */}
                  <div className="col-12">
                    <button
                      className="btn btn-primary w-100"
                      onClick={handleGenerate}
                      disabled={loading || !formData.path}
                    >
                      {loading ? (
                        <>
                          <div className="spinner-border spinner-border-sm me-2" role="status">
                            <span className="visually-hidden">Loading...</span>
                          </div>
                          Generating...
                        </>
                      ) : (
                        <>
                          <i className="bi bi-play-fill me-1"></i>
                          Generate {formData.format.toUpperCase()}
                        </>
                      )}
                    </button>
                  </div>
                </div>
              ) : (
                <div className="row g-3">
                  <div className="col-12">
                    <label className="form-label fw-semibold">Batch Configuration (YAML)</label>
                    <textarea
                      className="form-control font-monospace"
                      rows={15}
                      value={batchConfig}
                      onChange={(e) => setBatchConfig(e.target.value)}
                      placeholder={`# Example batch configuration:
outputs:
  - path: "secrets/app/database"
    output_path: "app-db.envrc"
    format: "envrc"
    prefix: "DB_"
    
  - path: "secrets/app/api"
    output_path: "app-api.json"
    format: "json"
    transform_keys: true`}
                    />
                  </div>
                  
                  <div className="col-12">
                    <button
                      className="btn btn-primary w-100"
                      onClick={handleBatchProcess}
                      disabled={loading || !batchConfig.trim()}
                    >
                      {loading ? (
                        <>
                          <div className="spinner-border spinner-border-sm me-2" role="status">
                            <span className="visually-hidden">Loading...</span>
                          </div>
                          Processing...
                        </>
                      ) : (
                        <>
                          <i className="bi bi-play-fill me-1"></i>
                          Process Batch (Dry Run)
                        </>
                      )}
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Output Panel */}
        <div className="col-md-6">
          <div className="card h-100">
            <div className="card-header d-flex justify-content-between align-items-center">
              <h6 className="mb-0">
                <i className="bi bi-file-text me-1"></i>
                Generated Output
              </h6>
              {generatedContent && (
                <div className="btn-group btn-group-sm">
                  <button className="btn btn-outline-primary" onClick={copyToClipboard}>
                    <i className="bi bi-clipboard"></i>
                  </button>
                  <button className="btn btn-outline-secondary" onClick={downloadFile}>
                    <i className="bi bi-download"></i>
                  </button>
                </div>
              )}
            </div>
            <div className="card-body p-0">
              {error && (
                <div className="alert alert-danger m-3">
                  <i className="bi bi-exclamation-triangle me-1"></i>
                  {error}
                </div>
              )}

              {generatedContent ? (
                <pre className="m-0 p-3 bg-light h-100" style={{ fontSize: '0.875rem', overflowY: 'auto' }}>
                  <code>{generatedContent}</code>
                </pre>
              ) : (
                <div className="d-flex align-items-center justify-content-center h-100 text-muted">
                  <div className="text-center">
                    <i className="bi bi-file-earmark-text" style={{ fontSize: '3rem' }}></i>
                    <p className="mt-3 mb-0">Generated content will appear here</p>
                    <small>Configure your settings and click generate</small>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export default Generator


