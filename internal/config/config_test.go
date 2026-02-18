package config

import (
	"testing"
	"time"
)

func TestNewConfigDefaults(t *testing.T) {
	cfg := New()

	// Verify cron default
	if cfg.Cron != "0 3 * * *" {
		t.Errorf("expected default Cron '0 3 * * *', got %q", cfg.Cron)
	}

	// Verify nanobot defaults
	if cfg.Nanobot.Port != 18790 {
		t.Errorf("expected default Port 18790, got %d", cfg.Nanobot.Port)
	}

	if cfg.Nanobot.StartupTimeout != 30*time.Second {
		t.Errorf("expected default StartupTimeout 30s, got %v", cfg.Nanobot.StartupTimeout)
	}
}

func TestValidateCronValid(t *testing.T) {
	validExpressions := []string{
		"0 3 * * *",
		"*/5 * * * *",
		"0 0 1 1 *",
		"30 4 * * 1-5",
		"0 0 * * *",
	}

	for _, expr := range validExpressions {
		if err := ValidateCron(expr); err != nil {
			t.Errorf("expected cron expression %q to be valid, got error: %v", expr, err)
		}
	}
}

func TestValidateCronInvalid(t *testing.T) {
	invalidExpressions := []string{
		"invalid",
		"* * * *",    // missing field (only 4)
		"",           // empty
		"* * * * * *", // too many fields (6)
		"60 * * * *",  // invalid minute (>59)
		"* 25 * * *",  // invalid hour (>23)
	}

	for _, expr := range invalidExpressions {
		if err := ValidateCron(expr); err == nil {
			t.Errorf("expected cron expression %q to be invalid, but it passed validation", expr)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := New()
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config to pass validation, got error: %v", err)
		}
	})

	t.Run("invalid cron", func(t *testing.T) {
		cfg := New()
		cfg.Cron = "invalid"
		if err := cfg.Validate(); err == nil {
			t.Error("expected config with invalid cron to fail validation")
		}
	})
}
