package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

type Service struct {
	db      db.DBInterface
	workers int
}

func New(db db.DBInterface, workers int) *Service {
	if workers <= 0 {
		workers = 5
	}
	return &Service{db: db, workers: workers}
}

type testResult struct {
	ModelID    string  `json:"model_id"`
	Status     string  `json:"status"`
	LatencyMs  float64 `json:"latency_ms"`
	FC         bool    `json:"function_calling"`
	Vision     bool    `json:"vision"`
	Context    int     `json:"context_window"`
	ErrorMsg   string  `json:"error_message,omitempty"`
}

func (s *Service) Run(ctx context.Context, full bool) error {
	providers, err := s.db.ListProviders()
	if err != nil {
		return fmt.Errorf("list providers: %w", err)
	}

	g, ctx := errgroup.WithContext(ctx)
	sem := semaphore.NewWeighted(int64(s.workers))

	for _, prov := range providers {
		if prov.Status != "active" {
			continue
		}
		apiKey := os.Getenv(prov.KeyEnv)
		if apiKey == "" {
			continue
		}

		prov := prov
		g.Go(func() error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			defer sem.Release(1)

			models, err := s.db.ListModelsByProvider(prov.ID)
			if err != nil {
				return fmt.Errorf("list models for %s: %w", prov.ID, err)
			}

			for _, m := range models {
				if !full && m.Status == "active" {
					continue
				}
				m := m
				g.Go(func() error {
					if err := sem.Acquire(ctx, 1); err != nil {
						return err
					}
					defer sem.Release(1)

					result := s.testModel(ctx, prov, m)
					result.ID = m.ID
					result.LastTested = time.Now().Unix()

					if err := s.db.UpsertModel(&result); err != nil {
						return fmt.Errorf("upsert %s: %w", m.ID, err)
					}

				status := result.Status
				if status == "" {
					status = "active"
				}
				if status == "error" && result.ErrorMessage != "" {
					fmt.Printf("  %s: %s (%.0fms) — %s\n", m.ID, status, result.LatencyP50Ms, result.ErrorMessage)
				} else {
					fmt.Printf("  %s: %s (%.0fms)\n", m.ID, status, result.LatencyP50Ms)
				}
					return nil
				})
			}
			return nil
		})
	}
	return g.Wait()
}

func (s *Service) testModel(ctx context.Context, prov models.Provider, m models.Model) models.Model {
	baseURL := prov.BaseURL
	if baseURL == "" {
		return models.Model{
			ID:         m.ID,
			ProviderID: prov.ID,
			Status:     "error",
			ErrorMessage: "no base_url configured for provider",
			Source:     "audited",
			LastTested: time.Now().Unix(),
		}
	}
	apiKey := os.Getenv(prov.KeyEnv)
	endpoint := strings.TrimRight(baseURL, "/") + "/chat/completions"

	client := &http.Client{Timeout: 30 * time.Second}

	latencies := make([]float64, 0, 3)
	var fc bool
	var errMsg string
	ctxVal := 131072

	isReasoning := strings.Contains(m.ID, "o3") || strings.Contains(m.ID, "o4") || strings.Contains(m.ID, "deepseek-r1")

	for i := 0; i < 3; i++ {
		body := map[string]interface{}{
			"model": m.DisplayName,
			"messages": []map[string]string{
				{"role": "user", "content": "Say OK"},
			},
		}
		if isReasoning {
			body["max_completion_tokens"] = 20
		} else {
			body["max_tokens"] = 5
		}

		jsonBody, _ := json.Marshal(body)
		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
		if err != nil {
			errMsg = fmt.Sprintf("create req: %v", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		start := time.Now()
		resp, err := client.Do(req)
		latency := time.Since(start).Seconds() * 1000
		latencies = append(latencies, latency)

		if err != nil {
			errMsg = fmt.Sprintf("request: %v", err)
			continue
		}
		if resp.StatusCode == 429 {
			errMsg = "rate_limited"
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}
		if resp.StatusCode != 200 {
			errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		errMsg = ""
		fc = true
		break
	}

	if strings.Contains(m.ID, "codestral") || strings.Contains(m.ID, "devstral") {
		ctxVal = 256000
	} else if strings.Contains(m.ID, "gemini") || strings.Contains(m.ID, "gemma") {
		ctxVal = 1048576
	} else if strings.Contains(m.ID, "gpt-4.1") || strings.Contains(m.ID, "gpt-5") {
		ctxVal = 1048576
	} else if strings.Contains(m.ID, "deepseek") {
		ctxVal = 1048576
	}

	var p50 float64
	if len(latencies) > 0 {
		p50 = latencies[len(latencies)/2]
	}

	status := "active"
	tier := "free"
	if fc && strings.Contains(m.ID, "claude") {
		tier = "paid"
	}
	if errMsg != "" {
		status = "error"
		tier = "unknown"
	}

	return models.Model{
		ID:              m.ID,
		ProviderID:      prov.ID,
		DisplayName:     m.DisplayName,
		ContextWindow:   ctxVal,
		FunctionCalling: fc,
		Status:          status,
		ErrorMessage:    errMsg,
		LatencyP50Ms:    p50,
		Tier:            tier,
		Source:          "audited",
		LastTested:      time.Now().Unix(),
	}
}
