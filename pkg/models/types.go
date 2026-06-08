package models

type Provider struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	BaseURL         string `json:"api_base,omitempty"`
	CatalogURL      string `json:"catalog_url,omitempty"`
	KeyEnv          string `json:"key_env,omitempty"`
	IsFree          bool   `json:"is_free"`
	Enabled         bool   `json:"enabled"`
	Source          string `json:"source"`
	Status          string `json:"status"`
	Priority        int    `json:"priority"`
	TimeoutMs       int    `json:"timeout_ms,omitempty"`
	HeaderTimeoutMs int    `json:"header_timeout_ms,omitempty"`
	ChunkTimeoutMs  int    `json:"chunk_timeout_ms,omitempty"`
	EnterpriseURL   string `json:"enterprise_url,omitempty"`
	SetCacheKey     bool   `json:"set_cache_key,omitempty"`
	APIPackage      string `json:"api_package,omitempty"`
	EnvList         string `json:"env_list,omitempty"` // JSON array
	LastSynced      int64  `json:"last_synced,omitempty"`
}

type Model struct {
	ID                string  `json:"id"`
	ProviderID        string  `json:"provider_id"`
	DisplayName       string  `json:"display_name,omitempty"`
	Description       string  `json:"description,omitempty"`
	ContextWindow     int     `json:"context_window"`
	MaxOutput         int     `json:"max_output,omitempty"`
	FunctionCalling   bool    `json:"function_calling"`
	Vision            bool    `json:"vision"`
	Reasoning         bool    `json:"reasoning,omitempty"`
	Audio             bool    `json:"audio,omitempty"`
	OCR               bool    `json:"ocr,omitempty"`
	FineTuning        bool    `json:"fine_tuning,omitempty"`
	Classification    bool    `json:"classification,omitempty"`
	Moderation        bool    `json:"moderation,omitempty"`
	Streaming         bool    `json:"streaming"`
	StructuredOutput  bool    `json:"structured_outputs"`
	LatencyP50Ms      float64 `json:"latency_p50_ms,omitempty"`
	LatencyP95Ms      float64 `json:"latency_p95_ms,omitempty"`
	TokensPerSec      float64 `json:"tokens_per_sec,omitempty"`
	PricingPrompt     float64 `json:"pricing_prompt,omitempty"`
	PricingCompletion float64 `json:"pricing_completion,omitempty"`
	PricingCacheRead  float64 `json:"pricing_cache_read,omitempty"`
	PricingCacheWrite float64 `json:"pricing_cache_write,omitempty"`
	DefaultTemp       float64 `json:"default_temperature,omitempty"`
	Tier              string  `json:"tier"`
	Status            string  `json:"status"`
	ErrorMessage      string  `json:"error_message,omitempty"`
	Tags              string  `json:"tags,omitempty"`
	Aliases           string  `json:"aliases,omitempty"`
	Family            string  `json:"family,omitempty"`
	ReleaseDate       string  `json:"release_date,omitempty"`
	Deprecation       string  `json:"deprecation,omitempty"`
	Interleaved       string  `json:"interleaved,omitempty"` // JSON: true | {"field":"reasoning_content"}
	Experimental      bool    `json:"experimental,omitempty"`
	ModalitiesInput   string  `json:"modalities_input,omitempty"`
	ModalitiesOutput  string  `json:"modalities_output,omitempty"`
	CreatedTimestamp  int64   `json:"created_timestamp,omitempty"`
	OwnedBy           string  `json:"owned_by,omitempty"`
	LastTested        int64   `json:"last_tested,omitempty"`
	TestCount         int     `json:"test_count,omitempty"`
	FailCount         int     `json:"fail_count,omitempty"`
	Source            string  `json:"source"`
}

type Agent struct {
	ID              string  `json:"id"`
	TaskType        string  `json:"task_type,omitempty"`
	Description     string  `json:"description,omitempty"`
	CurrentModelID  string  `json:"current_model_id,omitempty"`
	FallbackIDs     string  `json:"fallback_ids,omitempty"` // JSON array
	PromptFile      string  `json:"prompt_file,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	MaxSteps        int     `json:"max_steps,omitempty"`
	Permission      string  `json:"permission,omitempty"` // JSON
	Color           string  `json:"color,omitempty"`
	Mode            string  `json:"mode"` // subagent, primary, all
	Hidden          bool    `json:"hidden,omitempty"`
	Status          string  `json:"status"`
	Source          string  `json:"source"`
}

type Skill struct {
	ID         string `json:"id"`
	Source     string `json:"source"`
	SourcePath string `json:"source_path,omitempty"`
	TargetPath string `json:"target_path,omitempty"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Hash       string `json:"hash,omitempty"`
	LastSynced int64  `json:"last_synced,omitempty"`
}

