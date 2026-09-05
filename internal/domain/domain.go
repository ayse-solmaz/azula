package domain

import (
	"errors"
	"time"
)

var (
	ErrUnauthorized         = errors.New("unauthorized")
	ErrNotFound             = errors.New("not found")
	ErrEmailTaken           = errors.New("email already registered")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrInvalidInput         = errors.New("invalid input")
	ErrTierLimit            = errors.New("project limit reached for current tier")
	ErrForbiddenFile        = errors.New("file type not allowed")
	ErrPathTraversal        = errors.New("invalid file path")
	ErrFileTooLarge         = errors.New("file exceeds 50MB limit")
	ErrMFARequired          = errors.New("mfa code required")
	ErrMFAInvalid           = errors.New("invalid mfa code")
	ErrMFANotPending        = errors.New("mfa enrollment not started")
	ErrBusy                 = errors.New("all investigation workers are busy")
	ErrUnknownDevice        = errors.New("untrusted device")
	ErrDeviceOTP            = errors.New("invalid or expired device verification code")
	ErrVersionNotFound      = errors.New("file version not found")
	ErrForbidden            = errors.New("role cannot perform this action")
	ErrOrgConflict          = errors.New("user already belongs to an organization")
	ErrProRequired          = errors.New("this feature requires Pro")
	ErrInvestigationLimit   = errors.New("monthly investigation limit reached")
	ErrGitNotConnected      = errors.New("git repository not connected")
	ErrSSONotConfigured     = errors.New("sso is not configured")
	ErrBillingNotConfigured = errors.New("stripe is not configured")
	ErrAgentHalted          = errors.New("agent kill switch is on")
	ErrCancelled            = errors.New("investigation cancelled")
	ErrAccountDisabled      = errors.New("account is disabled")
)

type Tier string

const (
	TierFree       Tier = "free"
	TierPro        Tier = "pro"
	TierEnterprise Tier = "enterprise"
)

