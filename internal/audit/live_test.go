package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/reeinharddd/okit/pkg/models"
)

type fakeDB struct {
	providers []models.Provider
	models    map[string][]models.Model
	upserted  []models.Model
	failOn    map[string]bool
}

func (f *fakeDB) ListProviders() ([]models.Provider, error) { return f.providers, nil }
func (f *fakeDB) ListModelsByProvider(pid string) ([]models.Model, error) {
	return f.models[pid], nil
}
func (f *fakeDB) UpsertModel(m *models.Model) error {
	if f.failOn[m.ID] {
		return fmt.Errorf("forced fail on %s", m.ID)
	}
	f.upserted = append(f.upserted, *m)
	return nil
}

func TestStripProviderPrefix(t *testing.T) {
	tests := []struct {
		prov, mid, want string
	}{
		{"groq", "groq/llama-3.3-70b-versatile", "llama-3.3-70b-versatile"},
		{"groq", "llama-3.3-70b-versatile", "llama-3.3-70b-versatile"},
		{"openrouter", "openrouter/anthropic/claude-opus-4.8", "anthropic/claude-opus-4.8"},
		{"nvidia", "nvidia/meta/llama-3.1-8b-instruct", "meta/llama-3.1-8b-instruct"},
	}
	for _, tt := range tests {
		got := stripProviderPrefix(tt.prov, tt.mid)
		if got != tt.want {
			t.Errorf("stripProviderPrefix(%q, %q) = %q, want %q", tt.prov, tt.mid, got, tt.want)
		}
	}
}

func TestLive_FetchRealModels_GroqShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.Error(w, "wrong path: "+r.URL.Path, 404)
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "no auth", 401)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":"llama-3.3-70b-versatile"},
			{"id":"openai/gpt-oss-120b"},
			{"id":"meta-llama/llama-4-scout-17b-16e-instruct"}
		]}`))
	}))
	defer srv.Close()

	prov := &models.Provider{
		ID: "groq", BaseURL: srv.URL, KeyEnv: "GROQ_API_KEY",
	}
	os.Setenv("GROQ_API_KEY", "test-key")
	defer os.Unsetenv("GROQ_API_KEY")

	fdb := &fakeDB{}
	l := NewLive(fdb, 1)
	ids, err := l.FetchRealModels(context.Background(), prov)
	if err != nil {
		t.Fatalf("FetchRealModels: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 models, got %d: %v", len(ids), ids)
	}
	if ids[1] != "openai/gpt-oss-120b" {
		t.Errorf("expected openai/gpt-oss-120b, got %q", ids[1])
	}
}

func TestLive_DiffProvider_IdentifiesPhantomAndMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.Error(w, "wrong", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":"llama-3.3-70b-versatile"},
			{"id":"openai/gpt-oss-120b"}
		]}`))
	}))
	defer srv.Close()

	prov := &models.Provider{
		ID: "groq", BaseURL: srv.URL, KeyEnv: "GROQ_API_KEY",
	}
	os.Setenv("GROQ_API_KEY", "test-key")
	defer os.Unsetenv("GROQ_API_KEY")

	fdb := &fakeDB{
		models: map[string][]models.Model{
			"groq": {
				{ID: "groq/llama-3.3-70b-versatile", ProviderID: "groq", DisplayName: "llama-3.3-70b-versatile", Status: "active"},
				{ID: "groq/llama-3.1-8b-instant", ProviderID: "groq", DisplayName: "llama-3.1-8b-instant", Status: "active"},
				{ID: "groq/compound", ProviderID: "groq", DisplayName: "compound", Status: "active"},
			},
		},
	}
	l := NewLive(fdb, 1)
	res, err := l.DiffProvider(context.Background(), prov)
	if err != nil {
		t.Fatalf("DiffProvider: %v", err)
	}
	if res.RealCount != 2 {
		t.Errorf("RealCount = %d, want 2", res.RealCount)
	}
	if res.DBCount != 3 {
		t.Errorf("DBCount = %d, want 3", res.DBCount)
	}
	wantPhantom := map[string]bool{"llama-3.1-8b-instant": true, "compound": true}
	if len(res.Phantom) != 2 {
		t.Errorf("Phantom = %v, want 2 entries", res.Phantom)
	}
	for _, p := range res.Phantom {
		if !wantPhantom[p] {
			t.Errorf("unexpected phantom: %q", p)
		}
	}
	if len(res.Missing) != 1 || res.Missing[0] != "openai/gpt-oss-120b" {
		t.Errorf("Missing = %v, want [openai/gpt-oss-120b]", res.Missing)
	}
}

