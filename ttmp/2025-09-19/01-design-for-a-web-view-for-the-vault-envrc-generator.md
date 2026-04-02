# Design for Web View of Vault Envrc Generator

## Overview

This document outlines the technical design for implementing a web-based interface for the `vault-envrc-generator` tool. The web view will provide an interactive way to explore Vault secrets trees, similar to the existing `tree` command, with additional capabilities for generating envrc files and managing Vault configurations through a modern web interface.

### Who this is for and how to get oriented (new joiner quickstart)

- Audience: engineers new to this repo who need to understand the design and how it maps to the existing code.
- Prerequisites: Go toolchain (see `go.mod`), pnpm (for web), access to a Vault instance and token.
- Start by skimming these docs and files:
  - Project intro and commands: `README.md`
  - Architecture overview: `pkg/doc/03-architecture-overview.md`
  - Batch/YAML reference: `pkg/doc/04-yaml-configuration-reference.md`
  - Seed guide: `pkg/doc/05-seed-configuration-guide.md`
  - Glazed/Vault middleware: `pkg/doc/06-vault-glazed-middleware.md`
  - Current CLI wiring: `cmd/vault-envrc-generator/main.go`
  - Existing commands: `cmds/tree.go`
  - Core packages: `pkg/vault/`, `pkg/listing/`, `pkg/envrc/`, `pkg/batch/`, `pkg/output/`, `pkg/vaultlayer/`, `pkg/glazed/`

Quick CLI sanity-check (assuming you have a Vault token):
```bash
export VAULT_ADDR="https://your-vault:8200"
export VAULT_TOKEN="..."
go build ./cmd/vault-envrc-generator
./vault-envrc-generator tree --path secrets/ --depth 1
```

## Architecture Overview

The solution will be implemented as a full-stack web application with the following components:

- **Backend**: Go HTTP server with REST API endpoints
- **Frontend**: React application with Redux for state management  
- **Integration**: Leverage existing vault client and tree functionality
- **Deployment**: Standalone web server that can be run locally or deployed

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   React + Redux │───▶│   Go HTTP API   │───▶│   Vault Client  │
│    Frontend     │    │    Backend      │    │   (existing)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
        │                       │                       │
        │                       │                       │
        ▼                       ▼                       ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Static Files  │    │  REST Endpoints │    │  HashiCorp      │
│   (HTML/CSS/JS) │    │  (JSON API)     │    │  Vault Server   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### Critical review and alignment with existing code

This spec now explicitly maps features and API routes to existing code in this repository to avoid re-implementing logic that already exists and to keep semantics consistent with the CLI:

- Tree discovery uses `pkg/listing.Walk` and `pkg/vault.NormalizeListPath`, not bespoke traversal.
- Secret retrieval and KV v2/v1 handling use `pkg/vault.Client` (`GetSecrets`, `ListSecrets`, `PutSecrets`), not raw Vault API calls.
- Token resolution and connection semantics use `pkg/vault.ResolveToken` and `pkg/vaultlayer` settings, same as the CLI.
- Generation endpoints reuse `pkg/envrc.Generator` (envrc/json/yaml), and batch endpoints reuse `pkg/batch.Processor`.
- Path templating for base paths and dynamic sections relies on `pkg/vault.BuildTemplateContext` and `pkg/vault.RenderTemplateString`.
- Output merge/overwrite semantics for JSON/YAML/envrc mirror `pkg/output.Write` and batch processor rules.

The UI’s “Vault Tree” should match the CLI’s conceptual model from `cmds/tree.go` while relying on `pkg/listing` to build a path inventory and then selectively materialize leaf values, with censoring identical to the CLI.

## Backend Design (Go)

### 1. Web Server Structure

Following the established patterns from go-go-mento, the web server will be implemented as a `serve` command:

```
cmd/vault-envrc-generator/
├── main.go                 # CLI entry point with serve command
└── cmds/
    ├── serve.go           # Web server command implementation
    ├── tree.go            # Existing tree command
    └── ...                # Other existing commands

pkg/
├── vault/                 # Existing vault client (reuse)
├── envrc/                 # Existing envrc generation (reuse)  
├── batch/                 # Existing batch processing (reuse)
├── webserver/             # New web server functionality
│   ├── server.go          # HTTP server setup and routing
│   ├── handlers/
│   │   ├── vault.go       # Vault tree/secrets API handlers
│   │   ├── config.go      # Configuration API handlers
│   │   ├── generate.go    # File generation API handlers
│   │   └── health.go      # Health check handlers
│   ├── middleware/
│   │   ├── cors.go        # CORS middleware
│   │   ├── logging.go     # Request logging middleware
│   │   └── recovery.go    # Panic recovery middleware
│   └── types/
│       ├── requests.go    # API request structures
│       ├── responses.go   # API response structures
│       └── config.go      # Web server configuration
└── vaultlayer/            # Existing vault layer (reuse)

web/                       # Frontend React application
├── package.json           # pnpm configuration with Vite
├── vite.config.ts         # Vite configuration
├── biome.json            # Biome linting configuration
├── src/
│   ├── components/        # React components
│   ├── store/            # Redux store and slices
│   ├── pages/            # Page components
│   ├── hooks/            # Custom React hooks
│   └── utils/            # Utility functions
└── dist/                 # Built assets (embedded in Go binary)
```

### 2. REST API Endpoints

#### Vault Connection & Health
```
GET  /api/v1/health                    # Server health check
POST /api/v1/vault/connect             # Test Vault connection (dev-mode only accepts token input)
GET  /api/v1/vault/status              # Current Vault connection status (address, auth ok, token TTL)
```

