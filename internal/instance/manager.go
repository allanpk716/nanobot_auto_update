package instance

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/HQGroup/nanobot-auto-updater/internal/config"
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
