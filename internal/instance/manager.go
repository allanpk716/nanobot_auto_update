package instance

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
	"github.com/HQGroup/nanobot-auto-updater/internal/logbuffer"
	"github.com/HQGroup/nanobot-auto-updater/internal/updater"
)

// InstanceManager 协调所有实例的停止→更新→启动流程
type InstanceManager struct {
	instances []*InstanceLifecycle
	logger    *slog.Logger
}

// NewInstanceManager 创建实例管理器
func NewInstanceManager(cfg *config.Config, baseLogger *slog.Logger) *InstanceManager {
	// 注入 component 字段
	logger := baseLogger.With("component", "instance-manager")

	// 为每个实例创建 InstanceLifecycle 包装器
	instances := make([]*InstanceLifecycle, 0, len(cfg.Instances))
	for _, instCfg := range cfg.Instances {
		lifecycle := NewInstanceLifecycle(instCfg, baseLogger)
		instances = append(instances, lifecycle)
	}

	return &InstanceManager{
		instances: instances,
		logger:    logger,
	}
}

// UpdateAll 执行完整更新流程: 停止所有 → UV 更新 → 启动所有
func (m *InstanceManager) UpdateAll(ctx context.Context) (*UpdateResult, error) {
	m.logger.Info("Starting full update process", "instance_count", len(m.instances))

	result := &UpdateResult{}

	// Phase 1: Stop all instances (graceful degradation)
	m.stopAll(ctx, result)

	// Phase 2: UV update (skip if any instance failed to stop)
	if len(result.StopFailed) > 0 {
		m.logger.Warn("Skipping UV update due to stop failures",
			"failed_count", len(result.StopFailed),
			"failed_instances", extractNames(result.StopFailed))
	} else {
		if err := m.performUpdate(ctx); err != nil {
			// Critical failure: UV update failed
			m.logger.Error("UV update failed, cannot start instances", "error", err)
			return result, fmt.Errorf("UV update failed: %w", err)
		}
	}

	// Phase 3: Start all instances (graceful degradation)
	m.startAll(ctx, result)

	// Log final result
	m.logger.Info("Update process completed",
		"stopped_success", len(result.Stopped),
		"stopped_failed", len(result.StopFailed),
		"started_success", len(result.Started),
		"started_failed", len(result.StartFailed))

	return result, nil
}

// stopAll 停止所有实例(串行执行,优雅降级)
func (m *InstanceManager) stopAll(ctx context.Context, result *UpdateResult) {
	m.logger.Info("Starting stop phase", "instance_count", len(m.instances))

	for _, inst := range m.instances {
		if err := inst.StopForUpdate(ctx); err != nil {
			m.logger.Error("Failed to stop instance",
				"error", err,
				"port", inst.config.Port)

			// 记录失败但不返回,继续停止其他实例
			// Type assertion: StopForUpdate always returns *InstanceError on error
			result.StopFailed = append(result.StopFailed, err.(*InstanceError))
		} else {
			result.Stopped = append(result.Stopped, inst.config.Name)
		}
	}

	m.logger.Info("Stop phase completed",
		"success", len(result.Stopped),
		"failed", len(result.StopFailed))
}

// startAll 启动所有实例(串行执行,优雅降级)
func (m *InstanceManager) startAll(ctx context.Context, result *UpdateResult) {
	m.logger.Info("Starting start phase", "instance_count", len(m.instances))

	for _, inst := range m.instances {
		if err := inst.StartAfterUpdate(ctx); err != nil {
			m.logger.Error("Failed to start instance",
				"error", err,
				"port", inst.config.Port)

			// 记录失败但不返回,继续启动其他实例
			// Type assertion: StartAfterUpdate always returns *InstanceError on error
			result.StartFailed = append(result.StartFailed, err.(*InstanceError))
		} else {
			result.Started = append(result.Started, inst.config.Name)
		}
	}

	m.logger.Info("Start phase completed",
		"success", len(result.Started),
		"failed", len(result.StartFailed))
}

// performUpdate 执行 UV 更新
func (m *InstanceManager) performUpdate(ctx context.Context) error {
	m.logger.Info("Starting UV update")

	// 复用 Phase 2 的 Updater 结构
	uvUpdater := updater.NewUpdater(m.logger)

	updateResult, err := uvUpdater.Update(ctx)
	if err != nil {
		m.logger.Error("UV update failed", "error", err)
		return err
	}

	m.logger.Info("UV update completed successfully", "result", updateResult)
	return nil
}

