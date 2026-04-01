package server

import (
	"github.com/htn-sys/htn-tunnel/internal/dashboard"
)

// KeyStoreAdapter wraps KeyStore to satisfy dashboard.KeyProvider interface,
// converting between server.APIKey and dashboard.APIKeyInfo types.
type KeyStoreAdapter struct {
	store *KeyStore
}

// NewKeyStoreAdapter creates an adapter around the given KeyStore.
func NewKeyStoreAdapter(store *KeyStore) *KeyStoreAdapter {
	return &KeyStoreAdapter{store: store}
}

func (a *KeyStoreAdapter) Validate(key string) bool {
	return a.store.Validate(key)
}

func (a *KeyStoreAdapter) GetKey(key string) *dashboard.APIKeyInfo {
	k := a.store.GetKey(key)
	if k == nil {
		return nil
	}
	return &dashboard.APIKeyInfo{
		Name:       k.Name,
		Subdomains: k.Subdomains,
		MaxTunnels: k.MaxTunnels,
		Active:     k.Active,
		CreatedAt:  k.CreatedAt,
	}
}

func (a *KeyStoreAdapter) CreateKey(name string, subdomains []string, maxTunnels int) (string, error) {
	return a.store.CreateKey(name, subdomains, maxTunnels)
}

func (a *KeyStoreAdapter) RevokeKey(keyID string) error {
	return a.store.RevokeKey(keyID)
}

func (a *KeyStoreAdapter) ListKeys() map[string]*dashboard.APIKeyInfo {
	keys := a.store.ListKeys()
	out := make(map[string]*dashboard.APIKeyInfo, len(keys))
	for id, k := range keys {
		out[id] = &dashboard.APIKeyInfo{
			Name:       k.Name,
			Subdomains: k.Subdomains,
			MaxTunnels: k.MaxTunnels,
			Active:     k.Active,
			CreatedAt:  k.CreatedAt,
		}
	}
	return out
}

func (a *KeyStoreAdapter) AddSubdomain(keyID, subdomain string) error {
	return a.store.AddSubdomain(keyID, subdomain)
}

func (a *KeyStoreAdapter) RemoveSubdomain(keyID, subdomain string) error {
	return a.store.RemoveSubdomain(keyID, subdomain)
}

func (a *KeyStoreAdapter) OwnedSubdomains(key string) []string {
	return a.store.OwnedSubdomains(key)
}
