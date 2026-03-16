package config

import (
	"testing"
	"time"
)

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
		name          string
		config        Config
		expectError   bool
		errorContains []string
	}{
		{
			name: "valid multi-instance config",
			config: Config{
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
				API: APIConfig{
					Port:        8080,
					BearerToken: "this-is-a-secure-token-with-at-least-32-characters",
					Timeout:     30 * time.Second,
				},
				Monitor: MonitorConfig{
					Interval: 15 * time.Minute,
					Timeout:  10 * time.Second,
				},
			},
			expectError: false,
		},
		{
			name: "no instances (error)",
			config: Config{
				API: APIConfig{
					Port:        8080,
					BearerToken: "this-is-a-secure-token-with-at-least-32-characters",
					Timeout:     30 * time.Second,
				},
				Monitor: MonitorConfig{
					Interval: 15 * time.Minute,
					Timeout:  10 * time.Second,
				},
			},
			expectError:   true,
			errorContains: []string{"at least one instance"},
		},
		{
			name: "duplicate names and ports",
			config: Config{
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
				API: APIConfig{
					Port:        8080,
					BearerToken: "this-is-a-secure-token-with-at-least-32-characters",
					Timeout:     30 * time.Second,
				},
				Monitor: MonitorConfig{
					Interval: 15 * time.Minute,
					Timeout:  10 * time.Second,
				},
			},
			expectError: true,
			errorContains: []string{"实例名称重复", "端口重复"},
		},
		{
			name: "invalid instance config",
			config: Config{
				Instances: []InstanceConfig{
					{
						Name:           "",
						Port:           18790,
						StartCommand:   "nanobot.exe",
						StartupTimeout: 30 * time.Second,
					},
				},
				API: APIConfig{
					Port:        8080,
					BearerToken: "this-is-a-secure-token-with-at-least-32-characters",
					Timeout:     30 * time.Second,
				},
				Monitor: MonitorConfig{
					Interval: 15 * time.Minute,
					Timeout:  10 * time.Second,
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
