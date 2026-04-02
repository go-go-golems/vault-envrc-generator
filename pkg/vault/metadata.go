package vault

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// ErrMetadataNotAvailable indicates that secret metadata cannot be retrieved for the given path.
var ErrMetadataNotAvailable = errors.New("secret metadata not available")

// SecretMetadata captures the KV v2 metadata information for a secret.
type SecretMetadata struct {
	CurrentVersion int
	OldestVersion  int
	MaxVersions    int
	CreatedTime    *time.Time
	UpdatedTime    *time.Time
	Versions       map[int]SecretVersionMetadata
}

// SecretVersionMetadata encapsulates metadata for a specific secret version.
type SecretVersionMetadata struct {
	Version      int
	CreatedTime  *time.Time
	DeletionTime *time.Time
	Destroyed    bool
}

// GetSecretMetadata retrieves KV v2 metadata for the provided secret path.
func (c *Client) GetSecretMetadata(path string) (*SecretMetadata, error) {
	mountPath, secretPath := c.parsePath(path)
	if secretPath == "" {
		return nil, fmt.Errorf("cannot retrieve metadata for mount path %q", path)
	}

	fullPath := fmt.Sprintf("%s/metadata/%s", mountPath, secretPath)
	secret, err := c.client.Logical().Read(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata from %s: %w", fullPath, err)
	}
	if secret == nil {
		return nil, ErrMetadataNotAvailable
	}

	rawData := extractMetadataData(secret.Data)
	if rawData == nil {
		return nil, ErrMetadataNotAvailable
	}

	meta := &SecretMetadata{
		CurrentVersion: intFromAny(rawData["current_version"]),
		OldestVersion:  intFromAny(rawData["oldest_version"]),
		MaxVersions:    intFromAny(rawData["max_versions"]),
		CreatedTime:    timePtrFromAny(rawData["created_time"]),
		UpdatedTime:    timePtrFromAny(rawData["updated_time"]),
		Versions:       map[int]SecretVersionMetadata{},
	}

	if versionsRaw, ok := rawData["versions"].(map[string]interface{}); ok {
		for ver, item := range versionsRaw {
			vn := intFromString(ver)
			if vn == 0 && ver != "0" {
				continue
			}
			vmeta := SecretVersionMetadata{
				Version:      vn,
				Destroyed:    boolFromAny(item, "destroyed"),
				CreatedTime:  timePtrFromAny(fromMap(item, "created_time")),
				DeletionTime: timePtrFromAny(fromMap(item, "deletion_time")),
			}
			meta.Versions[vn] = vmeta
		}
	}

	return meta, nil
}

func extractMetadataData(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	if data, ok := m["data"].(map[string]interface{}); ok {
		return data
	}
	return m
}

func intFromAny(v interface{}) int {
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	case float32:
		return int(t)
	case string:
		return intFromString(t)
	default:
		return 0
	}
}

func intFromString(s string) int {
	if s == "" {
		return 0
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func boolFromAny(m interface{}, key string) bool {
	if mp, ok := m.(map[string]interface{}); ok {
		if v, ok := mp[key]; ok {
			switch t := v.(type) {
			case bool:
				return t
			case string:
				return t == "true"
			}
		}
	}
	return false
}

func fromMap(m interface{}, key string) interface{} {
	if mp, ok := m.(map[string]interface{}); ok {
		return mp[key]
	}
	return nil
}

func timePtrFromAny(v interface{}) *time.Time {
	switch t := v.(type) {
	case time.Time:
		return &t
	case *time.Time:
		return t
	case string:
		if t == "" {
			return nil
		}
		if ts, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return &ts
		}
		if ts, err := time.Parse(time.RFC3339, t); err == nil {
			return &ts
		}
	case nil:
		return nil
	}
	return nil
}
