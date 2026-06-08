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
	ID           string
	Provider     string
	Capabilities json.RawMessage
	Aliases      []string
	Description  string
	MaxContext   int
	Deprecation  json.RawMessage
	DefaultTemp  float64
	Family       string
	ReleaseDate  string
	Pricing      json.RawMessage
	Created      int64
	OwnedBy      string
}

func NewService(params NewServiceParams) *Service {
	return &Service{db: params.DB}
}

var nonChatKeywords = []string{
	"embedding", "embed", "moderation", "ocr", "tts", "transcribe",
	"realtime", "imagen", "veo", "whisper", "speech", "dall-e",
	"stable-diffusion", "sdxl", "mistral-embed", "codestral-embed",
	"mistral-moderation", "mistral-ocr", "safety", "prompt-guard",
	"orpheus",
}

func hasCapabilities(id string, capsJSON json.RawMessage) bool {
	if len(capsJSON) > 0 {
		var caps map[string]bool
		if err := json.Unmarshal(capsJSON, &caps); err == nil {
			if v, ok := caps["completion_chat"]; ok {
				return v
			}
		}
	}
	return isChatModel(id)
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

type CatalogModel struct {
	ID           string          `json:"id"`
	Object       string          `json:"object,omitempty"`
	Created      int64           `json:"created,omitempty"`
	OwnedBy      string          `json:"owned_by,omitempty"`
	Capabilities json.RawMessage `json:"capabilities"`
	Description  string          `json:"description,omitempty"`
	MaxContext   int             `json:"max_context_length,omitempty"`
	Aliases      []string        `json:"aliases,omitempty"`
	Deprecation  json.RawMessage `json:"deprecation,omitempty"`
	DefaultTemp  float64         `json:"default_model_temperature,omitempty"`
	Family       string          `json:"family,omitempty"`
	ReleaseDate  string          `json:"release_date,omitempty"`
	Pricing      json.RawMessage `json:"pricing,omitempty"`
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
			if !hasCapabilities(m.ID, m.Capabilities) {
				continue
			}
			rawID := m.ID
			bare := rawID
			prefix := prov.ID + "/"
			if strings.HasPrefix(bare, prefix) {
				bare = bare[len(prefix):]
			}
			fullID := prov.ID + "/" + bare

			// Extract capabilities
			var caps map[string]bool
			json.Unmarshal(m.Capabilities, &caps)

			// Build aliases JSON
			var aliasesJSON string
			if len(m.Aliases) > 0 {
				b, _ := json.Marshal(m.Aliases)
				aliasesJSON = string(b)
			}

			// Parse deprecation
			var depStr string
			if len(m.Deprecation) > 0 {
				var dep interface{}
				if err := json.Unmarshal(m.Deprecation, &dep); err == nil {
					b, _ := json.Marshal(dep)
					depStr = string(b)
				}
			}

			// Parse pricing
			var promptCost, completionCost, cacheReadCost, cacheWriteCost float64
			if len(m.Pricing) > 0 {
				var pricing struct {
					Prompt     float64 `json:"prompt"`
					Completion float64 `json:"completion"`
					CacheRead  float64 `json:"cache_read"`
					CacheWrite float64 `json:"cache_write"`
				}
				if err := json.Unmarshal(m.Pricing, &pricing); err == nil {
					promptCost = pricing.Prompt
					completionCost = pricing.Completion
					cacheReadCost = pricing.CacheRead
					cacheWriteCost = pricing.CacheWrite
				}
			}

			// Build modalities
			var modInput, modOutput string
			if caps["vision"] {
				if modInput != "" {
					modInput = `["text","image"]`
				}
			}
			if caps["audio"] || caps["audio_transcription"] || caps["audio_speech"] {
				if modInput == "" {
					modInput = `["text"]`
				} else {
					modInput = `["text","image","audio"]`
				}
			}

			// Build interleaved JSON
			var interleavedJSON string
			if caps["reasoning"] {
				interleavedJSON = `true`
				if caps["reasoning_content"] {
					interleavedJSON = `{"field":"reasoning_content"}`
				} else if caps["reasoning_details"] {
					interleavedJSON = `{"field":"reasoning_details"}`
				}
			}

			model := &models.Model{
				ID:                fullID,
				ProviderID:        prov.ID,
				DisplayName:       rawID,
				Description:       m.Description,
				ContextWindow:     m.MaxContext,
				FunctionCalling:   caps["function_calling"],
				Vision:            caps["vision"],
				Reasoning:         caps["reasoning"],
				Audio:             caps["audio"] || caps["audio_transcription"] || caps["audio_speech"],
				OCR:               caps["ocr"],
				FineTuning:        caps["fine_tuning"],
				Classification:    caps["classification"],
				Moderation:        caps["moderation"],
				Streaming:         true,
				StructuredOutput:  caps["structured_outputs"],
				DefaultTemp:       m.DefaultTemp,
				Tier:              "unknown",
				Status:            "untested",
				Aliases:           aliasesJSON,
				Family:            m.Family,
				ReleaseDate:       m.ReleaseDate,
				Deprecation:       depStr,
				Interleaved:       interleavedJSON,
				ModalitiesInput:   modInput,
				ModalitiesOutput:  modOutput,
				CreatedTimestamp:  m.Created,
				OwnedBy:           m.OwnedBy,
				PricingPrompt:     promptCost,
				PricingCompletion: completionCost,
				PricingCacheRead:  cacheReadCost,
				PricingCacheWrite: cacheWriteCost,
				Source:            "discovered",
			}
			_ = s.db.UpsertModel(model)
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
		Data []CatalogModel `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	entries := make([]ModelEntry, 0, len(result.Data))
	for _, m := range result.Data {
		entries = append(entries, ModelEntry{
			ID:           m.ID,
			Provider:     prov.ID,
			Capabilities: m.Capabilities,
			Aliases:      m.Aliases,
			Description:  m.Description,
			MaxContext:   m.MaxContext,
			Deprecation:  m.Deprecation,
			DefaultTemp:  m.DefaultTemp,
			Family:       m.Family,
			ReleaseDate:  m.ReleaseDate,
			Pricing:      m.Pricing,
			Created:      m.Created,
			OwnedBy:      m.OwnedBy,
		})
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
