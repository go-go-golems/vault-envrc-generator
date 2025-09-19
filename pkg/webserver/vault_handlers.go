package webserver

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/rs/zerolog/log"

    "github.com/go-go-golems/vault-envrc-generator/pkg/listing"
    "github.com/go-go-golems/vault-envrc-generator/pkg/vault"
    "github.com/go-go-golems/vault-envrc-generator/pkg/envrc"
    "github.com/go-go-golems/vault-envrc-generator/pkg/batch"
    "gopkg.in/yaml.v3"
)

type errorResponse struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// GET /api/v1/vault/list/{path}
func (s *Server) handleVaultList(w http.ResponseWriter, r *http.Request) {
	if s.config == nil || s.config.Vault == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "server_config", Message: "vault settings missing"})
		return
	}

	// path after /api/v1/vault/list/
	prefix := "/api/v1/vault/list/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "invalid list path"})
		return
	}
	listPath := strings.TrimPrefix(r.URL.Path, prefix)
	if listPath == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "missing path"})
		return
	}

    // depth parameter accepted for forward-compatibility; unused in metadata list
    // _ = r.URL.Query().Get("depth")

	// Resolve token and client (auto token resolution consistent with CLI)
	token, err := vault.ResolveToken(r.Context(), s.config.Vault.VaultToken, vault.TokenSource(s.config.Vault.VaultTokenSource), s.config.Vault.VaultTokenFile, false)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "vault_token", Message: err.Error()})
		return
	}
	client, err := vault.NewClient(s.config.Vault.VaultAddr, token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_client", Message: err.Error()})
		return
	}

    normalized := vault.NormalizeListPath(listPath)
    keys, err := client.ListSecrets(normalized)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_list", Message: err.Error()})
        return
    }
    resp := map[string]interface{}{
        "path":      normalized,
        "timestamp": time.Now(),
        "keys":      keys,
    }
    writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/vault/tree?path=...&depth=...
func (s *Server) handleVaultTree(w http.ResponseWriter, r *http.Request) {
	if s.config == nil || s.config.Vault == nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "server_config", Message: "vault settings missing"})
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "missing path"})
		return
	}
    depth := 1
	if d := r.URL.Query().Get("depth"); d != "" {
		if n, err := strconv.Atoi(d); err == nil {
			depth = n
		}
	}
    include := r.URL.Query().Get("include") // metadata|values
    reveal := r.URL.Query().Get("reveal") == "true"
    pre := 2
    suf := 2
    if v := r.URL.Query().Get("censor_prefix"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { pre = n }
    }
    if v := r.URL.Query().Get("censor_suffix"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { suf = n }
    }

	token, err := vault.ResolveToken(r.Context(), s.config.Vault.VaultToken, vault.TokenSource(s.config.Vault.VaultTokenSource), s.config.Vault.VaultTokenFile, false)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "vault_token", Message: err.Error()})
		return
	}
	client, err := vault.NewClient(s.config.Vault.VaultAddr, token)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_client", Message: err.Error()})
		return
	}

    entries, warns := listing.Walk(client, path, depth)
    // Build tree; optionally materialize values
    tree := map[string]interface{}{}
    for _, e := range entries {
        isDir := strings.HasSuffix(e, "/")
        parts := strings.Split(strings.Trim(e, "/"), "/")
        cur := tree
        for i, p := range parts {
            last := i == len(parts)-1
            if last && !isDir && include == "values" {
                // materialize leaf value
                data, err := client.GetSecrets(strings.TrimSuffix(e, "/"))
                if err != nil {
                    cur[p+"__error__"] = err.Error()
                    break
                }
                cur[p] = materializeMap(data, reveal, pre, suf)
                break
            }
            if _, ok := cur[p]; !ok {
                cur[p] = map[string]interface{}{}
            }
            if last && !isDir {
                // metadata only: mark as secret
                cur[p].(map[string]interface{})["__secret__"] = true
                break
            }
            cur = cur[p].(map[string]interface{})
        }
    }

	resp := map[string]interface{}{
		"path":      path,
		"timestamp": time.Now(),
		"tree":      tree,
	}
	if len(warns) > 0 {
		ws := make([]string, 0, len(warns))
		for _, w2 := range warns {
			ws = append(ws, w2.Error())
		}
		resp["warnings"] = ws
	}

    writeJSON(w, http.StatusOK, resp)
    log.Debug().Str("path", path).Int("depth", depth).Msg("served tree")
}

