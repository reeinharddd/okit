package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/reeinharddd/okit/internal/db"
	"github.com/reeinharddd/okit/pkg/models"
)

type Service struct {
	db db.DBInterface
}

func New(database db.DBInterface) *Service {
	return &Service{db: database}
}

type taskDef struct {
	Description    string
	MinContext     int
	NeedsFC        bool
	NeedsVision    bool
	NeedsReasoning bool
	NeedsAudio     bool
	Priority       string
	MaxCostPerCall float64
}

var taskDefs = map[string]taskDef{
	"coding_complex": {"Complex coding tasks with function calling", 100000, true, false, false, false, "quality", 0.06},
	"coding_fast":    {"Fast coding with function calling", 50000, true, false, false, false, "speed", 0.03},
	"reasoning":      {"Deep reasoning and analysis", 100000, false, false, true, false, "quality", 0.05},
	"vision":         {"Vision and image understanding", 100000, false, true, false, false, "quality", 0.08},
	"long_context":   {"Long context research and analysis", 500000, false, false, false, false, "cost", 0.10},
	"fastest":        {"Simple tasks, maximum speed", 0, false, false, false, false, "speed", 0.01},
}

type candidate struct {
	model models.Model
	score float64
}

type scoredCandidate struct {
	ID    string  `json:"id"`
	Score float64 `json:"score"`
}

func (s *Service) SelectBestModel(taskType string, budget models.BudgetConfig, shadow bool) (*models.RoutingRule, error) {
	def, ok := taskDefs[taskType]
	if !ok {
		return nil, fmt.Errorf("unknown task type: %s", taskType)
	}

	allModels, err := s.db.ListModels()
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	cands := make([]candidate, 0, len(allModels))

	for _, m := range allModels {
		if !eligibleModel(m, def, budget) {
			continue
		}
		cands = append(cands, candidate{model: m, score: scoreModel(m, def)})
	}

	if len(cands) == 0 {
		return nil, fmt.Errorf("no suitable model found for task: %s", taskType)
	}

	sort.SliceStable(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		if cands[i].model.LatencyP50Ms != cands[j].model.LatencyP50Ms {
			return cands[i].model.LatencyP50Ms < cands[j].model.LatencyP50Ms
		}
		if cands[i].model.PricingPrompt+cands[i].model.PricingCompletion != cands[j].model.PricingPrompt+cands[j].model.PricingCompletion {
			return modelCost(cands[i].model) < modelCost(cands[j].model)
		}
		return cands[i].model.ID < cands[j].model.ID
	})

	best := cands[0].model
	candidateSummary := make([]scoredCandidate, 0, len(cands))
	for _, cand := range cands {
		candidateSummary = append(candidateSummary, scoredCandidate{ID: cand.model.ID, Score: cand.score})
	}
	summaryJSON, _ := json.Marshal(candidateSummary)
	fallbacks := make([]string, 0, 2)
	for i := 1; i < len(cands) && len(fallbacks) < 2; i++ {
		if cands[i].model.ID != best.ID {
			fallbacks = append(fallbacks, cands[i].model.ID)
		}
	}
	if len(fallbacks) == 0 {
		fallbacks = []string{best.ID}
	}

	fallbackJSON, _ := json.Marshal(fallbacks)
	reason := fmt.Sprintf("selected %s for %s", best.ID, taskType)
	if budget.PreferredTier != "" {
		reason += fmt.Sprintf(" with budget=%s", budget.PreferredTier)
	}

	rule := &models.RoutingRule{
		TaskKey:        taskType,
		Description:    def.Description,
		MinContext:     def.MinContext,
		NeedsFC:        def.NeedsFC,
		NeedsVision:    def.NeedsVision,
		MaxCostPerCall: def.MaxCostPerCall,
		CurrentModelID: best.ID,
		FallbackIDs:    string(fallbackJSON),
		LastAssigned:   time.Now().Unix(),
	}
	if err := s.db.InsertRoutingEvent(&models.RoutingEvent{
		TaskKey:       taskType,
		SelectedModel: best.ID,
		Candidates:    string(summaryJSON),
		Reason:        reason,
		Shadow:        shadow,
	}); err != nil {
		return nil, fmt.Errorf("log routing event: %w", err)
	}
	return rule, nil
}

