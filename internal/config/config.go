package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	MongoURI            string
	APIPort             string
	JWTSecret           string
	JWTExpiry           time.Duration
	MFAIssuer           string
	MCPFileRoot         string
	SamplePipeline      string
	OllamaBaseURL       string
	OpenAIKey           string
	ModelAProvider      string
	ModelAName          string
	ModelBName          string
	ModelBProvider      string
	WorkerSlots         int
	RequestTimeout      time.Duration
	WebURL              string
	FreeTierMaxProjects int
	FinetuneDir         string
	FinetuneDemo        bool
	SMTPHost            string
	SMTPPort            string
	SMTPUser            string
	SMTPPass            string
	SMTPFrom            string
	MailOutboxDir       string
	DeviceOTPEcho       bool
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
	jwtExpiry, err := time.ParseDuration(getenv("JWT_EXPIRY", "12h"))
	if err != nil {
		jwtExpiry = 12 * time.Hour
	}
	maxProjects, err := strconv.Atoi(getenv("FREE_TIER_MAX_PROJECTS", "3"))
	if err != nil || maxProjects < 1 {
		maxProjects = 3
	}
	return Config{
		MongoURI:            getenv("MONGODB_URI", "mongodb://localhost:27017/azula"),
		APIPort:             getenv("API_PORT", "8080"),
		JWTSecret:           getenv("JWT_SECRET", "change-me-in-production"),
		JWTExpiry:           jwtExpiry,
		MFAIssuer:           getenv("MFA_ISSUER", "Azula"),
		MCPFileRoot:         getenv("MCP_FILE_ROOT", "./uploads"),
		SamplePipeline:      getenv("SAMPLE_PIPELINE_DIR", "./samples/broken-pipeline"),
		OllamaBaseURL:       getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OpenAIKey:           os.Getenv("OPENAI_API_KEY"),
		ModelAProvider:      getenv("LLM_MODEL_A_PROVIDER", "ollama"),
		ModelAName:          getenv("LLM_MODEL_A_NAME", "qwen2.5:1.5b"),
		ModelBProvider:      getenv("LLM_MODEL_B_PROVIDER", "ollama"),
		ModelBName:          getenv("LLM_MODEL_B_NAME", "azula-incident"),
		WorkerSlots:         slots,
		RequestTimeout:      timeout,
		WebURL:              getenv("WEB_URL", "http://localhost:3000"),
		FreeTierMaxProjects: maxProjects,
		FinetuneDir:         getenv("FINETUNE_JOB_DIR", "./data/finetune"),
		FinetuneDemo:        getenv("FINETUNE_DEMO_MODE", "false") == "true",
		SMTPHost:            os.Getenv("SMTP_HOST"),
		SMTPPort:            getenv("SMTP_PORT", "587"),
		SMTPUser:            os.Getenv("SMTP_USER"),
		SMTPPass:            os.Getenv("SMTP_PASS"),
		SMTPFrom:            getenv("SMTP_FROM", "azula@localhost"),
		MailOutboxDir:       getenv("MAIL_OUTBOX_DIR", "./data/outbox"),
		DeviceOTPEcho:       getenv("DEVICE_OTP_ECHO", "false") == "true",
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