#### Vault Tree Exploration
```
GET  /api/v1/vault/tree?path={path}&depth={n}&include={none|metadata|values}&reveal={bool}&censor_prefix={n}&censor_suffix={n}
# Response: Structured tree similar to CLI `tree`, built from pkg/listing + selective materialization

GET  /api/v1/vault/secrets/{path}      # Get specific secret values (leaf only)
GET  /api/v1/vault/list/{path}         # List keys at path (non-recursive, directories end with '/')
```

#### File Generation
```
POST /api/v1/generate/envrc            # Generate .envrc content (uses pkg/envrc)
POST /api/v1/generate/json             # Generate JSON content (uses pkg/envrc)  
POST /api/v1/generate/yaml             # Generate YAML content (uses pkg/envrc)
POST /api/v1/batch/process             # Process batch configuration (uses pkg/batch)
```

#### Configuration Management
```
GET  /api/v1/config                    # Get current configuration
PUT  /api/v1/config                    # Update configuration
GET  /api/v1/templates                 # Get available templates
```

Notes on endpoint semantics and mapping to packages:
- Tree/list endpoints must call `listing.Walk()` for discovery, then optionally call `vault.Client.GetSecrets()` to materialize leaf values when `include=values`.
- Censoring is applied server-side with the same algorithm as CLI; extract `censorString` from `cmds/tree.go` into a shared helper (e.g., `pkg/webserver/util`), or reimplement identically.
- Generate endpoints wrap `envrc.NewGenerator(options).Generate(secrets)`, where `secrets` are fetched via `vault.Client.GetSecrets(path)`.
- Batch processing endpoint should be a thin adaptor around `batch.Processor.Process`, honoring `ContinueOnError`, `DryRun`, `SortKeys`, and merge/overwrite semantics.

### 3. Data Structures

```go
// API Request/Response types
type VaultTreeRequest struct {
    Path         string `json:"path"`
    Depth        int    `json:"depth"`
    RevealValues bool   `json:"reveal_values"`
    CensorPrefix int    `json:"censor_prefix"`
    CensorSuffix int    `json:"censor_suffix"`
}

type VaultTreeResponse struct {
    Tree      map[string]interface{} `json:"tree"`
    Path      string                 `json:"path"`
    Timestamp time.Time              `json:"timestamp"`
    Errors    []string               `json:"errors,omitempty"`
}

type VaultConnectionRequest struct {
    Address     string `json:"address"`
    Token       string `json:"token"`
    TokenSource string `json:"token_source"`
    TokenFile   string `json:"token_file"`
}

type GenerateRequest struct {
    Path          string            `json:"path"`
    Format        string            `json:"format"` // envrc, json, yaml
    Prefix        string            `json:"prefix"`
    TransformKeys bool              `json:"transform_keys"`
    IncludeKeys   []string          `json:"include_keys"`
    ExcludeKeys   []string          `json:"exclude_keys"`
    EnvMap        map[string]string `json:"env_map"`
}

type BatchProcessRequest struct {
    Config     string `json:"config"`     # YAML configuration
    DryRun     bool   `json:"dry_run"`
    OutputPath string `json:"output_path"`
}
```

Response schemas (high level):
- VaultTreeResponse.tree: nested map[string]any where directories map to child objects and leaf secrets map to either `{ "__secret__": { k: v } }` when materialized or `{ "__secret__": { k: censored } }` when censored.
- Secrets response: `{ path: string, secrets: map[string]any }`, no censoring (guarded by authorization and UI controls).
- Generate responses: `{ format: "envrc|json|yaml", content: string }`.

### 4. Serve Command Implementation

Following the pattern from go-go-mento's serve command, the web server will be integrated as a command:

```go
// cmds/serve.go
type ServeCommand struct {
    *gcmds.CommandDescription
}

type ServeSettings struct {
    Port         string `glazed.parameter:"port"`
    Host         string `glazed.parameter:"host"`
    CorsOrigins  string `glazed.parameter:"cors-origins"`
    DevMode      bool   `glazed.parameter:"dev-mode"`
}

func NewServeCommand() (*ServeCommand, error) {
    // Build parameter layers (server settings + vault layer)
    layer, err := glzcli.NewCommandSettingsLayer()
    if err != nil {
        return nil, err
    }
    
    cd := gcmds.NewCommandDescription(
        "serve",
        gcmds.WithShort("Start web server for Vault tree exploration"),
        gcmds.WithFlags(
            parameters.NewParameterDefinition("port", parameters.ParameterTypeString, 
                parameters.WithDefault("8080"), parameters.WithHelp("Server port")),
            parameters.NewParameterDefinition("host", parameters.ParameterTypeString, 
                parameters.WithDefault("localhost"), parameters.WithHelp("Server host")),
            parameters.NewParameterDefinition("cors-origins", parameters.ParameterTypeString, 
                parameters.WithDefault("*"), parameters.WithHelp("CORS allowed origins")),
            parameters.NewParameterDefinition("dev-mode", parameters.ParameterTypeBool, 
                parameters.WithDefault(false), parameters.WithHelp("Enable development mode")),
        ),
        gcmds.WithLayersList(layer),
    )
    
    // Add vault layer for connection parameters
    _, err = vaultlayer.AddVaultLayerToCommand(cd)
    if err != nil {
        return nil, err
    }
    
    return &ServeCommand{cd}, nil
}

func (c *ServeCommand) Run(ctx context.Context, parsed *glayers.ParsedLayers) error {
    s := &ServeSettings{}
    if err := parsed.InitializeStruct(glayers.DefaultSlug, s); err != nil {
        return err
    }
    
    vs, err := vaultlayer.GetVaultSettings(parsed)
    if err != nil {
        return err
    }
    
    // Initialize web server with embedded assets
    server := webserver.New(&webserver.Config{
        Host:        s.Host,
        Port:        s.Port,
        CorsOrigins: strings.Split(s.CorsOrigins, ","),
        DevMode:     s.DevMode,
        VaultSettings: vs,
    })
    
    return server.Start(ctx)
}
```

