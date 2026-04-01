package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
)

// APIKey holds metadata for one registered API key.
type APIKey struct {
	Name        string    `json:"name"`
	Subdomains  []string  `json:"subdomains"`
	MaxTunnels  int       `json:"max_tunnels"`
	CreatedAt   time.Time `json:"created_at"`
	Active      bool      `json:"active"`
	tunnelCount int       // runtime only, not persisted
}

// keyStoreData is the on-disk JSON structure.
type keyStoreData struct {
	Keys map[string]*APIKey `json:"keys"`
}

// KeyStore manages API keys persisted to a JSON file.
type KeyStore struct {
	mu       sync.RWMutex
	keys     map[string]*APIKey // htk_xxx → APIKey
	filePath string
}

// NewKeyStore loads keys from filePath. Creates empty file if not exists.
func NewKeyStore(filePath string) (*KeyStore, error) {
	ks := &KeyStore{
		keys:     make(map[string]*APIKey),
		filePath: filePath,
	}
	if filePath == "" {
		return ks, nil
	}

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		slog.Info("key store file not found, starting empty", "path", filePath)
		return ks, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read key store: %w", err)
	}
	// Handle empty file gracefully.
	if len(bytes.TrimSpace(data)) == 0 {
		slog.Info("key store file empty, starting fresh", "path", filePath)
		return ks, nil
	}

	var stored keyStoreData
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("parse key store: %w", err)
	}
	if stored.Keys != nil {
		ks.keys = stored.Keys
	}
	slog.Info("loaded API keys", "count", len(ks.keys))
	return ks, nil
}

// Validate returns true if the key exists and is active.
func (ks *KeyStore) Validate(key string) bool {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	k, ok := ks.keys[key]
	return ok && k.Active
}

// GetKey returns the APIKey for key, or nil if not found.
func (ks *KeyStore) GetKey(key string) *APIKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.keys[key]
}

// CreateKey generates a new API key with the given metadata.
// Returns the full key string. Persists to disk.
func (ks *KeyStore) CreateKey(name string, subdomains []string, maxTunnels int) (string, error) {
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if maxTunnels <= 0 {
		maxTunnels = 10
	}

	// Validate subdomains not owned by another key.
	ks.mu.Lock()
	defer ks.mu.Unlock()

	for _, sub := range subdomains {
		if err := ValidateSubdomain(sub); err != nil {
			return "", err
		}
		if owner := ks.findSubdomainOwnerLocked(sub); owner != "" {
			return "", fmt.Errorf("subdomain %q is already owned", sub)
		}
	}

	key := GenerateAPIKey()
	ks.keys[key] = &APIKey{
		Name:       name,
		Subdomains: subdomains,
		MaxTunnels: maxTunnels,
		CreatedAt:  time.Now().UTC(),
		Active:     true,
	}

	if err := ks.saveLocked(); err != nil {
		delete(ks.keys, key)
		return "", fmt.Errorf("persist key: %w", err)
	}
	slog.Info("API key created", "name", name, "subdomains", subdomains)
	return key, nil
}

// RevokeKey sets active=false for the key. Persists to disk.
func (ks *KeyStore) RevokeKey(keyID string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	k := ks.findKeyByIDOrPreview(keyID)
	if k == nil {
		return fmt.Errorf("key not found")
	}
	k.Active = false
	return ks.saveLocked()
}

// ListKeys returns a copy of all keys.
func (ks *KeyStore) ListKeys() map[string]*APIKey {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	out := make(map[string]*APIKey, len(ks.keys))
	for id, k := range ks.keys {
		out[id] = k
	}
	return out
}

// FindSubdomainOwner returns the key ID that owns subdomain, or "".
func (ks *KeyStore) FindSubdomainOwner(subdomain string) string {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	return ks.findSubdomainOwnerLocked(subdomain)
}

func (ks *KeyStore) findSubdomainOwnerLocked(subdomain string) string {
	for keyID, k := range ks.keys {
		if !k.Active {
			continue
		}
		for _, s := range k.Subdomains {
			if s == subdomain {
				return keyID
			}
		}
	}
	return ""
}

// AddSubdomain claims a new subdomain for key. Persists to disk.
func (ks *KeyStore) AddSubdomain(keyID, subdomain string) error {
	if err := ValidateSubdomain(subdomain); err != nil {
		return err
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()

	k, ok := ks.keys[keyID]
	if !ok || !k.Active {
		return fmt.Errorf("invalid key")
	}
	if owner := ks.findSubdomainOwnerLocked(subdomain); owner != "" {
		if owner == keyID {
			return nil // already owned
		}
		return fmt.Errorf("subdomain %q is already owned", subdomain)
	}
	k.Subdomains = append(k.Subdomains, subdomain)
	return ks.saveLocked()
}

// RemoveSubdomain releases a subdomain from key. Persists to disk.
func (ks *KeyStore) RemoveSubdomain(keyID, subdomain string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	k, ok := ks.keys[keyID]
	if !ok {
		return fmt.Errorf("invalid key")
	}
	filtered := make([]string, 0, len(k.Subdomains))
	for _, s := range k.Subdomains {
		if s != subdomain {
			filtered = append(filtered, s)
		}
	}
	k.Subdomains = filtered
	return ks.saveLocked()
}

// IncrementTunnels atomically increments tunnel count for key.
func (ks *KeyStore) IncrementTunnels(key string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	k, ok := ks.keys[key]
	if !ok {
		return fmt.Errorf("unknown key")
	}
	if k.tunnelCount >= k.MaxTunnels {
		return fmt.Errorf("tunnel limit reached (%d/%d)", k.tunnelCount, k.MaxTunnels)
	}
	k.tunnelCount++
	return nil
}

// DecrementTunnels decrements tunnel count for key (floor 0).
func (ks *KeyStore) DecrementTunnels(key string) {
	ks.mu.Lock()
	defer ks.mu.Unlock()
	if k, ok := ks.keys[key]; ok && k.tunnelCount > 0 {
		k.tunnelCount--
	}
}

// OwnedSubdomains returns the subdomains owned by key.
func (ks *KeyStore) OwnedSubdomains(key string) []string {
	ks.mu.RLock()
	defer ks.mu.RUnlock()
	k, ok := ks.keys[key]
	if !ok {
		return nil
	}
	return k.Subdomains
}

// findKeyByIDOrPreview finds a key by full ID or masked preview (htk_a1b2...d4e5).
func (ks *KeyStore) findKeyByIDOrPreview(idOrPreview string) *APIKey {
	if k, ok := ks.keys[idOrPreview]; ok {
		return k
	}
	// Try matching by preview
	for id, k := range ks.keys {
		if MaskKey(id) == idOrPreview {
			return k
		}
	}
	return nil
}

// saveLocked writes keys to disk. Caller must hold ks.mu.
func (ks *KeyStore) saveLocked() error {
	if ks.filePath == "" {
		return nil
	}
	data, err := json.MarshalIndent(keyStoreData{Keys: ks.keys}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ks.filePath, data, 0600)
}

// GenerateAPIKey returns a new random key with htk_ prefix.
func GenerateAPIKey() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return "htk_" + hex.EncodeToString(b)
}

// MaskKey returns a preview like "htk_a1b2...d4e5".
func MaskKey(key string) string {
	if len(key) < 12 {
		return key
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// IsAPIKey returns true if the token has the htk_ prefix.
func IsAPIKey(token string) bool {
	return strings.HasPrefix(token, "htk_")
}