type MCPServer struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // local, remote
	Command  string `json:"command,omitempty"` // JSON array
	URL      string `json:"url,omitempty"`
	Enabled  bool   `json:"enabled"`
	EnvVars  string `json:"env_vars,omitempty"` // JSON
	Timeout  int    `json:"timeout_ms,omitempty"`
	Source   string `json:"source,omitempty"`
}

type LSPServer struct {
	ID             string `json:"id"`
	Command        string `json:"command"` // JSON array
	Extensions     string `json:"extensions,omitempty"` // JSON array
	Env            string `json:"env,omitempty"` // JSON
	Initialization string `json:"initialization,omitempty"` // JSON
	Disabled       bool   `json:"disabled,omitempty"`
}

type Command struct {
	ID          string `json:"id"`
	Template    string `json:"template"`
	Description string `json:"description,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Model       string `json:"model,omitempty"`
	Subtask     bool   `json:"subtask,omitempty"`
	Source      string `json:"source,omitempty"`
	Status      string `json:"status"`
}

type RoutingRule struct {
	TaskKey        string `json:"task_key"`
	Description    string `json:"description,omitempty"`
	MinContext     int    `json:"min_context,omitempty"`
	NeedsFC        bool   `json:"needs_fc"`
	NeedsVision    bool   `json:"needs_vision"`
	MaxCostPerCall float64 `json:"max_cost_per_call,omitempty"`
	CurrentModelID string `json:"current_model_id,omitempty"`
	FallbackIDs    string `json:"fallback_ids,omitempty"`
	LastAssigned   int64  `json:"last_assigned,omitempty"`
}

type RoutingEvent struct {
	ID            int64  `json:"id"`
	TaskKey       string `json:"task_key"`
	SelectedModel string `json:"selected_model,omitempty"`
	Candidates    string `json:"candidates,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Shadow        bool   `json:"shadow"`
	CreatedAt     string `json:"created_at,omitempty"`
}

type ModelProfile struct {
	ModelID       string  `json:"model_id"`
	RealContext   int     `json:"real_context,omitempty"`
	MaxOutput     int     `json:"max_output,omitempty"`
	SupportsStream bool   `json:"supports_stream"`
	SupportsSO    bool    `json:"supports_so"`
	StreamTPS     float64 `json:"stream_tps,omitempty"`
	ProfiledAt    int64   `json:"profiled_at,omitempty"`
}

type BudgetConfig struct {
	ID             string  `json:"id"`
	DailyGlobalUSD float64 `json:"daily_global_usd"`
	PreferredTier  string  `json:"preferred_tier"` // free_only, budget, quality
	UpdatedAt      string  `json:"updated_at,omitempty"`
}

type SyncLog struct {
	ID         int64  `json:"id"`
	Phase      string `json:"phase"`
	Status     string `json:"status"`
	Details    string `json:"details,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type ExecLog struct {
	ID         int64  `json:"id"`
	Agent      string `json:"agent,omitempty"`
	Model      string `json:"model,omitempty"`
	Task       string `json:"task,omitempty"`
	TokensIn   int    `json:"tokens_in,omitempty"`
	TokensOut  int    `json:"tokens_out,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
}

type Snapshot struct {
	ID        int64  `json:"id"`
	Hash      string `json:"hash"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at,omitempty"`
}

type Source struct {
	ID        string `json:"id"`
	RemoteURL string `json:"remote_url"`
	LocalPath string `json:"local_path"`
	Commit    string `json:"commit,omitempty"`
	Status    string `json:"status"`
	LastSynced int64 `json:"last_synced,omitempty"`
}

type SourceItem struct {
	ID         string `json:"id"`
	SourceID   string `json:"source_id"`
	Type       string `json:"type"`
	SourcePath string `json:"source_path,omitempty"`
	TargetPath string `json:"target_path,omitempty"`
	Hash       string `json:"hash,omitempty"`
	Status     string `json:"status"`
}

type ConfigFragment struct {
	ID         string `json:"id"`
	ConfigType string `json:"config_type"`
	Content    string `json:"content"`
	Source     string `json:"source,omitempty"`
	Hash       string `json:"hash,omitempty"`
	CreatedAt  string `json:"created_at,omitempty"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}