Refinement:
- Parameters should be provided via glazed layers (use `glzcli.NewCommandSettingsLayer()` and `vaultlayer.AddVaultLayerToCommand`), not custom flag parsing.
- In production, the server reads Vault address and token from env/config via `vaultlayer`; in dev mode only, `POST /vault/connect` may accept a token payload for testing.
- Follow SPA static serving and fallback behavior from go-go-mento’s `go/cmd/frontend/serve.go` (HEAD support, direct index fallback, debug logging).

### 5. Web Server Package Structure

```go
// pkg/webserver/server.go
type Server struct {
    config *Config
    router *mux.Router
}

//go:embed web/dist
var embeddedDist embed.FS

func New(config *Config) *Server {
    s := &Server{config: config}
    s.setupRoutes()
    return s
}

func (s *Server) setupRoutes() {
    s.router = mux.NewRouter()
    
    // API routes
    api := s.router.PathPrefix("/api/v1").Subrouter()
    api.Use(s.corsMiddleware, s.loggingMiddleware, s.recoveryMiddleware)
    
    // Vault endpoints
    vaultHandler := &handlers.VaultHandler{Config: s.config}
    api.HandleFunc("/health", handlers.HealthCheck).Methods("GET")
    api.HandleFunc("/vault/tree", vaultHandler.GetTree).Methods("GET")
    api.HandleFunc("/vault/secrets/{path:.*}", vaultHandler.GetSecrets).Methods("GET")
    api.HandleFunc("/generate/envrc", vaultHandler.GenerateEnvrc).Methods("POST")
    // ... other endpoints
    
    // Static file serving with SPA fallback (like go-go-mento)
    s.setupStaticRoutes()
}

func (s *Server) setupStaticRoutes() {
    sub, err := fs.Sub(embeddedDist, "web/dist")
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to create sub filesystem")
    }
    
    static := http.FileServer(http.FS(sub))
    s.router.PathPrefix("/").Methods("GET", "HEAD").Handler(
        http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // SPA fallback logic similar to go-go-mento
            // ... implementation details
        }))
}
```

### 6. Handler Implementation Pseudocode

```go
// pkg/webserver/handlers/vault.go
type VaultHandler struct {
    Config *webserver.Config
}

func (h *VaultHandler) GetTree(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    req, err := parseTreeRequest(r)
    if err != nil {
        writeErrorResponse(w, http.StatusBadRequest, err)
        return
    }
    
    // Create Vault client using existing pkg/vault
    client, err := h.createVaultClient(r.Context())
    if err != nil {
        writeErrorResponse(w, http.StatusInternalServerError, err)
        return
    }
    
    // Reuse existing tree walking logic from cmds/tree.go
    // Extract common functionality to pkg/vault or pkg/tree
    tree, err := h.walkVaultTree(client, req)
    if err != nil {
        writeErrorResponse(w, http.StatusInternalServerError, err)
        return
    }
    
    // Return JSON response
    writeJSONResponse(w, &types.VaultTreeResponse{
        Tree:      tree,
        Path:      req.Path,
        Timestamp: time.Now(),
    })
}

func (h *VaultHandler) GenerateEnvrc(w http.ResponseWriter, r *http.Request) {
    var req types.GenerateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeErrorResponse(w, http.StatusBadRequest, err)
        return
    }
    
    // Use existing pkg/envrc functionality
    generator := envrc.NewGenerator()
    content, err := generator.Generate(req.Path, &envrc.Options{
        Prefix:        req.Prefix,
        TransformKeys: req.TransformKeys,
        IncludeKeys:   req.IncludeKeys,
        ExcludeKeys:   req.ExcludeKeys,
    })
    if err != nil {
        writeErrorResponse(w, http.StatusInternalServerError, err)
        return
    }
    
    writeJSONResponse(w, &types.GenerateResponse{
        Content:   content,
        Format:    "envrc",
        Generated: time.Now(),
    })
}
```

### 7. Tree construction strategy (reuse, not reimplement)

- Discovery: `pkg/listing.Walk(client, NormalizeListPath(path), depth)` returns a sorted list of paths and an error slice. Use errors as annotations in the tree for transparency, mirroring `cmds/tree.go` behavior (e.g., `name__error__`).
- Materialization: For entries ending without `/`, call `client.GetSecrets(path)`; wrap values into a `__secret__` node. Use censoring unless `reveal=true`.
- Censoring: Extract and reuse the CLI’s `censorString` behavior with prefix/suffix controls; default 2/2 should match CLI defaults.
- Leaf vs directory: Use trailing slash convention from Vault list API when constructing nodes; normalize via `vault.NormalizeListPath`.
- Errors: For list/get failures, record `<node_name>__error__` with the error text; do not stop traversal unless critical (connection/token errors).

## Frontend Design (React + Redux)

### 1. Frontend Build System

Following the go-go-mento pattern, the frontend will use pnpm + Vite with Biome for linting:

