package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHealthCheckConfig_Validate_TooSmall(t *testing.T) {
	h := &HealthCheckConfig{Interval: 5 * time.Second}
	err := h.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "必须至少 10 秒")
}

func TestHealthCheckConfig_Validate_TooLarge(t *testing.T) {
	h := &HealthCheckConfig{Interval: 15 * time.Minute}
	err := h.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不能超过 10 分钟")
}

func TestHealthCheckConfig_Validate_Valid(t *testing.T) {
	h := &HealthCheckConfig{Interval: 1 * time.Minute}
	err := h.Validate()
	assert.NoError(t, err)
}

func TestConfig_HealthCheck_DefaultsAndValidation(t *testing.T) {
	cfg := &Config{}
	cfg.defaults()
	assert.Equal(t, 1*time.Minute, cfg.HealthCheck.Interval)
}
