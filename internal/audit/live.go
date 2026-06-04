package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/reeinharddd/okit/pkg/models"
)

type LiveProviderResult struct {
	ProviderID string
	RealCount  int
	DBCount    int
	Phantom    []string
	Missing    []string
	FetchError string
}

type LiveModelResult struct {
	ModelID   string
	Provider  string
	Status    string
	LatencyMs float64
	ErrorMsg  string
}

type FixReport struct {
	ProviderID   string
	PhantomFixed int
	MissingAdded int
	Skipped      int
	FetchError   string
}

type LiveReport struct {
	Providers []LiveProviderResult
	Smoke     []LiveModelResult
}

type Live struct {
	db      dbReader
	hc      *http.Client
	workers int
}

type dbReader interface {
	ListProviders() ([]models.Provider, error)
	ListModelsByProvider(providerID string) ([]models.Model, error)
	UpsertModel(m *models.Model) error
}

func NewLive(db dbReader, workers int) *Live {
	if workers <= 0 {
		workers = 4
	}
	return &Live{
		db:      db,
		hc:      &http.Client{Timeout: 25 * time.Second},
		workers: workers,
	}
}

func (l *Live) FetchRealModels(ctx context.Context, prov *models.Provider) ([]string, error) {
	apiKey := os.Getenv(prov.KeyEnv)
	if apiKey == "" {
		return nil, fmt.Errorf("missing API key %s", prov.KeyEnv)
	}

	listURL := strings.TrimRight(prov.BaseURL, "/") + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", listURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "opencode-kit/audit (Go)")
	req.Header.Set("Accept", "application/json")

	// GitHub models use a different Accept header
	if prov.ID == "github-models" || prov.ID == "github-copilot" {
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}

	resp, err := l.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

func stripProviderPrefix(provID, mid string) string {
	prefix := provID + "/"
	if strings.HasPrefix(mid, prefix) {
		return mid[len(prefix):]
	}
	return mid
}

func (l *Live) DiffProvider(ctx context.Context, prov *models.Provider) (LiveProviderResult, error) {
	res := LiveProviderResult{ProviderID: prov.ID}
	real, err := l.FetchRealModels(ctx, prov)
	if err != nil {
		res.FetchError = err.Error()
		dbModels, _ := l.db.ListModelsByProvider(prov.ID)
		res.DBCount = len(dbModels)
		return res, nil
	}
	res.RealCount = len(real)
	realSet := make(map[string]bool, len(real))
	for _, r := range real {
		realSet[r] = true
	}
	dbModels, err := l.db.ListModelsByProvider(prov.ID)
	if err != nil {
		return res, err
	}
	res.DBCount = len(dbModels)
	dbSet := make(map[string]bool, len(dbModels))
	for _, m := range dbModels {
		dbSet[stripProviderPrefix(prov.ID, m.ID)] = true
	}
	for r := range realSet {
		if !dbSet[r] {
			res.Missing = append(res.Missing, r)
		}
	}
	for d := range dbSet {
		if !realSet[d] {
			res.Phantom = append(res.Phantom, d)
		}
	}
	return res, nil
}

func (l *Live) DiffAll(ctx context.Context) ([]LiveProviderResult, error) {
	providers, err := l.db.ListProviders()
	if err != nil {
		return nil, err
	}
	results := make([]LiveProviderResult, 0, len(providers))
	for i := range providers {
		p := &providers[i]
		if p.Status != "active" {
			continue
		}
		if p.BaseURL == "" {
			continue
		}
		r, err := l.DiffProvider(ctx, p)
		if err != nil {
			r.FetchError = err.Error()
		}
		results = append(results, r)
	}
	return results, nil
}

type SmokeOpts struct {
	OnlyBroken bool
	MaxWorkers int
}

func (l *Live) SmokeAll(ctx context.Context, opts SmokeOpts) ([]LiveModelResult, error) {
	providers, err := l.db.ListProviders()
	if err != nil {
		return nil, err
	}
	results := make([]LiveModelResult, 0)
	for i := range providers {
		p := &providers[i]
		if p.Status != "active" {
			continue
		}
		if p.BaseURL == "" {
			continue
		}
		apiKey := os.Getenv(p.KeyEnv)
		if apiKey == "" {
			continue
		}
		models, err := l.db.ListModelsByProvider(p.ID)
		if err != nil {
			continue
		}
		for _, m := range models {
			if m.Status != "active" {
				continue
			}
			r := l.SmokeOne(ctx, p, m.DisplayName, apiKey)
			results = append(results, r)
		}
	}
	return results, nil
}

func (l *Live) SmokeOne(ctx context.Context, prov *models.Provider, modelID, apiKey string) LiveModelResult {
	res := LiveModelResult{ModelID: modelID, Provider: prov.ID}

	chatURL := strings.TrimRight(prov.BaseURL, "/") + "/chat/completions"

	bodyMap := map[string]interface{}{
		"model":    modelID,
		"messages": []map[string]string{{"role": "user", "content": "OK"}},
	}
	lower := strings.ToLower(modelID)
	isReasoning := strings.Contains(lower, "o3") || strings.Contains(lower, "o4") ||
		strings.Contains(lower, "deepseek-r1") || strings.Contains(lower, "gpt-5") ||
		strings.Contains(lower, "gpt-oss") || strings.Contains(lower, "qwq") ||
		strings.Contains(lower, "reasoning")
	if isReasoning {
		bodyMap["max_completion_tokens"] = 1
	} else {
		bodyMap["max_tokens"] = 1
	}
	body, _ := json.Marshal(bodyMap)
	req, err := http.NewRequestWithContext(ctx, "POST", chatURL, bytes.NewReader(body))
	if err != nil {
		res.Status = "err"
		res.ErrorMsg = err.Error()
		return res
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "opencode-kit/audit (Go)")
	t0 := time.Now()
	resp, err := l.hc.Do(req)
	res.LatencyMs = float64(time.Since(t0).Microseconds()) / 1000.0
	if err != nil {
		res.Status = "err"
		res.ErrorMsg = err.Error()
		return res
	}
	defer resp.Body.Close()
	body2, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	switch resp.StatusCode {
	case 200:
		res.Status = "ok"
	case 404:
		res.Status = "not_found"
		res.ErrorMsg = strings.TrimSpace(string(body2))
	case 429:
		res.Status = "rate_limited"
		res.ErrorMsg = strings.TrimSpace(string(body2))
	case 401, 403:
		res.Status = "unauthorized"
		res.ErrorMsg = strings.TrimSpace(string(body2))
	default:
		res.Status = fmt.Sprintf("http_%d", resp.StatusCode)
		res.ErrorMsg = strings.TrimSpace(string(body2))
	}
	return res
}

var nonChatKeywords = []string{
	"embedding", "embed", "moderation", "ocr", "tts", "transcribe",
	"realtime", "imagen", "veo", "whisper", "speech", "dall-e",
	"stable-diffusion", "sdxl", "mistral-embed", "codestral-embed",
	"mistral-moderation", "mistral-ocr", "safety", "prompt-guard",
	"audio-preview", "image-preview", "video-", "image-generation",
	"text-to-speech", "lyria", "flux-",
}

func IsChatModel(id string) bool {
	lower := strings.ToLower(id)
	for _, kw := range nonChatKeywords {
		if strings.Contains(lower, kw) {
			return false
		}
	}
	return true
}

func (l *Live) FixAll(ctx context.Context) ([]FixReport, error) {
	providers, err := l.db.ListProviders()
	if err != nil {
		return nil, err
	}
	reports := make([]FixReport, 0, len(providers))
	for i := range providers {
		p := &providers[i]
		if p.Status != "active" {
			continue
		}
		if p.BaseURL == "" {
			continue
		}
		rep := FixReport{ProviderID: p.ID}
		real, fetchErr := l.FetchRealModels(ctx, p)
		if fetchErr != nil {
			rep.FetchError = fetchErr.Error()
			reports = append(reports, rep)
			continue
		}
		realSet := make(map[string]bool, len(real))
		for _, r := range real {
			realSet[r] = true
		}
		dbModels, err := l.db.ListModelsByProvider(p.ID)
		if err != nil {
			rep.FetchError = err.Error()
			reports = append(reports, rep)
			continue
		}
		dbSet := make(map[string]bool, len(dbModels))
		for _, m := range dbModels {
			dbSet[stripProviderPrefix(p.ID, m.ID)] = true
		}
		now := time.Now().Unix()
		// 1. Mark phantoms as error (only if not already error).
		for _, m := range dbModels {
			bare := stripProviderPrefix(p.ID, m.ID)
			if realSet[bare] {
				continue
			}
			if m.Status == "error" {
				continue
			}
			updated := m
			updated.Status = "error"
			updated.ErrorMessage = "not_in_real_catalog: live audit " + time.Now().UTC().Format("2006-01-02")
			updated.LastTested = now
			if err := l.db.UpsertModel(&updated); err != nil {
				rep.FetchError = err.Error()
				continue
			}
			rep.PhantomFixed++
		}
		// 2. Insert missing free chat models as untested.
		for r := range realSet {
			if dbSet[r] {
				continue
			}
			if !IsChatModel(r) {
				rep.Skipped++
				continue
			}
			bare := r
			prefix := p.ID + "/"
			if strings.HasPrefix(bare, prefix) {
				bare = bare[len(prefix):]
			}
			if err := l.db.UpsertModel(&models.Model{
				ID:          p.ID + "/" + bare,
				ProviderID:  p.ID,
				DisplayName: bare,
				Source:      "live_audit",
				Status:      "untested",
				LastTested:  now,
			}); err != nil {
				rep.FetchError = err.Error()
				continue
			}
			rep.MissingAdded++
		}
		reports = append(reports, rep)
	}
	return reports, nil
}
