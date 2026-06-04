package discover

import (
	"fmt"
	"os"

	"github.com/reeinharddd/okit/pkg/models"
)

const defaultContextWindow = 128000

// ActivateUntestedFreeModels marks untested models from free providers
// as active without requiring an API audit call.
func (s *Service) ActivateUntestedFreeModels() error {
	providers, err := s.db.ListProviders()
	if err != nil {
		return err
	}
	for _, prov := range providers {
		if !prov.IsFree {
			continue
		}
		modelsList, err := s.db.ListModelsByProvider(prov.ID)
		if err != nil {
			continue
		}
		count := 0
		for _, m := range modelsList {
			if m.Status != "untested" {
				continue
			}
			m.Status = "active"
			m.Tier = "free"
			if m.ContextWindow <= 0 {
				m.ContextWindow = defaultContextWindow
			}
			if err := s.db.UpsertModel(&m); err == nil {
				count++
			}
		}
		if count > 0 {
			fmt.Printf("  %s: %d models auto-activated\n", prov.ID, count)
		}
	}
	return nil
}

// DeduplicateProviders merges providers that share the same base URL,
// moving all models to the provider with the active API key set.
func (s *Service) DeduplicateProviders() error {
	providers, err := s.db.ListProviders()
	if err != nil {
		return err
	}
	byURL := make(map[string][]models.Provider)
	for _, p := range providers {
		if p.BaseURL == "" {
			continue
		}
		byURL[p.BaseURL] = append(byURL[p.BaseURL], p)
	}
	for url, group := range byURL {
		if len(group) < 2 {
			continue
		}
		fmt.Printf("  Merging %d providers sharing URL %s\n", len(group), url)
		// Winner: prefer provider with KeyEnv set, then seed source, then first
		winner := group[0]
		for _, p := range group[1:] {
			winnerHasKey := os.Getenv(winner.KeyEnv) != ""
			pHasKey := os.Getenv(p.KeyEnv) != ""
			if pHasKey && !winnerHasKey {
				winner = p
			} else if !winnerHasKey && !pHasKey && p.Source == "seed" && winner.Source != "seed" {
				winner = p
			}
		}
		for _, p := range group {
			if p.ID == winner.ID {
				continue
			}
			modelsList, err := s.db.ListModelsByProvider(p.ID)
			if err != nil {
				continue
			}
			moved := 0
			for _, m := range modelsList {
				oldID := m.ID
				newID := winner.ID + "/" + m.DisplayName
				existing, _ := s.db.GetModel(newID)
				if existing != nil {
					continue
				}
				m.ID = newID
				m.ProviderID = winner.ID
				if err := s.db.UpsertModel(&m); err != nil {
					continue
				}
				_ = s.db.DeleteModel(oldID)
				moved++
			}
			_ = s.db.DeleteProvider(p.ID)
			fmt.Printf("    %s → %s (%d models moved)\n", p.ID, winner.ID, moved)
		}
	}
	return nil
}
