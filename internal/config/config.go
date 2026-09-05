package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

var errWeakJWT = errors.New("JWT_SECRET must be set to a strong value in production")

type Config struct {
	MongoURI             string
	APIPort              string
	JWTSecret            string
	JWTExpiry            time.Duration
	MFAIssuer            string
	MCPFileRoot          string
	SamplePipeline       string
	OllamaBaseURL        string
	OpenAIKey            string
	ModelAProvider       string
	ModelAName           string
	ModelBName           string
	ModelBProvider       string
	ModelCProvider       string
	ModelCName           string
	WorkerSlots          int
	RequestTimeout       time.Duration
	WebURL               string
	FreeTierMaxProjects  int
	FinetuneDir          string
	FinetuneDemo         bool
	SMTPHost             string
	SMTPPort             string
	SMTPUser             string
	SMTPPass             string
	SMTPFrom             string
	MailOutboxDir        string
	DeviceOTPEcho        bool
	StripeSecretKey      string
	StripeWebhookSecret  string
	StripePriceID        string
	BillingDemo          bool
	OIDCIssuer           string
	OIDCClientID         string
	OIDCClientSecret     string
	OIDCRedirectURL      string
	FreeTierMaxInvs      int
	Env                  string
	GraphQLPlayground    bool
	KillSwitch           bool
	ForceCouncilOnSample bool
}

func Load() Config {
	_ = godotenv.Load()
	timeout, err := time.ParseDuration(getenv("LLM_REQUEST_TIMEOUT", "60s"))
	if err != nil {
		timeout = 60 * time.Second
	}
	slots, err := strconv.Atoi(getenv("LLM_WORKER_SLOTS", "5"))
	if err != nil || slots < 1 {
		slots = 5
	}
	jwtExpiry, err := time.ParseDuration(getenv("JWT_EXPIRY", "8h"))
	if err != nil {
		jwtExpiry = 8 * time.Hour
	}
	maxProjects, err := strconv.Atoi(getenv("FREE_TIER_MAX_PROJECTS", "3"))
	if err != nil || maxProjects < 1 {
		maxProjects = 3
	}
	maxInvs, err := strconv.Atoi(getenv("FREE_TIER_MAX_INVESTIGATIONS", "10"))
	if err != nil || maxInvs < 1 {
		maxInvs = 10
	}
	stripeKey := os.Getenv("STRIPE_SECRET_KEY")
	billingDemo := getenv("BILLING_DEMO", "")
	demo := billingDemo == "true" || (billingDemo == "" && stripeKey == "")
	env := getenv("AZULA_ENV", "development")
	playground := getenv("AZULA_GRAPHQL_PLAYGROUND", "")
	showPlayground := playground == "true" || (playground == "" && env != "production")
	return Config{
		MongoURI:             getenv("MONGODB_URI", "mongodb://localhost:27017/azula"),
		APIPort:              getenv("API_PORT", "8080"),
		JWTSecret:            getenv("JWT_SECRET", "change-me-in-production"),
		JWTExpiry:            jwtExpiry,
		MFAIssuer:            getenv("MFA_ISSUER", "Azula"),
		MCPFileRoot:          getenv("MCP_FILE_ROOT", "./uploads"),
		SamplePipeline:       getenv("SAMPLE_PIPELINE_DIR", "./samples/broken-pipeline"),
		OllamaBaseURL:        getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OpenAIKey:            os.Getenv("OPENAI_API_KEY"),
		ModelAProvider:       getenv("LLM_MODEL_A_PROVIDER", "ollama"),
		ModelAName:           getenv("LLM_MODEL_A_NAME", "qwen2.5:1.5b"),
		ModelBProvider:       getenv("LLM_MODEL_B_PROVIDER", "ollama"),
		ModelBName:           getenv("LLM_MODEL_B_NAME", "azula-incident"),
		ModelCProvider:       getenv("LLM_MODEL_C_PROVIDER", "openai"),
		ModelCName:           getenv("LLM_MODEL_C_NAME", "gpt-4o-mini"),
		WorkerSlots:          slots,
		RequestTimeout:       timeout,
		WebURL:               getenv("WEB_URL", "http://localhost:3001"),
		FreeTierMaxProjects:  maxProjects,
		FinetuneDir:          getenv("FINETUNE_JOB_DIR", "./data/finetune"),
		FinetuneDemo:         getenv("FINETUNE_DEMO_MODE", "false") == "true",
		SMTPHost:             os.Getenv("SMTP_HOST"),
		SMTPPort:             getenv("SMTP_PORT", "587"),
		SMTPUser:             os.Getenv("SMTP_USER"),
		SMTPPass:             os.Getenv("SMTP_PASS"),
		SMTPFrom:             getenv("SMTP_FROM", "azula@localhost"),
		MailOutboxDir:        getenv("MAIL_OUTBOX_DIR", "./data/outbox"),
		DeviceOTPEcho:        getenv("DEVICE_OTP_ECHO", "false") == "true",
		StripeSecretKey:      stripeKey,
		StripeWebhookSecret:  os.Getenv("STRIPE_WEBHOOK_SECRET"),
		StripePriceID:        getenv("STRIPE_PRICE_ID", ""),
		BillingDemo:          demo,
		OIDCIssuer:           os.Getenv("OIDC_ISSUER"),
		OIDCClientID:         os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret:     os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:      getenv("OIDC_REDIRECT_URL", ""),
		FreeTierMaxInvs:      maxInvs,
		Env:                  env,
		GraphQLPlayground:    showPlayground,
		KillSwitch:           getenv("AZULA_KILL_SWITCH", "false") == "true",
		ForceCouncilOnSample: getenv("AZULA_FORCE_COUNCIL_SAMPLE", "true") != "false",
	}
}

func (c Config) Production() bool {
	return strings.EqualFold(c.Env, "production")
}

func (c Config) Validate() error {
	if !c.Production() {
		return nil
	}
	if c.JWTSecret == "" || c.JWTSecret == "change-me-in-production" {
		return errWeakJWT
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
