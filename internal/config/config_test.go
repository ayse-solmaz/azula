package config

import "testing"

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