// materializeMap mirrors cmds/tree.go censoring semantics
func materializeMap(data map[string]interface{}, reveal bool, pre int, suf int) map[string]string {
    out := make(map[string]string, len(data))
    for k, v := range data {
        sval := fmt.Sprintf("%v", v)
        if reveal {
            out[k] = sval
        } else {
            out[k] = censorString(sval, pre, suf)
        }
    }
    return out
}

func censorString(s string, pre int, suf int) string {
    if pre < 0 { pre = 0 }
    if suf < 0 { suf = 0 }
    n := len(s)
    if n == 0 { return s }
    if pre+suf >= n { return strings.Repeat("*", n) }
    return s[:pre] + "..." + s[n-suf:]
}

// GET /api/v1/vault/secrets/{path}
func (s *Server) handleVaultSecrets(w http.ResponseWriter, r *http.Request) {
    if s.config == nil || s.config.Vault == nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "server_config", Message: "vault settings missing"})
        return
    }

    // path after /api/v1/vault/secrets/
    prefix := "/api/v1/vault/secrets/"
    if !strings.HasPrefix(r.URL.Path, prefix) {
        writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "invalid secrets path"})
        return
    }
    secretPath := strings.TrimPrefix(r.URL.Path, prefix)
    if secretPath == "" {
        writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "missing path"})
        return
    }

    reveal := r.URL.Query().Get("reveal") == "true"
    pre := 2
    suf := 2
    if v := r.URL.Query().Get("censor_prefix"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { pre = n }
    }
    if v := r.URL.Query().Get("censor_suffix"); v != "" {
        if n, err := strconv.Atoi(v); err == nil { suf = n }
    }

    token, err := vault.ResolveToken(r.Context(), s.config.Vault.VaultToken, vault.TokenSource(s.config.Vault.VaultTokenSource), s.config.Vault.VaultTokenFile, false)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "vault_token", Message: err.Error()})
        return
    }
    client, err := vault.NewClient(s.config.Vault.VaultAddr, token)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_client", Message: err.Error()})
        return
    }

    data, err := client.GetSecrets(secretPath)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_get", Message: err.Error()})
        return
    }

    resp := map[string]interface{}{
        "path":      secretPath,
        "timestamp": time.Now(),
        "secrets":   materializeMap(data, reveal, pre, suf),
    }
    writeJSON(w, http.StatusOK, resp)
    log.Debug().Str("path", secretPath).Bool("reveal", reveal).Msg("served secrets")
}

// GET /api/v1/vault/status
func (s *Server) handleVaultStatus(w http.ResponseWriter, r *http.Request) {
    if s.config == nil || s.config.Vault == nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "server_config", Message: "vault settings missing"})
        return
    }

    token, err := vault.ResolveToken(r.Context(), s.config.Vault.VaultToken, vault.TokenSource(s.config.Vault.VaultTokenSource), s.config.Vault.VaultTokenFile, false)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "vault_token", Message: err.Error()})
        return
    }
    client, err := vault.NewClient(s.config.Vault.VaultAddr, token)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_client", Message: err.Error()})
        return
    }

    // Try to get token info
    tokenInfo := map[string]interface{}{
        "address": s.config.Vault.VaultAddr,
        "token_source": s.config.Vault.VaultTokenSource,
    }

    // Try to lookup token details (this may fail if token doesn't have permission)
    if tokenData, err := client.GetAPIClient().Auth().Token().LookupSelf(); err == nil && tokenData != nil {
        if data := tokenData.Data; data != nil {
            if displayName, ok := data["display_name"].(string); ok && displayName != "" {
                tokenInfo["display_name"] = displayName
            }
            if policies, ok := data["policies"].([]interface{}); ok {
                tokenInfo["policies"] = policies
            }
            if ttl, ok := data["ttl"].(json.Number); ok {
                tokenInfo["ttl_seconds"] = ttl
            }
            if entityId, ok := data["entity_id"].(string); ok && entityId != "" {
                tokenInfo["entity_id"] = entityId
            }
        }
    }

    resp := map[string]interface{}{
        "connected": true,
        "timestamp": time.Now(),
        "token_info": tokenInfo,
    }
    writeJSON(w, http.StatusOK, resp)
}