func TestLive_SmokeOne_OKAnd404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		if model == "ok-model" {
			w.Write([]byte(`{"id":"x","choices":[{"message":{"content":"hi"}}]}`))
			return
		}
		if model == "missing-model" {
			w.WriteHeader(404)
			w.Write([]byte(`{"error":{"message":"model not found"}}`))
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()

	prov := &models.Provider{
		ID: "groq", BaseURL: srv.URL, KeyEnv: "GROQ_API_KEY",
	}
	l := NewLive(&fakeDB{}, 1)

	okRes := l.SmokeOne(context.Background(), prov, "ok-model", "test")
	if okRes.Status != "ok" {
		t.Errorf("ok-model status = %q, want ok", okRes.Status)
	}
	if okRes.LatencyMs < 0 {
		t.Errorf("ok-model latency negative: %f", okRes.LatencyMs)
	}

	notFound := l.SmokeOne(context.Background(), prov, "missing-model", "test")
	if notFound.Status != "not_found" {
		t.Errorf("missing-model status = %q, want not_found", notFound.Status)
	}
	if !strings.Contains(notFound.ErrorMsg, "model not found") {
		t.Errorf("missing-model err = %q, want contains 'model not found'", notFound.ErrorMsg)
	}
}

func TestLive_FixAll_MarksPhantomsAndInsertsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.Error(w, "wrong", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[
			{"id":"llama-3.3-70b-versatile"},
			{"id":"openai/gpt-oss-120b"},
			{"id":"meta-llama/llama-4-scout-17b-16e-instruct"},
			{"id":"openai/whisper-large-v3"}
		]}`))
	}))
	defer srv.Close()

	os.Setenv("GROQ_API_KEY", "test-key")
	defer os.Unsetenv("GROQ_API_KEY")

	fdb := &fakeDB{
		providers: []models.Provider{
			{ID: "groq", Status: "active", BaseURL: srv.URL, KeyEnv: "GROQ_API_KEY"},
		},
		models: map[string][]models.Model{
			"groq": {
				{ID: "groq/llama-3.3-70b-versatile", ProviderID: "groq", DisplayName: "llama-3.3-70b-versatile", Status: "active"},
				{ID: "groq/llama-3.1-8b-instant", ProviderID: "groq", DisplayName: "llama-3.1-8b-instant", Status: "active"},
				{ID: "groq/old-deleted-model", ProviderID: "groq", DisplayName: "old-deleted-model", Status: "error"},
			},
		},
	}
	l := NewLive(fdb, 1)
	reports, err := l.FixAll(context.Background())
	if err != nil {
		t.Fatalf("FixAll: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}
	rep := reports[0]
	if rep.ProviderID != "groq" {
		t.Errorf("ProviderID = %q, want groq", rep.ProviderID)
	}
	if rep.PhantomFixed != 1 {
		t.Errorf("PhantomFixed = %d, want 1 (only the active one)", rep.PhantomFixed)
	}
	if rep.MissingAdded != 2 {
		t.Errorf("MissingAdded = %d, want 2 (gpt-oss + llama-4-scout, whisper is non-chat)", rep.MissingAdded)
	}
	if rep.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1 (whisper)", rep.Skipped)
	}
	phantomMarked := false
	for _, u := range fdb.upserted {
		if u.ID == "groq/llama-3.1-8b-instant" {
			phantomMarked = true
			if u.Status != "error" {
				t.Errorf("phantom status = %q, want error", u.Status)
			}
			if !strings.Contains(u.ErrorMessage, "not_in_real_catalog") {
				t.Errorf("phantom error_message = %q, want contains not_in_real_catalog", u.ErrorMessage)
			}
		}
	}
	if !phantomMarked {
		t.Errorf("expected phantom model to be upserted with error status")
	}
	alreadyErrorSkipped := true
	for _, u := range fdb.upserted {
		if u.ID == "groq/old-deleted-model" {
			alreadyErrorSkipped = false
		}
	}
	if !alreadyErrorSkipped {
		t.Errorf("already-error model should NOT be re-upserted")
	}
	for _, u := range fdb.upserted {
		if strings.HasPrefix(u.ID, "groq/groq/") {
			t.Errorf("double-prefix detected: %s", u.ID)
		}
	}
}
