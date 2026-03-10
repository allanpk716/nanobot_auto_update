---
phase: 06-configuration-extension
plan: 01
subsystem: config
tags: [yaml, validation, mapstructure, tdd, backward-compatibility]

# Dependency graph
requires:
  - phase: 01-infrastructure
    provides: Config struct and validation framework
provides:
  - InstanceConfig struct for multi-instance definition
  - Multi-instance validation logic (unique names/ports)
  - Mode compatibility checking (legacy vs new)
affects: [instance-manager, lifecycle-controller]

# Tech tracking
tech-stack:
  added: []
  patterns: [mapstructure-tags, errors.Join-aggregation, O(n)-uniqueness-validation]

key-files:
  created:
    - internal/config/instance.go
    - internal/config/instance_test.go
    - internal/config/multi_instance_test.go
  modified:
    - internal/config/config.go

key-decisions:
  - "使用 mapstructure 标签而非 yaml 标签(viper 要求)"
  - "使用 map-based O(n) 算法验证唯一性而非嵌套循环"
  - "使用 errors.Join 聚合所有验证错误而非遇到首个错误即返回"
  - "模式互斥检测: legacy (nanobot section) vs new (instances array)"

patterns-established:
  - "TDD with RED-GREEN-REFACTOR cycles for config validation"
  - "Detailed error messages including instance names and positions"
  - "Backward compatibility: legacy configs without instances still work"

requirements-completed: [CONF-01, CONF-02, CONF-03]

# Metrics
duration: 3min
completed: 2026-03-10
---

# Phase 6 Plan 01: 配置扩展支持多实例 Summary

**扩展 Config 结构支持 YAML 数组定义多个 nanobot 实例,包含完整验证逻辑(name/port/startup_timeout)和向后兼容性,使用 TDD 模式实现**

## Performance

- **Duration:** 3 min
- **Started:** 2026-03-10T14:32:52Z
- **Completed:** 2026-03-10T14:35:52Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- 创建 InstanceConfig 结构体支持单实例配置(name/port/start_command/startup_timeout)
- 实现多实例模式下的名称和端口唯一性验证,使用 O(n) map-based 算法
- 实现模式互斥检测,防止同时使用 legacy 和 new 配置模式
- 使用 errors.Join 聚合所有验证错误,避免静默失败
- 保持向后兼容性,无 instances 字段的 v1.0 配置仍可正常加载

## Task Commits

每个任务通过 TDD 模式(RED-GREEN)原子提交:

1. **Task 1: 创建 InstanceConfig 结构体和验证方法**
   - `6d3ff5a` - test: add failing tests for InstanceConfig validation (RED)
   - `6c50676` - feat: implement InstanceConfig validation logic (GREEN)

2. **Task 2: 扩展 Config 结构和实现多实例验证**
   - `b0f476e` - test: add failing tests for multi-instance validation (RED)
   - `5adeb31` - feat: extend Config for multi-instance validation (GREEN)

**Plan metadata:** 稍后提交

_Note: TDD 任务包含多个提交 (test → feat)_

## Files Created/Modified
- `internal/config/instance.go` - InstanceConfig 结构定义和验证逻辑(必填字段检查、端口范围、timeout 最小值)
- `internal/config/instance_test.go` - InstanceConfig 验证测试用例(table-driven tests)
- `internal/config/multi_instance_test.go` - 多实例验证测试(模式互斥、名称唯一性、端口唯一性)
- `internal/config/config.go` - 扩展 Config 结构添加 Instances 字段,重构 Validate() 方法支持多实例验证

## Decisions Made
- 使用 mapstructure 标签而非 yaml 标签,因为 viper 使用 mapstructure 进行配置解析
- 唯一性验证使用 map-based O(n) 算法而非嵌套 O(n²) 循环,提升性能
- 使用 errors.Join 聚合所有验证错误,让用户一次性看到所有配置问题
- 错误消息包含实例名称和位置信息(从 1 开始计数),便于定位问题
- StartupTimeout 为 0 时不验证最小值,允许使用默认值

## Deviations from Plan

None - 计划完全按预期执行,所有 TDD 阶段通过,无需偏离。

## Issues Encountered
None - TDD 模式确保所有功能在实现前有测试覆盖,避免了集成问题。

## User Setup Required
None - 无外部服务配置,配置扩展为纯代码变更。

## Next Phase Readiness
配置扩展完成,为 Phase 6-02(实例管理器实现)提供了基础:
- InstanceConfig 结构体可用于 internal/instance 包
- Config.Load() 自动处理 instances 数组解析
- 验证逻辑确保配置正确性,减少运行时错误

下一步需要实现 internal/instance 包使用这些配置进行实例管理。

## Self-Check: PASSED

- ✅ internal/config/instance.go exists
- ✅ internal/config/config.go modified with Instances field
- ✅ Commit 6d3ff5a exists (Task 1 RED)
- ✅ Commit 5adeb31 exists (Task 2 GREEN)
- ✅ SUMMARY.md created

---
*Phase: 06-configuration-extension*
*Completed: 2026-03-10*