// extractNames 辅助函数,从 InstanceError 中提取实例名称
func extractNames(errs []*InstanceError) []string {
	names := make([]string, len(errs))
	for i, err := range errs {
		names[i] = err.InstanceName
	}
	return names
}

// GetLogBuffer returns the LogBuffer for the specified instance.
// INST-02: Used by HTTP API to access instance buffers for SSE streaming.
func (m *InstanceManager) GetLogBuffer(instanceName string) (*logbuffer.LogBuffer, error) {
	for _, inst := range m.instances {
		if inst.config.Name == instanceName {
			return inst.GetLogBuffer(), nil
		}
	}
	return nil, &InstanceError{
		InstanceName: instanceName,
		Operation:    "get_log_buffer",
		Err:          fmt.Errorf("instance not found"),
	}
}

// GetInstanceNames returns the names of all configured instances.
// UI-07: Used by Web UI to populate instance selector dropdown.
func (m *InstanceManager) GetInstanceNames() []string {
	names := make([]string, 0, len(m.instances))
	for _, inst := range m.instances {
		names = append(names, inst.config.Name)
	}
	return names
}

// GetInstanceConfigs returns the configurations of all instances.
// Used by status API to get instance name and port information.
func (m *InstanceManager) GetInstanceConfigs() []config.InstanceConfig {
	configs := make([]config.InstanceConfig, 0, len(m.instances))
	for _, inst := range m.instances {
		configs = append(configs, inst.config)
	}
	return configs
}

// AutoStartResult 包含自动启动流程的结果
// AUTOSTART-04: 汇总成功/失败/跳过的实例
type AutoStartResult struct {
	Started []string         `json:"started"` // 成功启动的实例名称
	Failed  []*InstanceError `json:"failed"`  // 启动失败的实例错误
	Skipped []string         `json:"skipped"` // 跳过自动启动的实例 (auto_start: false)
}

// StartAllInstances 启动所有配置为自动启动的实例(串行执行,优雅降级)
// AUTOSTART-02: 启动所有 auto_start=true 的实例
// AUTOSTART-03: 失败时继续启动其他实例
// AUTOSTART-04: 返回包含汇总信息的 AutoStartResult
func (m *InstanceManager) StartAllInstances(ctx context.Context) *AutoStartResult {
	m.logger.Info("开始自动启动阶段", "instance_count", len(m.instances))

	result := &AutoStartResult{}
	startTime := time.Now()

	for _, inst := range m.instances {
		// 通过 InstanceLifecycle 访问 InstanceConfig.ShouldAutoStart()
		if !inst.ShouldAutoStart() {
			m.logger.Info("跳过实例(auto_start=false)",
				"instance", inst.Name(),
				"port", inst.Port())
			result.Skipped = append(result.Skipped, inst.Name())
			continue
		}

		// 记录单个实例启动时间
		instStart := time.Now()
		m.logger.Info("正在启动实例",
			"instance", inst.Name(),
			"port", inst.Port())

		if err := inst.StartAfterUpdate(ctx); err != nil {
			duration := time.Since(instStart)
			m.logger.Error("启动实例失败",
				"error", err,
				"instance", inst.Name(),
				"port", inst.Port(),
				"duration", duration)
			// 记录失败但继续启动其他实例(优雅降级)
			result.Failed = append(result.Failed, err.(*InstanceError))
		} else {
			duration := time.Since(instStart)
			m.logger.Info("实例启动成功",
				"instance", inst.Name(),
				"port", inst.Port(),
				"duration", duration)
			result.Started = append(result.Started, inst.Name())
		}
	}

	// 记录汇总日志
	totalDuration := time.Since(startTime)
	failedNames := extractNames(result.Failed)

	if len(result.Failed) > 0 {
		m.logger.Warn("自动启动完成(部分失败)",
			"started", len(result.Started),
			"failed", len(result.Failed),
			"skipped", len(result.Skipped),
			"failed_instances", failedNames,
			"total_duration", totalDuration)
	} else {
		m.logger.Info("自动启动完成",
			"started", len(result.Started),
			"failed", len(result.Failed),
			"skipped", len(result.Skipped),
			"total_duration", totalDuration)
	}

	return result
}