```
web/
├── package.json              # pnpm configuration
├── vite.config.ts           # Vite build configuration
├── biome.json              # Biome linting and formatting
├── tsconfig.json           # TypeScript configuration
├── index.html              # Entry HTML template
├── src/
│   ├── components/
│   │   ├── common/
│   │   │   ├── Header.tsx         # App header with connection status
│   │   │   ├── Sidebar.tsx        # Navigation sidebar
│   │   │   ├── LoadingSpinner.tsx # Loading indicator
│   │   │   └── ErrorBoundary.tsx  # Error handling
│   │   ├── vault/
│   │   │   ├── VaultTree.tsx      # Main tree view component
│   │   │   ├── TreeNode.tsx       # Individual tree node
│   │   │   ├── SecretViewer.tsx   # Secret details viewer
│   │   │   └── PathBreadcrumb.tsx # Path navigation
│   │   ├── config/
│   │   │   ├── ConnectionForm.tsx # Vault connection setup
│   │   │   └── SettingsPanel.tsx  # App settings
│   │   └── generate/
│   │       ├── GenerateForm.tsx   # File generation form
│   │       ├── BatchConfig.tsx    # Batch processing config
│   │       └── OutputPreview.tsx  # Generated content preview
│   ├── store/
│   │   ├── index.ts              # Redux store setup
│   │   ├── slices/
│   │   │   ├── vaultSlice.ts     # Vault state management
│   │   │   ├── configSlice.ts    # Configuration state
│   │   │   └── uiSlice.ts        # UI state (loading, errors)
│   │   └── api/
│   │       └── vaultApi.ts       # RTK Query API definitions
│   ├── pages/
│   │   ├── Dashboard.tsx         # Main dashboard
│   │   ├── TreeExplorer.tsx      # Tree exploration page
│   │   ├── Generator.tsx         # File generation page
│   │   └── Settings.tsx          # Settings page
│   ├── hooks/
│   │   ├── useVaultConnection.ts # Connection management
│   │   └── useTreeNavigation.ts  # Tree navigation logic
│   ├── utils/
│   │   ├── formatting.ts         # Data formatting utilities
│   │   └── validation.ts         # Form validation
│   └── main.tsx                  # React app entry point
└── dist/                         # Build output (embedded in Go)
```

### 2. Build Configuration Files

#### package.json
```json
{
  "name": "vault-envrc-web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "packageManager": "pnpm@10.15.0",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "lint": "biome check src/",
    "lint:fix": "biome check --write src/",
    "format": "biome format --write src/"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "react-router-dom": "^6.28.0",
    "@reduxjs/toolkit": "^2.3.0",
    "react-redux": "^9.1.2",
    "react-query": "^3.39.3"
  },
  "devDependencies": {
    "@types/react": "^18.3.12",
    "@types/react-dom": "^18.3.1",
    "@vitejs/plugin-react": "^4.3.3",
    "@biomejs/biome": "^1.9.4",
    "typescript": "~5.6.2",
    "vite": "^5.4.10"
  }
}
```

#### vite.config.ts
```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ['react', 'react-dom'],
          redux: ['@reduxjs/toolkit', 'react-redux']
        }
      }
    }
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true
      }
    }
  }
})
```

#### biome.json
```json
{
  "$schema": "https://biomejs.dev/schemas/1.9.4/schema.json",
  "organizeImports": {
    "enabled": true
  },
  "linter": {
    "enabled": true,
    "rules": {
      "recommended": true,
      "complexity": {
        "noExcessiveCognitiveComplexity": "warn"
      },
      "style": {
        "noNonNullAssertion": "warn"
      }
    }
  },
  "formatter": {
    "enabled": true,
    "formatWithErrors": false,
    "indentStyle": "space",
    "indentWidth": 2,
    "lineWidth": 100
  },
  "javascript": {
    "formatter": {
      "semicolons": "always",
      "trailingCommas": "es5"
    }
  },
  "files": {
    "include": ["src/**/*", "*.ts", "*.tsx", "*.js", "*.jsx"],
    "ignore": ["dist/**/*", "node_modules/**/*"]
  }
}
```

### 3. Build Integration with Go

Following the go-go-mento pattern, create a build script for web assets:

```
cmd/vault-envrc-generator/
├── build-web/
│   └── main.go              # Dagger build script (similar to go-go-mento)
└── cmds/
    └── serve.go             # Embed web/dist assets
```

#### Build Script Structure
```go
// cmd/vault-envrc-generator/build-web/main.go
//go:generate go run . 

package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "path/filepath"
    
    "dagger.io/dagger"
)

func main() {
    pnpmVersion := os.Getenv("WEB_PNPM_VERSION")
    if pnpmVersion == "" {
        pnpmVersion = "10.15.0" // keep in sync with web/package.json
    }
    
    ctx := context.Background()
    client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
    if err != nil {
        log.Fatalf("connect dagger: %v", err)
    }
    defer func() { _ = client.Close() }()
    
    // Build web assets and export to cmds/serve/dist/
    // Similar to go-go-mento implementation
    // ...
}
```

#### Embedded Assets in Serve Command
```go
// cmds/serve.go
//go:generate go run ../build-web

package cmds

import (
    "embed"
    "io/fs"
    // ...
)

//go:embed dist
var embeddedDist embed.FS

func (c *ServeCommand) setupStaticRoutes() {
    sub, err := fs.Sub(embeddedDist, "dist")
    if err != nil {
        return err
    }
    
    // Setup static file serving with SPA fallback
    // Similar to go-go-mento pattern
    // ...
}
```

### 2. Redux State Structure

```typescript
interface RootState {
  vault: {
    connection: {
      status: 'disconnected' | 'connecting' | 'connected' | 'error';
      address: string;
      tokenSource: string;
      error?: string;
    };
    tree: {
      currentPath: string;
      treeData: Record<string, any>;
      loading: boolean;
      error?: string;
      revealValues: boolean;
      expandedNodes: Set<string>;
    };
    secrets: {
      currentSecret: Record<string, any>;
      loading: boolean;
      error?: string;
    };
  };
  config: {
    settings: {
      censorPrefix: number;
      censorSuffix: number;
      defaultDepth: number;
      autoRefresh: boolean;
    };
    templates: Array<{
      name: string;
      format: string;
      config: any;
    }>;
  };
  ui: {
    sidebarOpen: boolean;
    currentPage: string;
    notifications: Array<{
      id: string;
      type: 'success' | 'error' | 'warning' | 'info';
      message: string;
    }>;
  };
}
```

### 3. Key React Components