func scoreModel(m models.Model, def taskDef) float64 {
	score := 0.0

	ctxScore := math.Min(float64(m.ContextWindow)/100000, 5.0)
	score += ctxScore * 2

	if m.FunctionCalling {
		score += 3
	}
	if m.Vision {
		score += 2
	}
	if m.Reasoning {
		score += 2
	}
	if m.Audio {
		score += 1
	}
	if m.OCR {
		score += 1
	}

	latencyScore := 0.0
	if m.LatencyP50Ms > 0 {
		latencyScore = math.Max(0, 5-m.LatencyP50Ms/500)
	} else {
		latencyScore = 1
	}
	score += latencyScore

	isPaid := m.Tier == "paid"
	if isPaid {
		score -= 2
	}

	if m.ProviderID == "mistral" && (m.ContextWindow >= 200000) {
		score += 2
	}

	if circuitOpen(m) {
		score -= 8
	} else if m.FailCount > 0 {
		score -= math.Min(float64(m.FailCount), 3)
	}

	cost := modelCost(m)
	if cost > 0 {
		score += math.Max(0, 3-cost*25)
	}

	switch def.Priority {
	case "speed":
		score += latencyScore * 2
	case "quality":
		score += ctxScore * 1.5
		if m.FunctionCalling {
			score += 2
		}
		if def.NeedsReasoning && m.Reasoning {
			score += 3
			if m.Interleaved != "" {
				score += 1
			}
		}
		if def.NeedsVision && m.Vision {
			score += 2
		}
	case "cost":
		if !isPaid {
			score += 3
		}
	}

	if def.MaxCostPerCall > 0 && cost > def.MaxCostPerCall {
		score -= 50
	}

	return score
}

func modelCost(m models.Model) float64 {
	if m.PricingPrompt == 0 && m.PricingCompletion == 0 && m.PricingCacheRead == 0 {
		return 0
	}
	return m.PricingPrompt + m.PricingCompletion + (m.PricingCacheRead * 0.1)
}

func circuitOpen(m models.Model) bool {
	if m.FailCount < 3 {
		return false
	}
	if m.LastTested == 0 {
		return true
	}
	return time.Since(time.Unix(m.LastTested, 0)) < 24*time.Hour
}

func eligibleModel(m models.Model, def taskDef, budget models.BudgetConfig) bool {
	if m.Status != "active" {
		return false
	}
	if budget.PreferredTier == "free_only" && m.Tier == "paid" {
		return false
	}
	if m.ContextWindow < def.MinContext {
		return false
	}
	if def.NeedsFC && !m.FunctionCalling {
		return false
	}
	if def.NeedsVision && !m.Vision {
		return false
	}
	if def.NeedsReasoning && !m.Reasoning {
		return false
	}
	if def.NeedsAudio && !m.Audio {
		return false
	}
	if circuitOpen(m) {
		return false
	}
	if def.MaxCostPerCall > 0 && modelCost(m) > def.MaxCostPerCall {
		return false
	}
	return true
}

func (s *Service) ReassignAll(ctx context.Context, shadow bool) error {
	budget, err := s.db.GetBudget()
	if err != nil {
		budget = &models.BudgetConfig{ID: "default", DailyGlobalUSD: 0.50, PreferredTier: "free_only"}
	}

	for taskType := range taskDefs {
		rule, err := s.SelectBestModel(taskType, *budget, shadow)
		if err != nil {
			fmt.Printf("  Warning: no model for %s: %v\n", taskType, err)
			continue
		}
		if shadow {
			fmt.Printf("  Shadow: %s -> %s\n", taskType, rule.CurrentModelID)
			continue
		}
		if err := s.db.UpsertRoutingRule(rule); err != nil {
			return fmt.Errorf("upsert rule %s: %w", taskType, err)
		}
		fmt.Printf("  Route: %s → %s\n", taskType, rule.CurrentModelID)
	}
	return nil
}

func FormatCandidateSummary(raw string) string {
	if raw == "" || raw == "[]" {
		return "-"
	}
	var cands []scoredCandidate
	if err := json.Unmarshal([]byte(raw), &cands); err != nil {
		return raw
	}
	parts := make([]string, 0, len(cands))
	for _, cand := range cands {
		parts = append(parts, fmt.Sprintf("%s=%.2f", cand.ID, cand.Score))
	}
	return strings.Join(parts, ", ")
}
