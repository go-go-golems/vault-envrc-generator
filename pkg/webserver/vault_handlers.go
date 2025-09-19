package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/go-go-golems/vault-envrc-generator/pkg/listing"
	"github.com/go-go-golems/vault-envrc-generator/pkg/vault"
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