#### Main Tree Explorer Component
```tsx
const VaultTree: React.FC = () => {
  const dispatch = useAppDispatch();
  const { treeData, currentPath, loading, revealValues } = useAppSelector(state => state.vault.tree);
  
  const handleNodeExpand = useCallback((path: string) => {
    dispatch(fetchVaultTree({ path, depth: 1 }));
  }, [dispatch]);
  
  const handlePathChange = useCallback((newPath: string) => {
    dispatch(setCurrentPath(newPath));
    dispatch(fetchVaultTree({ path: newPath, depth: 2 }));
  }, [dispatch]);
  
  return (
    <div className="vault-tree">
      <PathBreadcrumb path={currentPath} onPathChange={handlePathChange} />
      <div className="tree-controls">
        <ToggleSwitch 
          label="Reveal Values" 
          checked={revealValues}
          onChange={(checked) => dispatch(setRevealValues(checked))}
        />
        <RefreshButton onClick={() => dispatch(refreshTree())} />
      </div>
      {loading ? (
        <LoadingSpinner />
      ) : (
        <TreeView data={treeData} onNodeExpand={handleNodeExpand} />
      )}
    </div>
  );
};
```

#### Tree Node Component
```tsx
interface TreeNodeProps {
  name: string;
  data: any;
  path: string;
  level: number;
  onExpand: (path: string) => void;
  onSelect: (path: string) => void;
}

const TreeNode: React.FC<TreeNodeProps> = ({ name, data, path, level, onExpand, onSelect }) => {
  const [expanded, setExpanded] = useState(false);
  const isFolder = typeof data === 'object' && !data.__secret__;
  const hasError = name.endsWith('__error__');
  
  const handleToggle = () => {
    if (isFolder) {
      setExpanded(!expanded);
      if (!expanded) {
        onExpand(path);
      }
    } else {
      onSelect(path);
    }
  };
  
  return (
    <div className={`tree-node level-${level} ${hasError ? 'error' : ''}`}>
      <div className="node-header" onClick={handleToggle}>
        <span className="node-icon">
          {isFolder ? (expanded ? '📂' : '📁') : '🔑'}
        </span>
        <span className="node-name">{name}</span>
        {hasError && <span className="error-indicator">⚠️</span>}
      </div>
      {expanded && isFolder && (
        <div className="node-children">
          {Object.entries(data).map(([childName, childData]) => (
            <TreeNode
              key={childName}
              name={childName}
              data={childData}
              path={`${path}/${childName}`}
              level={level + 1}
              onExpand={onExpand}
              onSelect={onSelect}
            />
          ))}
        </div>
      )}
    </div>
  );
};
```

## Page Layouts (ASCII Mockups)

