package config

import (
	"strings"
	"testing"
	"time"
)

func TestNewConfigDefaults(t *testing.T) {
	cfg := New()

	// Verify cron default
	if cfg.Cron != "0 3 * * *" {
		t.Errorf("expected default Cron '0 3 * * *', got %q", cfg.Cron)
	}

	// Note: Nanobot defaults (Port, StartupTimeout) are now set in Validate()
	// when using legacy mode, not in New()
	// This allows proper mode detection for multi-instance support
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

// Integration tests for multi-instance configuration

func TestLoadInstancesYAML(t *testing.T) {
	cfg, err := Load("../../testutil/testdata/config/instances_valid.yaml")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify loaded data
	if len(cfg.Instances) != 2 {
		t.Errorf("expected 2 instances, got %d", len(cfg.Instances))
	}

	// Verify first instance
	if cfg.Instances[0].Name != "instance1" {
		t.Errorf("expected instance1, got %s", cfg.Instances[0].Name)
	}
	if cfg.Instances[0].Port != 18790 {
		t.Errorf("expected port 18790, got %d", cfg.Instances[0].Port)
	}
	if cfg.Instances[0].StartCommand != "C:\\path\\to\\nanobot.exe" {
		t.Errorf("expected start_command 'C:\\path\\to\\nanobot.exe', got %s", cfg.Instances[0].StartCommand)
	}
	if cfg.Instances[0].StartupTimeout != 30*time.Second {
		t.Errorf("expected startup_timeout 30s, got %v", cfg.Instances[0].StartupTimeout)
	}

	// Verify second instance
	if cfg.Instances[1].Name != "instance2" {
		t.Errorf("expected instance2, got %s", cfg.Instances[1].Name)
	}
	if cfg.Instances[1].Port != 18791 {
		t.Errorf("expected port 18791, got %d", cfg.Instances[1].Port)
	}
	if cfg.Instances[1].StartupTimeout != 0 {
		t.Errorf("expected startup_timeout 0 (not set), got %v", cfg.Instances[1].StartupTimeout)
	}

	// Verify cron
	if cfg.Cron != "0 3 * * *" {
		t.Errorf("expected cron '0 3 * * *', got %s", cfg.Cron)
	}

	// Verify pushover
	if cfg.Pushover.ApiToken != "test_token" {
		t.Errorf("expected api_token 'test_token', got %s", cfg.Pushover.ApiToken)
	}
	if cfg.Pushover.UserKey != "test_user" {
		t.Errorf("expected user_key 'test_user', got %s", cfg.Pushover.UserKey)
	}
}

func TestLoadLegacyConfig(t *testing.T) {
	cfg, err := Load("../../testutil/testdata/config/legacy_v1.yaml")
	if err != nil {
		t.Fatalf("Load legacy config failed: %v", err)
	}

	// Verify legacy config still works
	if cfg.Nanobot.Port != 18790 {
		t.Errorf("expected port 18790, got %d", cfg.Nanobot.Port)
	}
	if cfg.Nanobot.StartupTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.Nanobot.StartupTimeout)
	}
	if cfg.Nanobot.RepoPath != "C:\\Users\\test\\.nanobot\\repo" {
		t.Errorf("expected repo_path 'C:\\Users\\test\\.nanobot\\repo', got %s", cfg.Nanobot.RepoPath)
	}

	// Verify no instances loaded
	if len(cfg.Instances) != 0 {
		t.Errorf("expected no instances, got %d", len(cfg.Instances))
	}
}

func TestLoadDuplicateName(t *testing.T) {
	_, err := Load("../../testutil/testdata/config/instances_duplicate_name.yaml")
	if err == nil {
		t.Fatal("expected error for duplicate name, got nil")
	}

	// Verify error message contains "实例名称重复"
	if !strings.Contains(err.Error(), "实例名称重复") {
		t.Errorf("error message should contain '实例名称重复', got: %v", err)
	}
}

func TestLoadDuplicatePort(t *testing.T) {
	_, err := Load("../../testutil/testdata/config/instances_duplicate_port.yaml")
	if err == nil {
		t.Fatal("expected error for duplicate port, got nil")
	}

	// Verify error message contains "端口重复"
	if !strings.Contains(err.Error(), "端口重复") {
		t.Errorf("error message should contain '端口重复', got: %v", err)
	}
}

func TestLoadMixedMode(t *testing.T) {
	_, err := Load("../../testutil/testdata/config/mixed_mode.yaml")
	if err == nil {
		t.Fatal("expected error for mixed mode, got nil")
	}

	// Verify error message contains "不能同时使用"
	if !strings.Contains(err.Error(), "不能同时使用") {
		t.Errorf("error message should contain '不能同时使用', got: %v", err)
	}
}

