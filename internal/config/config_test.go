package config

import (
	"os"
	"testing"
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