### 1. Main Dashboard Layout
```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Vault Envrc Generator Web                    🟢 Connected: vault.company.com    │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ┌─────────────┐ │                                                               │
│ │ 🏠 Dashboard│ │                     Welcome to Vault Web UI                   │
│ │ 🌳 Explorer │ │                                                               │
│ │ ⚙️  Generator│ │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐│
│ │ 🔧 Settings │ │  │   Quick Stats   │  │  Recent Paths   │  │   Quick Actions ││
│ │             │ │  │                 │  │                 │  │                 ││
│ │             │ │  │ 🔑 42 Secrets   │  │ secrets/app/db  │  │ 🌳 Browse Tree  ││
│ │             │ │  │ 📁 8 Folders    │  │ secrets/api/key │  │ ⚡ Generate     ││
│ │             │ │  │ ⏱️  Last: 2min   │  │ secrets/config  │  │ 🔄 Batch Run    ││
│ │             │ │  └─────────────────┘  └─────────────────┘  └─────────────────┘│
│ │             │ │                                                               │
│ └─────────────┘ │                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 2. Tree Explorer Layout
```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Vault Tree Explorer                          🟢 Connected | 🔄 Auto-refresh    │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ┌─────────────┐ │ Path: secrets/app/production                    [ Reveal ] [⚙️]│
│ │ 🏠 Dashboard│ ├─────────────────────────────────────────────────────────────────┤
│ │ 🌳 Explorer │ │ secrets > app > production                                    │
│ │ ⚙️  Generator│ │                                                               │
│ │ 🔧 Settings │ │ ┌─ 📁 database/                                               │
│ │             │ │ │  ├─ 🔑 primary     { host: "db****.com", port: "54**" }     │
│ │             │ │ │  ├─ 🔑 replica      { host: "db****.com", port: "54**" }     │
│ │             │ │ │  └─ 🔑 credentials  { username: "ap***", password: "***rd" } │
│ │             │ │ ├─ 📁 cache/                                                   │
│ │             │ │ │  └─ 🔑 redis        { url: "re****://...", timeout: "30" }   │
│ │             │ │ ├─ 📁 external-apis/                                          │
│ │             │ │ │  ├─ 🔑 stripe       { key: "sk***", webhook: "wh***" }       │
│ │             │ │ │  └─ 🔑 sendgrid     { api_key: "SG***" }                     │
│ │             │ │ └─ 🔑 app-config     { debug: "false", log_level: "info" }     │
│ └─────────────┘ │                                                               │
│                 │ Selected: secrets/app/production/database/primary             │
│                 │ ┌─────────────────────────────────────────────────────────────┤
│                 │ │ host: "database-primary.company.com"                       │
│                 │ │ port: "5432"                                               │
│                 │ │ database: "app_production"                                 │
│                 │ │ username: "app_user"                                       │
│                 │ │ password: "super_secret_password"                          │
│                 │ │                                          [📋 Copy] [⬇️ Export]│
│                 │ └─────────────────────────────────────────────────────────────┤
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 3. File Generator Layout
```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ File Generator                                                                  │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ┌─────────────┐ │ ┌─ Single Path Generation ──────────────────────────────────┐  │
│ │ 🏠 Dashboard│ │ │                                                           │  │
│ │ 🌳 Explorer │ │ │ Vault Path: [secrets/app/database/primary        ] [📁]   │  │
│ │ ⚙️  Generator│ │ │                                                           │  │
│ │ 🔧 Settings │ │ │ Output Format: [▼ .envrc    ]  Prefix: [DB_     ]         │  │
│ │             │ │ │                                                           │  │
│ │             │ │ │ ☑️ Transform Keys   ☑️ Sort Keys   ☐ Include Comments      │  │
│ │             │ │ │                                                           │  │
│ │             │ │ │ Include Keys: [host,port,database,username,password]      │  │
│ │             │ │ │ Exclude Keys: [                                      ]    │  │
│ │             │ │ │                                                           │  │
│ │             │ │ │                                        [Generate Preview] │  │
│ │             │ │ └───────────────────────────────────────────────────────────┘  │
│ │             │ │                                                               │
│ │             │ │ ┌─ Generated Output Preview ────────────────────────────────┐  │
│ │             │ │ │ # Generated from secrets/app/database/primary             │  │
│ │             │ │ │ export DB_HOST="database-primary.company.com"             │  │
│ │             │ │ │ export DB_PORT="5432"                                     │  │
│ │             │ │ │ export DB_DATABASE="app_production"                       │  │
│ │             │ │ │ export DB_USERNAME="app_user"                             │  │
│ │             │ │ │ export DB_PASSWORD="super_secret_password"                │  │
│ │             │ │ │                                                           │  │
│ │             │ │ │                           [📋 Copy] [💾 Download] [📧 Send]│  │
│ │             │ │ └───────────────────────────────────────────────────────────┘  │
│ └─────────────┘ │                                                               │
│                 │ ┌─ Batch Processing ─────────────────────────────────────────┐  │
│                 │ │                                                           │  │
│                 │ │ Configuration File: [Choose File] or [📝 Edit Inline]     │  │
│                 │ │                                                           │  │
│                 │ │ ☑️ Dry Run   ☐ Continue on Error   Output: [./output/]    │  │
│                 │ │                                                           │  │
│                 │ │                                        [Process Batch]   │  │
│                 │ │                                                           │  │
│                 │ └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

### 4. Settings Page Layout
```
┌─────────────────────────────────────────────────────────────────────────────────┐
│ Settings                                                                        │
├─────────────────────────────────────────────────────────────────────────────────┤
│ ┌─────────────┐ │ ┌─ Vault Connection ─────────────────────────────────────────┐  │
│ │ 🏠 Dashboard│ │ │                                                           │  │
│ │ 🌳 Explorer │ │ │ Vault Address: [https://vault.company.com:8200      ]     │  │
│ │ ⚙️  Generator│ │ │                                                           │  │
│ │ 🔧 Settings │ │ │ Token Source:  [▼ Environment Variable]                   │  │
│ │             │ │ │ Token:         [••••••••••••••••••••••••••••••••••]       │  │
│ │             │ │ │ Token File:    [~/.vault-token                      ]     │  │
│ │             │ │ │                                                           │  │
│ │             │ │ │ Status: 🟢 Connected (Last check: 30 seconds ago)         │  │
│ │             │ │ │                                        [Test Connection] │  │
│ │             │ │ └───────────────────────────────────────────────────────────┘  │
│ │             │ │                                                               │
│ │             │ │ ┌─ Display Options ──────────────────────────────────────────┐  │
│ │             │ │ │                                                           │  │
│ │             │ │ │ Default Tree Depth:    [2        ] (0 = unlimited)       │  │
│ │             │ │ │ Censor Prefix Length:  [2        ] characters             │  │
│ │             │ │ │ Censor Suffix Length:  [2        ] characters             │  │
│ │             │ │ │                                                           │  │
│ │             │ │ │ ☑️ Auto-refresh tree every 60 seconds                     │  │
│ │             │ │ │ ☑️ Show path breadcrumbs                                  │  │
│ │             │ │ │ ☐ Reveal values by default                                │  │
│ │             │ │ │                                                           │  │
│ │             │ │ └───────────────────────────────────────────────────────────┘  │
│ │             │ │                                                               │
│ │             │ │ ┌─ Export/Import ────────────────────────────────────────────┐  │
│ │             │ │ │                                                           │  │
│ │             │ │ │ [Export Settings]  [Import Settings]  [Reset to Defaults] │  │
│ │             │ │ │                                                           │  │
│ │             │ │ └───────────────────────────────────────────────────────────┘  │
│ └─────────────┘ │                                                               │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: Backend Foundation (Week 1-2)
1. **Project Structure Setup**
   - [ ] Create `pkg/webserver/` package structure
   - [ ] Add serve command to existing `cmds/` directory
   - [ ] Set up Go modules for additional dependencies (Gorilla Mux)
   - [ ] Create basic HTTP server with health endpoint

2. **Vault Integration & Code Reuse**
   - [ ] Extract common tree walking logic from `cmds/tree.go` to `pkg/vault/tree.go`
   - [ ] Create web handlers that reuse existing `pkg/vault` client
   - [ ] Implement tree walking endpoint (`/api/v1/vault/tree`)
   - [ ] Add secret retrieval endpoint (`/api/v1/vault/secrets/{path}`)
   - [ ] Add connection testing endpoint using existing vaultlayer

3. **Serve Command Integration**
   - [ ] Implement ServeCommand following glazed framework patterns
   - [ ] Integrate with existing vaultlayer for configuration
   - [ ] Add serve-specific parameters (port, host, cors-origins)
   - [ ] Set up embedded asset serving with SPA fallback
   - [ ] Extract `censorString` into shared util and reuse for API responses

### Phase 2: Core API Endpoints (Week 2-3)
1. **File Generation Endpoints**
   - [ ] Implement envrc generation endpoint (reuse existing `pkg/envrc` logic)
   - [ ] Add JSON/YAML generation endpoints
   - [ ] Create batch processing endpoint using existing `pkg/batch`
   - [ ] Align output merge/overwrite semantics with `pkg/output.Write`

2. **Configuration Management**
   - [ ] Settings persistence (file-based or in-memory)
   - [ ] Template management for common configurations
   - [ ] Vault connection state management

3. **Error Handling & Validation**
   - [ ] Comprehensive error responses with proper HTTP status codes
   - [ ] Input validation for all endpoints
   - [ ] Rate limiting and basic security measures
   - [ ] Standardize error payloads: `{ code, message, details? }`

### Phase 3: Frontend Foundation (Week 3-4)
1. **Frontend Build System Setup**
   - [ ] Create `web/` directory with pnpm + Vite configuration
   - [ ] Set up Biome for linting and formatting
   - [ ] Configure TypeScript and React with Vite
   - [ ] Create Dagger build script following go-go-mento pattern
   - [ ] Set up development proxy for API calls

2. **React Application Structure**
   - [ ] Create React app with TypeScript in `web/src/`
   - [ ] Set up Redux Toolkit with RTK Query for API calls
   - [ ] Configure routing with React Router
   - [ ] Implement embedded asset integration with serve command

3. **Core Components & State**
   - [ ] Create basic layout components (Header, Sidebar, Main)
   - [ ] Implement connection status indicator
   - [ ] Build loading states and error boundaries
   - [ ] Define Redux slices for vault, config, and UI state
   - [ ] Add optimistic UI for tree expand/collapse and lazy-loading

### Phase 4: Tree Explorer Implementation (Week 4-5)
1. **Tree Visualization**
   - [ ] Create TreeNode component with expand/collapse functionality
   - [ ] Implement PathBreadcrumb navigation
   - [ ] Add search/filter functionality for large trees
   - [ ] Create SecretViewer for displaying secret details
   - [ ] Add per-node error badges and tooltips sourced from API `__error__` fields

2. **Interactive Features**
   - [ ] Click-to-expand tree nodes
   - [ ] Path navigation and history
   - [ ] Copy-to-clipboard functionality
   - [ ] Value censoring toggle
   - [ ] Per-key copy and bulk copy in SecretViewer
   - [ ] Download generated file directly from UI (envrc/json/yaml)

3. **Performance Optimization**
   - [ ] Lazy loading for deep tree structures
   - [ ] Virtualization for large lists
   - [ ] Caching strategies for tree data
   - [ ] Debounce search and path typing; prefetch likely siblings

### Phase 5: File Generation Interface (Week 5-6)
1. **Generation Forms**
   - [ ] Single path generation form with live preview
   - [ ] Batch configuration editor (YAML editor with syntax highlighting)
   - [ ] Template selection and customization

2. **Output Handling**
   - [ ] Live preview of generated files
   - [ ] Download functionality
   - [ ] Copy to clipboard
   - [ ] Email/share functionality (optional)
   - [ ] Respect sort-keys and template file options; parity with CLI

3. **Advanced Features**
   - [ ] Drag-and-drop for batch configuration files
   - [ ] Configuration validation and error highlighting
   - [ ] Generation history and favorites

### Phase 6: Settings & Polish (Week 6-7)
1. **Settings Interface**
   - [ ] Vault connection configuration form
   - [ ] Display preferences (censoring, tree depth, etc.)
   - [ ] Theme selection (light/dark mode)
   - [ ] Export/import settings

2. **User Experience**
   - [ ] Responsive design for mobile/tablet
   - [ ] Keyboard shortcuts for power users
   - [ ] Tooltips and help text
   - [ ] Accessibility improvements (ARIA labels, keyboard navigation)

3. **Production Ready**
   - [ ] Build optimization and asset bundling
   - [ ] Docker containerization
   - [ ] Documentation and deployment guides
   - [ ] Security hardening (CSP headers, etc.)

### Phase 7: Testing & Documentation (Week 7-8)
1. **Testing**
   - [ ] Unit tests for Go handlers and utilities
   - [ ] React component testing with React Testing Library
   - [ ] Integration tests for API endpoints
   - [ ] End-to-end testing with Cypress or Playwright
   - [ ] Golden tests for envrc/json/yaml outputs against known fixtures

2. **Documentation**
   - [ ] API documentation (OpenAPI/Swagger)
   - [ ] User guide with screenshots
   - [ ] Development setup instructions
   - [ ] Deployment documentation
   - [ ] Link cross-references to existing docs (see below)

## Technology Stack

### Backend
- **Language**: Go 1.24+
- **HTTP Framework**: Gorilla Mux (consistent with go-go-mento)
- **Command Framework**: Glazed framework (already in use)
- **Logging**: `github.com/rs/zerolog` (already in use)
- **Configuration**: Existing vaultlayer and glazed parameter layers
- **Asset Embedding**: Go embed for static files

### Frontend
- **Framework**: React 18+ with TypeScript
- **Build Tool**: Vite (fast, modern)
- **Package Manager**: pnpm (consistent with go-go-mento)
- **State Management**: Redux Toolkit + RTK Query
- **Routing**: React Router v6
- **Linting**: Biome (fast, all-in-one)
- **UI Components**: Headless UI or similar lightweight library

### Build System
- **Build Orchestration**: Dagger (consistent with go-go-mento)
- **Asset Pipeline**: Vite build → Go embed
- **Development**: Vite dev server with API proxy
- **Production**: Single Go binary with embedded assets

### Development Tools
- **Code Quality**: Biome for frontend, existing Go linters
- **Testing**: Vitest for frontend, Go testing package for backend
- **API Documentation**: Generate from Go handlers
- **Development Workflow**: Hot reload for frontend, Go air for backend

## Security Considerations

1. **Vault Token Handling**
   - Never store tokens in localStorage
   - Prefer server-side env/config via `vaultlayer`; in dev mode only, allow POSTing a token
   - If a token is accepted from the browser in dev mode, keep it server-side only; do not echo back
   - Expose minimal status in `/vault/status` (no token contents), and warn on expiry

2. **API Security**
   - CORS configuration for allowed origins
   - Rate limiting on sensitive endpoints
   - Input validation and sanitization
   - HTTPS enforcement in production
   - Never log secret values; use structured logging with redaction and contexts

3. **Content Security**
   - CSP headers to prevent XSS
   - Sanitize displayed secret values
   - Secure handling of generated files
   - Audit logging for secret access
   - Censor by default; require explicit reveal toggle per session

## Deployment Options

1. **Local Development**
   - Single binary with embedded static assets (like go-go-mento)
   - Development mode: `vault-envrc-generator serve --dev-mode`
   - Hot reloading: Vite dev server + Go air for backend
   - Configuration via existing vaultlayer (env vars, flags, config files)

2. **Production Deployment**
   - Single Go binary with embedded web assets
   - Standard deployment: `vault-envrc-generator serve --port 8080`
   - Docker container with multi-stage build
   - Existing configuration patterns (Vault address, token sources)

3. **Build Process**
   - Development: `pnpm dev` in web/ + `go run cmd/vault-envrc-generator serve --dev-mode`
   - Production build: `go generate ./cmds/serve` (runs Dagger build) + `go build`
   - CI/CD: Dagger handles web build, Go handles binary compilation

## Future Enhancements

1. **Advanced Features**
   - Real-time collaboration (WebSocket for shared sessions)
   - Secret versioning and history visualization
   - Audit trail and access logging
   - Integration with CI/CD pipelines

2. **Enterprise Features**
   - RBAC integration with Vault policies
   - LDAP/SAML authentication
   - Multi-tenant support
   - Advanced reporting and analytics

3. **Developer Experience**
   - VS Code extension integration
   - CLI tool for headless operations
   - API client libraries
   - Webhook notifications

This comprehensive design provides a solid foundation for implementing a modern, user-friendly web interface for the vault-envrc-generator tool while leveraging the existing robust backend functionality.

## Mapping to existing packages and commands

- Vault client and path utilities: `pkg/vault/{client.go,path.go,token_loader.go,templates.go,context.go}`
- Listing (discovery): `pkg/listing/{walker.go,types.go}`
- Format generation: `pkg/envrc/generator.go`
- Batch processing: `pkg/batch/{types.go,processor.go}`
- Output merge/append: `pkg/output/writer.go`
- Glazed + Vault parameter layers: `pkg/glazed/middleware.go`, `pkg/vaultlayer/layer.go`
- CLI tree reference (behavioral parity): `cmds/tree.go`

Key symbols/functions to search in the codebase:
- `vault.NewClient`, `(*vault.Client).GetSecrets`, `(*vault.Client).ListSecrets`
- `vault.ResolveToken`, `vault.TokenSource`, `vault.BuildTemplateContext`, `vault.RenderTemplateString`
- `listing.Walk`, `vault.NormalizeListPath`
- `envrc.NewGenerator`, `(*envrc.Generator).Generate`
- `batch.Processor.Process`
- `output.Write`

## Feature descriptions (user-centric)

- Tree Explorer: Explore secrets with directory semantics matching Vault’s list API. View structure without values (metadata) or reveal with censoring/reveal toggle. Errors appear inline per node.
- Secret Viewer: Inspect per-leaf secret with per-key copy and bulk copy. Default censored view with option to reveal. Export secret as envrc/json/yaml.
- Single-Path Generate: Choose a Vault path and generate envrc/json/yaml using the same include/exclude/transform/prefix rules as the CLI.
- Batch Processing: Upload or paste a YAML batch config and run it through the backend batch processor with dry-run, continue-on-error, sort-keys and overwrite controls.
- Settings: Configure Vault address and token source (production via env/config; dev mode can accept token input). Control tree depth, censoring prefix/suffix, and auto-refresh.

## Error model

- HTTP status codes: 400 (validation), 401 (invalid token), 403 (insufficient Vault policy), 404 (path not found), 422 (batch config invalid), 500 (unexpected).
- Error payload shape: `{ code: string, message: string, details?: any }`.
- Tree/list errors: returned as per-node `__error__` properties without failing the entire response, matching CLI behavior.

## Documentation links (internal)

- Project README: `README.md` (see Command Reference and Quick Start sections)
- Architecture overview: `pkg/doc/03-architecture-overview.md`
- YAML configuration reference (batch): `pkg/doc/04-yaml-configuration-reference.md`
- Seed configuration guide: `pkg/doc/05-seed-configuration-guide.md`
- Glazed/Vault middleware: `pkg/doc/06-vault-glazed-middleware.md`
- Analyze command reference and env preview: `pkg/doc/06-analyze-env-preview.md`, `pkg/doc/07-analyze-command-reference.md`

## Open questions and decisions

- Should `GET /vault/tree` materialize values by default at depth=1 for leaves, or only on explicit `include=values`? Current spec defaults to `include=metadata` for safety.
- In production, do we support per-user Vault tokens (SSO/OIDC flow) or a single service token with policy scoping? Spec assumes server-side token from env/config; future SSO integration possible.
- Do we expose secret version metadata (KV v2) in the tree? Could be added as optional `include=metadata` enrichment.
- Do we persist UI settings server-side per session, or client-side only? Spec assumes client-side with reasonable defaults.

## Non-goals (initial version)

- No write/delete Vault operations from the web UI (read-only exploration and generation only).
- No SSO/OIDC login flow yet (future enhancement).
- No multi-tenant isolation beyond the configured Vault token/policies.

## Observability

- Structured logging with `zerolog`; never log token or values. Include request IDs and Vault path prefixes (censored) for correlation. Provide minimal metrics (request counts, latencies) to logs initially; Prometheus integration later if needed.
