package config

import (
	"os"
	"testing"
	"time"
)

func TestValidateProductionJWT(t *testing.T) {
	c := Config{Env: "production", JWTSecret: "change-me-in-production"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected weak jwt rejected")
	}
	c.JWTSecret = "not-the-default-secret-value"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCouncilFastDefaults(t *testing.T) {
	t.Setenv("AZULA_COUNCIL_FAST", "")
	t.Setenv("AZULA_COUNCIL_CONTEXT_CHARS", "")
	t.Setenv("AZULA_COUNCIL_AGENT_TIMEOUT", "")
	t.Setenv("AZULA_COUNCIL_MAX_TOKENS", "")
	c := Load()
	if !c.CouncilFast {
		t.Fatal("council fast should default on")
	}
	if c.CouncilContextChars != 8000 {
		t.Fatalf("context chars=%d", c.CouncilContextChars)
	}
	if c.CouncilAgentTimeout != 25*time.Second {
		t.Fatalf("agent timeout=%s", c.CouncilAgentTimeout)
	}
	if c.CouncilMaxTokens != 512 {
		t.Fatalf("max tokens=%d", c.CouncilMaxTokens)
	}
	t.Setenv("AZULA_COUNCIL_FAST", "false")
	if Load().CouncilFast {
		t.Fatal("explicit false must disable fast council")
	}
}

func TestDeviceOTPEchoDefaults(t *testing.T) {
	t.Setenv("DEVICE_OTP_ECHO", "")
	t.Setenv("AZULA_ENV", "development")
	if !Load().DeviceOTPEcho {
		t.Fatal("development should echo device OTP unless explicitly disabled")
	}
	t.Setenv("AZULA_ENV", "production")
	t.Setenv("JWT_SECRET", "not-the-default-secret-value")
	if Load().DeviceOTPEcho {
		t.Fatal("production must not echo device OTP by default")
	}
	t.Setenv("DEVICE_OTP_ECHO", "true")
	if !Load().DeviceOTPEcho {
		t.Fatal("explicit true must win")
	}
	t.Setenv("AZULA_ENV", "development")
	t.Setenv("DEVICE_OTP_ECHO", "false")
	if Load().DeviceOTPEcho {
		t.Fatal("explicit false must disable echo in development")
	}
	_ = os.Unsetenv("DEVICE_OTP_ECHO")
}
