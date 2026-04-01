# Phase 1: API Key Store (JSON File)

## Priority: HIGH | Status: Not started | Effort: 3-4h

## Overview
Replace hardcoded `tokens` list with a persistent JSON-based key store. Each key has metadata (name, limits, status). Old `tokens` config continues to work for backward compatibility.

## Key Format
```
htk_a1b2c3d4e5f6...  (prefix "htk_" + 32 hex chars = 36 chars total)
```

## Data Model

**`internal/server/key_store.go`** (new file):

```go
type APIKey struct {
    Name        string   `json:"name"`
    Subdomains  []string `json:"subdomains"`   // fixed subdomains owned by this key
    MaxTunnels  int      `json:"max_tunnels"`
    CreatedAt   time.Time `json:"created_at"`
    Active      bool     `json:"active"`
}

type KeyStore struct {
    mu       sync.RWMutex
    keys     map[string]*APIKey  // htk_xxx → APIKey
    filePath string
}
```

## Storage Format

**`/etc/htn-tunnel/keys.json`:**
```json
{
  "keys": {
    "htk_a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4": {
      "name": "Hoang",
      "subdomains": ["hoang", "myapp"],
      "max_tunnels": 5,
      "created_at": "2026-04-01T10:00:00Z",
      "active": true
    },
    "htk_b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5": {
      "name": "Team Dev",
      "subdomains": ["dev", "staging"],
      "max_tunnels": 10,
      "created_at": "2026-04-01T10:00:00Z",
      "active": true
    }
  }
}
```

## Implementation Steps

### 1. Create `internal/server/key_store.go`

```go
// KeyStore methods:
func NewKeyStore(filePath string) (*KeyStore, error)  // load from disk
func (ks *KeyStore) Validate(key string) bool          // check key exists + active
func (ks *KeyStore) GetKey(key string) *APIKey          // get key info
func (ks *KeyStore) CreateKey(name string, subdomains []string, maxTunnels int) (string, error)
func (ks *KeyStore) RevokeKey(key string) error
func (ks *KeyStore) ListKeys() map[string]*APIKey
func (ks *KeyStore) save() error                        // persist to disk
func GenerateAPIKey() string                            // htk_ + 32 hex
```

### 2. Add config field

**`internal/config/config.go`:**
```go
type ServerConfig struct {
    // ... existing fields ...
    KeyStorePath string `yaml:"key_store_path"` // default: same dir as config + "/keys.json"
}
```

### 3. Update auth flow

**`internal/server/auth.go`** — modify `TokenStore` or create wrapper:

```go
// AuthStore wraps both legacy TokenStore and new KeyStore
type AuthStore struct {
    legacy *TokenStore  // old tokens from config
    keys   *KeyStore    // new API keys from keys.json
}

func (a *AuthStore) Validate(token string) bool {
    // Try new key store first (htk_ prefix)
    if strings.HasPrefix(token, "htk_") {
        return a.keys.Validate(token)
    }
    // Fall back to legacy tokens
    return a.legacy.Validate(token)
}
```

### 4. Update server.go to use AuthStore

**`internal/server/server.go`:**
```go
// In NewServer():
keyStore, err := NewKeyStore(cfg.KeyStorePath)
authStore := &AuthStore{legacy: tokenStore, keys: keyStore}
```

### 5. Add config defaults

```go
func (c *ServerConfig) defaults() {
    // ...
    if c.KeyStorePath == "" {
        c.KeyStorePath = "/etc/htn-tunnel/keys.json"
    }
}
```

## Files to Create
- `internal/server/key_store.go`

## Files to Modify
- `internal/config/config.go` — add `KeyStorePath`
- `internal/server/auth.go` — add `AuthStore` wrapper
- `internal/server/server.go` — use `AuthStore`

## Success Criteria
- [ ] `htk_` keys validated from keys.json
- [ ] Old `tokens` config still works
- [ ] Keys can be created/revoked via KeyStore API
- [ ] keys.json persisted on disk after changes
- [ ] Server starts with empty keys.json (no keys = no API key users yet)

## Security
- keys.json file permissions: 0600 (owner read/write only)
- Keys are NOT bcrypt-hashed (unlike old tokens) — simpler, key is random enough
- Admin token required for key management API
