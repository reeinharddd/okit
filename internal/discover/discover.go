package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
)

type Service struct {
	db db.DBInterface
}

type NewServiceParams struct {
	DB db.DBInterface
}

type ModelEntry struct {
	ID       string
	Provider string
}

func NewService(params NewServiceParams) *Service {
	return &Service{db: params.DB}
}

var nonChatKeywords = []string{
	"embedding", "embed", "moderation", "ocr", "tts", "transcribe",
	"realtime", "imagen", "veo", "whisper", "speech", "dall-e",
	"stable-diffusion", "sdxl", "mistral-embed", "codestral-embed",
	"mistral-moderation", "mistral-ocr", "safety", "prompt-guard",
}

func isChatModel(id string) bool {
	lower := strings.ToLower(id)
	for _, kw := range nonChatKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}

func (s *Service) Discover(ctx context.Context) error {
	providers, err := s.db.ListProviders()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}

	for _, prov := range providers {
		apiKey := os.Getenv(prov.KeyEnv)
		if apiKey == "" {
			continue
		}
		if prov.CatalogURL == "" {
			fmt.Printf("  Warning [%s]: no catalog_url set, skipping\n", prov.ID)
			continue
		}
		entries, err := fetchCatalog(ctx, &prov, apiKey)
		if err != nil {
			fmt.Printf("  Warning [%s]: %v\n", prov.ID, err)
			continue
		}
		now := time.Now().Unix()
		_ = s.db.UpsertProvider(&models.Provider{
			ID:         prov.ID,
			Name:       prov.Name,
			BaseURL:    prov.BaseURL,
			CatalogURL: prov.CatalogURL,
			KeyEnv:     prov.KeyEnv,
			IsFree:     prov.IsFree,
			Source:     prov.Source,
			Status:     "active",
			Priority:   prov.Priority,
			LastSynced: now,
		})
		count := 0
		for _, m := range entries {
			if !isChatModel(m.ID) {
				continue
			}
			bare := m.ID
			// Some catalogs include provider name prefix (e.g. "groq/compound").
			// Avoid double-prefix "groq/groq/compound".
			prefix := prov.ID + "/"
			if strings.HasPrefix(bare, prefix) {
				bare = bare[len(prefix):]
			}
			fullID := prov.ID + "/" + bare
			_ = s.db.UpsertModel(&models.Model{
				ID:          fullID,
				ProviderID:  prov.ID,
				DisplayName: bare,
				Source:      "discovered",
				Status:      "untested",
			})
			count++
		}
		fmt.Printf("  %s: %d models discovered\n", prov.ID, count)
	}
	return nil
}

func fetchCatalog(ctx context.Context, prov *models.Provider, apiKey string) ([]ModelEntry, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", prov.CatalogURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "opencode-kit/discover (Go)")
	req.Header.Set("Accept", "application/json")

	// GitHub models use a different Accept header
	if prov.ID == "github-models" || prov.ID == "github-copilot" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	entries := make([]ModelEntry, 0, len(result.Data))
	for _, m := range result.Data {
		entries = append(entries, ModelEntry{ID: m.ID, Provider: prov.ID})
	}
	return entries, nil
}

func DetectAvailableProviders(querier interface{ ListProviders() ([]models.Provider, error) }) []string {
	providers, err := querier.ListProviders()
	if err != nil {
		return nil
	}
	var out []string
	for _, p := range providers {
		if os.Getenv(p.KeyEnv) != "" {
			out = append(out, p.ID)
		}
	}
	return out
}
