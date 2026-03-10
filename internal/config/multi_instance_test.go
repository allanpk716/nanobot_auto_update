package config

import (
	"testing"
	"time"
)

func TestValidateModeCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "only nanobot config (legacy mode)",
			config: Config{
				Cron: "0 3 * * *",
				Nanobot: NanobotConfig{
					Port:           18790,
					StartupTimeout: 30 * time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "only instances config (new mode)",
			config: Config{
				Cron: "0 3 * * *",
				Instances: []InstanceConfig{
					{
						Name:           "instance1",
						Port:           18790,
						StartCommand:   "nanobot.exe",
						StartupTimeout: 30 * time.Second,
					},
				},
			},
			expectError: false,
		},
		{
			name: "both nanobot and instances (conflict)",
			config: Config{
				Cron: "0 3 * * *",
				Nanobot: NanobotConfig{
					Port:           18790,
					StartupTimeout: 30 * time.Second,
				},
				Instances: []InstanceConfig{
					{
						Name:           "instance1",
						Port:           18791,
						StartCommand:   "nanobot.exe",
						StartupTimeout: 30 * time.Second,
					},
				},
			},
			expectError: true,
			errorMsg:    "不能同时使用 'nanobot' section 和 'instances' 数组",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateModeCompatibility()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateUniqueNames(t *testing.T) {
	tests := []struct {
		name        string
		instances   []InstanceConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "unique names",
			instances: []InstanceConfig{
				{Name: "instance1", Port: 18790, StartCommand: "cmd1"},
				{Name: "instance2", Port: 18791, StartCommand: "cmd2"},
				{Name: "instance3", Port: 18792, StartCommand: "cmd3"},
			},
			expectError: false,
		},
		{
			name: "duplicate names",
			instances: []InstanceConfig{
				{Name: "instance1", Port: 18790, StartCommand: "cmd1"},
				{Name: "instance2", Port: 18791, StartCommand: "cmd2"},
				{Name: "instance1", Port: 18792, StartCommand: "cmd3"},
			},
			expectError: true,
			errorMsg:    "实例名称重复",
		},
		{
			name: "multiple duplicates",
			instances: []InstanceConfig{
				{Name: "instance1", Port: 18790, StartCommand: "cmd1"},
				{Name: "instance1", Port: 18791, StartCommand: "cmd2"},
				{Name: "instance2", Port: 18792, StartCommand: "cmd3"},
				{Name: "instance2", Port: 18793, StartCommand: "cmd4"},
			},
			expectError: true,
			errorMsg:    "实例名称重复",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUniqueNames(tt.instances)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestValidateUniquePorts(t *testing.T) {
	tests := []struct {
		name        string
		instances   []InstanceConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "unique ports",
			instances: []InstanceConfig{
				{Name: "instance1", Port: 18790, StartCommand: "cmd1"},
				{Name: "instance2", Port: 18791, StartCommand: "cmd2"},
				{Name: "instance3", Port: 18792, StartCommand: "cmd3"},
			},
			expectError: false,
		},
		{
			name: "duplicate ports",
			instances: []InstanceConfig{
				{Name: "instance1", Port: 18790, StartCommand: "cmd1"},
				{Name: "instance2", Port: 18791, StartCommand: "cmd2"},
				{Name: "instance3", Port: 18790, StartCommand: "cmd3"},
			},
			expectError: true,
			errorMsg:    "端口重复",
		},
		{
			name: "multiple duplicates",
			instances: []InstanceConfig{
				{Name: "instance1", Port: 18790, StartCommand: "cmd1"},
				{Name: "instance2", Port: 18790, StartCommand: "cmd2"},
				{Name: "instance3", Port: 18791, StartCommand: "cmd3"},
				{Name: "instance4", Port: 18791, StartCommand: "cmd4"},
			},
			expectError: true,
			errorMsg:    "端口重复",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUniquePorts(tt.instances)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}

func TestConfigValidateWithInstances(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorContains []string
	}{
		{
			name: "valid multi-instance config",
			config: Config{
				Cron: "0 3 * * *",
				Instances: []InstanceConfig{
					{
						Name:           "instance1",
						Port:           18790,
						StartCommand:   "nanobot1.exe",
						StartupTimeout: 30 * time.Second,
					},
					{
						Name:           "instance2",
						Port:           18791,
						StartCommand:   "nanobot2.exe",
						StartupTimeout: 30 * time.Second,
					},
				},
			},
			expectError: false,
		},
		{
			name: "legacy config without instances",
			config: Config{
				Cron: "0 3 * * *",
				Nanobot: NanobotConfig{
					Port:           18790,
					StartupTimeout: 30 * time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "duplicate names and ports",
			config: Config{
				Cron: "0 3 * * *",
				Instances: []InstanceConfig{
					{
						Name:           "instance1",
						Port:           18790,
						StartCommand:   "nanobot1.exe",
						StartupTimeout: 30 * time.Second,
					},
					{
						Name:           "instance1",
						Port:           18790,
						StartCommand:   "nanobot2.exe",
						StartupTimeout: 30 * time.Second,
					},
				},
			},
			expectError: true,
			errorContains: []string{"实例名称重复", "端口重复"},
		},
		{
			name: "invalid instance config",
			config: Config{
				Cron: "0 3 * * *",
				Instances: []InstanceConfig{
					{
						Name:           "",
						Port:           18790,
						StartCommand:   "nanobot.exe",
						StartupTimeout: 30 * time.Second,
					},
				},
			},
			expectError: true,
			errorContains: []string{"缺少必填字段"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				for _, msg := range tt.errorContains {
					if !contains(err.Error(), msg) {
						t.Errorf("expected error containing %q, got %q", msg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			}
		})
	}
}
