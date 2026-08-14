// Package catalog implements static catalog loading from JSON definitions.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/aminio9/gereh/services/model-access/internal/domain"
)

type staticCatalogFile struct {
	Version   string                          `json:"version"`
	Providers map[string]staticProviderConfig `json:"providers"`
}

type staticProviderConfig struct {
	Pools map[string]staticPoolConfig `json:"pools"`
}

type staticPoolConfig struct {
	Models []staticModel `json:"models"`
}

type staticModel struct {
	ProviderModelID     string   `json:"provider_model_id"`
	DisplayName         string   `json:"display_name"`
	Description         string   `json:"description"`
	AgentUsable         bool     `json:"agent_usable"`
	Capabilities        []string `json:"capabilities"`
	InputModalities     []string `json:"input_modalities"`
	OutputModalities    []string `json:"output_modalities"`
	ContextWindowTokens int64    `json:"context_window_tokens"`
	MaxOutputTokens     int64    `json:"max_output_tokens"`
}

// StaticLoader loads and caches platform catalog definitions from a JSON file.
type StaticLoader struct {
	mu       sync.RWMutex
	filePath string
	cached   *staticCatalogFile
}

// NewStaticLoader constructs and initializes a StaticLoader from the given JSON file.
func NewStaticLoader(filePath string) (*StaticLoader, error) {
	loader := &StaticLoader{filePath: filePath}
	if err := loader.reload(); err != nil {
		return nil, fmt.Errorf("load static catalog %q: %w", filePath, err)
	}

	return loader, nil
}

func (l *StaticLoader) reload() error {
	data, err := os.ReadFile(l.filePath)
	if err != nil {
		return err
	}

	var parsed staticCatalogFile
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse static catalog JSON: %w", err)
	}

	l.mu.Lock()
	l.cached = &parsed
	l.mu.Unlock()

	return nil
}

// LoadPlatformOfferings returns discovered models for a given provider and pool.
func (l *StaticLoader) LoadPlatformOfferings(
	providerKey string,
	poolKey string,
) ([]domain.DiscoveredModel, error) {
	l.mu.RLock()
	cached := l.cached
	l.mu.RUnlock()

	if cached == nil {
		return nil, domain.ErrCatalogUnavailable
	}

	providerCfg, ok := cached.Providers[providerKey]
	if !ok {
		return nil, nil
	}

	poolCfg, ok := providerCfg.Pools[poolKey]
	if !ok {
		// Fallback to "global" pool if specific poolKey not explicitly configured
		poolCfg, ok = providerCfg.Pools["global"]
		if !ok {
			return nil, nil
		}
	}

	result := make([]domain.DiscoveredModel, 0, len(poolCfg.Models))
	for _, m := range poolCfg.Models {
		result = append(result, domain.DiscoveredModel{
			ProviderKey:         providerKey,
			ProviderModelID:     m.ProviderModelID,
			DisplayName:         m.DisplayName,
			Description:         m.Description,
			AgentUsable:         m.AgentUsable,
			Capabilities:        m.Capabilities,
			InputModalities:     m.InputModalities,
			OutputModalities:    m.OutputModalities,
			ContextWindowTokens: m.ContextWindowTokens,
			MaxOutputTokens:     m.MaxOutputTokens,
		})
	}

	return result, nil
}

// FindCuratedMetadata searches the static catalog for curated display metadata
// to enrich provider-discovered models.
func (l *StaticLoader) FindCuratedMetadata(
	providerKey string,
	providerModelID string,
) *domain.DiscoveredModel {
	l.mu.RLock()
	cached := l.cached
	l.mu.RUnlock()

	if cached == nil {
		return nil
	}

	providerCfg, ok := cached.Providers[providerKey]
	if !ok {
		return nil
	}

	for _, pool := range providerCfg.Pools {
		for _, m := range pool.Models {
			if m.ProviderModelID == providerModelID {
				return &domain.DiscoveredModel{
					ProviderKey:         providerKey,
					ProviderModelID:     m.ProviderModelID,
					DisplayName:         m.DisplayName,
					Description:         m.Description,
					AgentUsable:         m.AgentUsable,
					Capabilities:        m.Capabilities,
					InputModalities:     m.InputModalities,
					OutputModalities:    m.OutputModalities,
					ContextWindowTokens: m.ContextWindowTokens,
					MaxOutputTokens:     m.MaxOutputTokens,
				}
			}
		}
	}

	return nil
}