// POST /api/v1/generate/{format}
func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
    if s.config == nil || s.config.Vault == nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "server_config", Message: "vault settings missing"})
        return
    }
    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed", Message: "POST required"})
        return
    }
    prefix := "/api/v1/generate/"
    if !strings.HasPrefix(r.URL.Path, prefix) {
        writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "invalid path"})
        return
    }
    format := strings.TrimPrefix(r.URL.Path, prefix)
    if format == "" {
        writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_request", Message: "missing format"})
        return
    }

    var req struct {
        Path          string            `json:"path"`
        Prefix        string            `json:"prefix"`
        TransformKeys bool              `json:"transform_keys"`
        IncludeKeys   []string          `json:"include_keys"`
        ExcludeKeys   []string          `json:"exclude_keys"`
        SortKeys      bool              `json:"sort_keys"`
        TemplateFile  string            `json:"template_file"`
        EnvMap        map[string]string `json:"env_map"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_json", Message: err.Error()})
        return
    }

    token, err := vault.ResolveToken(r.Context(), s.config.Vault.VaultToken, vault.TokenSource(s.config.Vault.VaultTokenSource), s.config.Vault.VaultTokenFile, false)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "vault_token", Message: err.Error()})
        return
    }
    client, err := vault.NewClient(s.config.Vault.VaultAddr, token)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_client", Message: err.Error()})
        return
    }

    secrets, err := client.GetSecrets(req.Path)
    if err != nil {
        writeJSON(w, http.StatusNotFound, errorResponse{Code: "vault_get", Message: err.Error()})
        return
    }
    if len(req.EnvMap) > 0 {
        mapped := make(map[string]interface{}, len(req.EnvMap))
        for envName, srcKey := range req.EnvMap {
            if v, ok := secrets[srcKey]; ok {
                mapped[envName] = v
            }
        }
        secrets = mapped
        req.TransformKeys = false
        req.Prefix = ""
        req.IncludeKeys = nil
        req.ExcludeKeys = nil
    }

    gen := envrc.NewGenerator(&envrc.Options{
        Prefix:        req.Prefix,
        ExcludeKeys:   req.ExcludeKeys,
        IncludeKeys:   req.IncludeKeys,
        TransformKeys: req.TransformKeys,
        Format:        format,
        TemplateFile:  req.TemplateFile,
        SortKeys:      req.SortKeys,
    })
    content, err := gen.Generate(secrets)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "generate_failed", Message: err.Error()})
        return
    }
    writeJSON(w, http.StatusOK, map[string]any{ "format": format, "content": content })
}

// POST /api/v1/batch/process (dry-run only)
func (s *Server) handleBatchProcess(w http.ResponseWriter, r *http.Request) {
    if s.config == nil || s.config.Vault == nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "server_config", Message: "vault settings missing"})
        return
    }
    if r.Method != http.MethodPost {
        writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Code: "method_not_allowed", Message: "POST required"})
        return
    }
    token, err := vault.ResolveToken(r.Context(), s.config.Vault.VaultToken, vault.TokenSource(s.config.Vault.VaultTokenSource), s.config.Vault.VaultTokenFile, false)
    if err != nil {
        writeJSON(w, http.StatusUnauthorized, errorResponse{Code: "vault_token", Message: err.Error()})
        return
    }
    client, err := vault.NewClient(s.config.Vault.VaultAddr, token)
    if err != nil {
        writeJSON(w, http.StatusInternalServerError, errorResponse{Code: "vault_client", Message: err.Error()})
        return
    }

    var cfgYaml string
    if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
        b, err := io.ReadAll(r.Body)
        if err != nil {
            writeJSON(w, http.StatusBadRequest, errorResponse{Code: "read_body", Message: err.Error()})
            return
        }
        cfgYaml = string(b)
    } else {
        var payload struct { Config string `json:"config"` }
        if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
            writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_json", Message: err.Error()})
            return
        }
        cfgYaml = payload.Config
    }

    var cfg batch.Config
    if err := yaml.Unmarshal([]byte(cfgYaml), &cfg); err != nil {
        writeJSON(w, http.StatusBadRequest, errorResponse{Code: "bad_yaml", Message: err.Error()})
        return
    }

    proc := &batch.Processor{ Client: client }
    // capture stdout for dry-run
    oldStdout := os.Stdout
    rpr, wpr, _ := os.Pipe()
    os.Stdout = wpr
    err = proc.Process(&cfg, batch.ProcessorOptions{ DryRun: true, SortKeys: true, ContinueOnError: true })
    wpr.Close()
    os.Stdout = oldStdout
    if err != nil {
        writeJSON(w, http.StatusUnprocessableEntity, errorResponse{Code: "batch_failed", Message: err.Error()})
        return
    }
    out, _ := io.ReadAll(rpr)
    writeJSON(w, http.StatusOK, map[string]any{ "dry_run": true, "output": string(out) })
}