type TrustedDevice struct {
	DeviceID   string
	Name       string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type User struct {
	ID                   string
	Email                string
	PasswordHash         string
	Tier                 Tier
	MFAEnabled           bool
	MFASecret            string
	MFAPendingSecret     string
	TrustedDevices       []TrustedDevice
	PendingDeviceID      string
	PendingDeviceName    string
	PendingDeviceOTP     string
	PendingDeviceExp     time.Time
	OrgID                string
	OrgName              string
	OrgRole              string
	StripeCustomerID     string
	StripeSubscription   string
	SSOSubject           string
	DisplayName          string
	Disabled             bool
	PrefsVersion         int
	NotifyEmail          bool
	NotifyInvestigations bool
	NotifyMarketing      bool
	ShareUsage           bool
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type FileVersion struct {
	ProjectID string    `bson:"projectId"`
	FileName  string    `bson:"fileName"`
	Version   int       `bson:"version"`
	Path      string    `bson:"path"`
	MimeType  string    `bson:"mimeType"`
	CreatedAt time.Time `bson:"createdAt"`
}

type AuditLog struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"userId"`
	Action    string    `bson:"action"`
	Resource  string    `bson:"resource"`
	Metadata  string    `bson:"metadata"`
	CreatedAt time.Time `bson:"createdAt"`
}

type ConsentRecord struct {
	ID        string    `bson:"_id"`
	UserID    string    `bson:"userId"`
	Purpose   string    `bson:"purpose"`
	Accepted  bool      `bson:"accepted"`
	CreatedAt time.Time `bson:"createdAt"`
}

type FineTuneJob struct {
	ID          string    `bson:"_id"`
	WorkspaceID string    `bson:"workspaceId"`
	UserID      string    `bson:"userId"`
	Status      string    `bson:"status"`
	AdapterPath string    `bson:"adapterPath"`
	Error       string    `bson:"error,omitempty"`
	CreatedAt   time.Time `bson:"createdAt"`
	UpdatedAt   time.Time `bson:"updatedAt"`
}

type OrgMember struct {
	UserID string `bson:"userId,omitempty"`
	Email  string `bson:"email"`
	Role   string `bson:"role"` // admin | engineer | viewer
}

type Organization struct {
	ID        string      `bson:"_id"`
	Name      string      `bson:"name"`
	OwnerID   string      `bson:"ownerId"`
	Members   []OrgMember `bson:"members"`
	CreatedAt time.Time   `bson:"createdAt"`
}

type Workspace struct {
	ID        string
	Name      string
	OwnerID   string
	OrgID     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectFile struct {
	Name       string
	Path       string
	MimeType   string
	UploadedAt time.Time
}

type Project struct {
	ID          string
	WorkspaceID string
	Name        string
	IsSample    bool
	Files       []ProjectFile
	GitURL      string
	GitBranch   string
	GitHead     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Entitlements struct {
	Tier                      Tier
	MaxProjects               int
	MaxInvestigationsPerMonth int
	InvestigationsUsed        int
	DeepAnalysis              bool
	Council                   bool
	Generate                  bool
	Evaluate                  bool
	GitMCP                    bool
	ModelSelection            bool
	TeamWorkspace             bool
	BillingConfigured         bool
	SSOEnabled                bool
	DemoUpgrade               bool
}

type GitRepo struct {
	URL       string
	Branch    string
	Head      string
	Connected bool
}

type GitBlameLine struct {
	Line    int
	SHA     string
	Author  string
	Summary string
}

type GitCommit struct {
	SHA     string
	Author  string
	Date    string
	Message string
}

type Generation struct {
	ID              string    `bson:"_id"`
	ProjectID       string    `bson:"projectId"`
	WorkspaceID     string    `bson:"workspaceId"`
	UserID          string    `bson:"userId"`
	InvestigationID string    `bson:"investigationId,omitempty"`
	Prompt          string    `bson:"prompt"`
	FileName        string    `bson:"fileName"`
	RowCount        int       `bson:"rowCount"`
	SchemaNote      string    `bson:"schemaNote"`
	QualityNotes    string    `bson:"qualityNotes"`
	Confidence      float64   `bson:"confidence"`
	Status          string    `bson:"status"`
	Error           string    `bson:"error,omitempty"`
	CreatedAt       time.Time `bson:"createdAt"`
	UpdatedAt       time.Time `bson:"updatedAt"`
}

type MetricDelta struct {
	Name   string  `bson:"name"`
	Before float64 `bson:"before"`
	After  float64 `bson:"after"`
	Delta  float64 `bson:"delta"`
}

type Evaluation struct {
	ID              string        `bson:"_id"`
	ProjectID       string        `bson:"projectId"`
	WorkspaceID     string        `bson:"workspaceId"`
	UserID          string        `bson:"userId"`
	InvestigationID string        `bson:"investigationId,omitempty"`
	GenerationID    string        `bson:"generationId,omitempty"`
	Summary         string        `bson:"summary"`
	Recommendation  string        `bson:"recommendation"`
	Confidence      float64       `bson:"confidence"`
	Metrics         []MetricDelta `bson:"metrics"`
	Status          string        `bson:"status"`
	Error           string        `bson:"error,omitempty"`
	CreatedAt       time.Time     `bson:"createdAt"`
	UpdatedAt       time.Time     `bson:"updatedAt"`
}

type PlanStep struct {
	Order       int    `bson:"order"`
	Description string `bson:"description"`
	Status      string `bson:"status"`
}

type Evidence struct {
	File    string `bson:"file"`
	Lines   string `bson:"lines"`
	Excerpt string `bson:"excerpt"`
}

type FastResult struct {
	Summary      string  `bson:"summary"`
	IncidentType string  `bson:"incidentType"`
	Confidence   float64 `bson:"confidence"`
}

type DeepResult struct {
	RootCause    string     `bson:"rootCause"`
	Confidence   float64    `bson:"confidence"`
	Evidence     []Evidence `bson:"evidence"`
	SuggestedFix string     `bson:"suggestedFix"`
}

type CouncilModel struct {
	Role       string     `bson:"role"`
	Hypothesis string     `bson:"hypothesis"`
	Confidence float64    `bson:"confidence"`
	Evidence   []Evidence `bson:"evidence"`
	Model      string     `bson:"model,omitempty"`
}

type Disagreement struct {
	Topic        string `bson:"topic"`
	Investigator string `bson:"investigator"`
	Challenger   string `bson:"challenger"`
}

type FinalJudgment struct {
	MostLikelyCause   string  `bson:"mostLikelyCause"`
	Confidence        float64 `bson:"confidence"`
	RecommendedAction string  `bson:"recommendedAction"`
}

type CouncilResult struct {
	Models          []CouncilModel `bson:"models"`
	Agreements      []string       `bson:"agreements"`
	Disagreements   []Disagreement `bson:"disagreements"`
	FinalJudgment   FinalJudgment  `bson:"finalJudgment"`
	Aggregation     string         `bson:"aggregation,omitempty"`
	NeedsReview     bool           `bson:"needsReview"`
	AggregationNote string         `bson:"aggregationNote,omitempty"`
}

const (
	StatusPending      = "pending"
	StatusFastClassify = "fast_classify"
	StatusDeepAnalyze  = "deep_analyze"
	StatusCouncil      = "council"
	StatusCompleted    = "completed"
	StatusFailed       = "failed"

	StepPending = "pending"
	StepRunning = "running"
	StepDone    = "done"
	StepFailed  = "failed"
	StepSkipped = "skipped"

	ExecutionLive     = "live"
	ExecutionFallback = "fallback"
	ExecutionMixed    = "mixed"
)

type Investigation struct {
	ID               string         `bson:"_id"`
	ProjectID        string         `bson:"projectId"`
	WorkspaceID      string         `bson:"workspaceId"`
	UserID           string         `bson:"userId"`
	Prompt           string         `bson:"prompt"`
	Status           string         `bson:"status"`
	Plan             []PlanStep     `bson:"plan"`
	FilesAccessed    []string       `bson:"filesAccessed,omitempty"`
	FastResult       *FastResult    `bson:"fastResult,omitempty"`
	DeepResult       *DeepResult    `bson:"deepResult,omitempty"`
	CouncilResult    *CouncilResult `bson:"councilResult,omitempty"`
	ErrorMessage     string         `bson:"errorMessage,omitempty"`
	ModelAName       string         `bson:"modelAName,omitempty"`
	ModelBName       string         `bson:"modelBName,omitempty"`
	ModelCName       string         `bson:"modelCName,omitempty"`
	EscalationReason string         `bson:"escalationReason,omitempty"`
	ExecutionMode    string         `bson:"executionMode,omitempty"`
	FallbackStages   []string       `bson:"fallbackStages,omitempty"`
	CreatedAt        time.Time      `bson:"createdAt"`
	UpdatedAt        time.Time      `bson:"updatedAt"`
}

type InvestigationContext struct {
	InvestigationID string
	ProjectID       string
	Prompt          string
	FileNames       []string
	FileContents    map[string]string
}

type ModelConfig struct {
	ID                 string    `bson:"_id"`
	WorkspaceID        string    `bson:"workspaceId"`
	ModelAProvider     string    `bson:"modelAProvider"`
	ModelAName         string    `bson:"modelAName"`
	ModelBProvider     string    `bson:"modelBProvider"`
	ModelBName         string    `bson:"modelBName"`
	ModelCProvider     string    `bson:"modelCProvider"`
	ModelCName         string    `bson:"modelCName"`
	Temperature        float64   `bson:"temperature"`
	MaxTokens          int       `bson:"maxTokens"`
	InvestigatorPrompt string    `bson:"investigatorPrompt"`
	ChallengerPrompt   string    `bson:"challengerPrompt"`
	JudgePrompt        string    `bson:"judgePrompt"`
	ActiveSlot         string    `bson:"activeSlot"`
	CreatedAt          time.Time `bson:"createdAt"`
	UpdatedAt          time.Time `bson:"updatedAt"`
}

type LLMOpsMetrics struct {
	TotalInvestigations int
	Completed           int
	Failed              int
	AvgConfidence       float64
	AvgDurationSec      float64
	WorkerSlots         int
	BusySlots           int
	ModelAName          string
	ModelBName          string
	OllamaReachable     bool
	OllamaModels        []string
	IncidentModelReady  bool
	AdapterOnDisk       bool
	TopCauses           []string
}
