---
phase: 11-configuration-extension
plan: 01a
type: tdd
wave: 0
depends_on: []
files_modified: []
autonomous: true
requirements: [CONF-01, CONF-02, CONF-03, CONF-04, CONF-05, CONF-06, SEC-03]
user_setup: []
must_haves:
  truths:
    - "所有新增配置项有完整的单元测试脚手架"
    - "测试存根可以独立运行，不依赖外部资源"
    - "测试存根遵循现有项目测试模式"
  artifacts:
    - path: "internal/config/api_test.go"
      provides: "APIConfig 验证测试脚手架"
      min_lines: 50
    - path: "internal/config/monitor_test.go"
      provides: "MonitorConfig 验证测试脚手架"
      min_lines: 50
  key_links:
    - from: "internal/config/api_test.go"
      to: "internal/config/api.go"
      via: "test import"
      pattern: "package config_test"
    - from: "internal/config/monitor_test.go"
      to: "internal/config/monitor.go"
      via: "test import"
      pattern: "package config_test"
---

<objective>
创建 Wave 0 单元测试脚手架（第一部分），为 APIConfig 和 MonitorConfig 验证提供测试基础设施

Purpose: 确保所有配置验证逻辑有完整的测试覆盖，遵循 TDD 原则
Output: 单元测试文件（存根状态）
</objective>

<execution_context>
@C:/Users/allan716/.claude/get-shit-done/workflows/execute-plan.md
@C:/Users/allan716/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/11-configuration-extension/11-RESEARCH.md
@.planning/phases/11-configuration-extension/11-VALIDATION.md

## 现有测试模式

From internal/config/config_test.go:
- 使用标准 Go testing 框架
- 测试函数命名: TestXxx
- 子测试使用 t.Run()
- 使用 Load() 加载 YAML 文件进行集成测试
- 错误消息包含字符串检查 (strings.Contains)

From internal/config/instance_test.go:
- 单元测试直接调用 Validate() 方法
- 边界值测试 (如 Port: 0, 65535, 65536)
- Duration 类型验证

## 测试要求

基于 11-VALIDATION.md Wave 0 Requirements:
- 测试必须覆盖 CONF-01~06 和 SEC-03
- 每个配置项需要有效和无效用例
- Bearer Token 必须测试长度验证 (SEC-03)
- Duration 配置必须测试最小值
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: Create APIConfig test scaffolding</name>
  <files>internal/config/api_test.go</files>
  <read_first>
    - internal/config/config_test.go (现有测试模式)
    - internal/config/instance_test.go (单元测试示例)
    - .planning/phases/11-configuration-extension/11-RESEARCH.md (APIConfig 规范)
  </read_first>
  <behavior>
    - Test 1: Valid APIConfig with all fields (Port: 8080, BearerToken: 32+ chars, Timeout: 30s)
    - Test 2: Invalid port (0, 65536)
    - Test 3: Bearer Token too short (1-31 chars)
    - Test 4: Timeout too short (< 5s)
    - Test 5: Empty Bearer Token fails validation
  </behavior>
  <action>
    Create internal/config/api_test.go with test stubs for APIConfig validation:

    ```go
    package config

    import (
        "strings"
        "testing"
        "time"
    )

    func TestAPIConfigValidate(t *testing.T) {
        t.Run("valid config", func(t *testing.T) {
            // TODO: Implement after api.go created
            t.Skip("Waiting for api.go implementation")
        })

        t.Run("invalid port", func(t *testing.T) {
            // TODO: Port validation tests
            t.Skip("Waiting for api.go implementation")
        })

        t.Run("bearer token too short", func(t *testing.T) {
            // TODO: SEC-03 token length validation
            t.Skip("Waiting for api.go implementation")
        })

        t.Run("timeout too short", func(t *testing.T) {
            // TODO: Timeout minimum validation
            t.Skip("Waiting for api.go implementation")
        })
    }
    ```

    Each test should:
    - Use t.Run() for subtests
    - Create APIConfig instances with specific values
    - Call Validate() method
    - Use strings.Contains to verify error messages
    - Follow naming convention from existing config_test.go
  </action>
  <verify>
    <automated>go test -v ./internal/config/ -run TestAPIConfigValidate</automated>
  </verify>
  <done>
    - File internal/config/api_test.go exists
    - Contains 5 test stubs (valid, invalid port, token too short, timeout too short, empty token)
    - All tests currently skip (waiting for implementation)
    - Tests can run: `go test -v ./internal/config/ -run TestAPIConfigValidate`
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Create MonitorConfig test scaffolding</name>
  <files>internal/config/monitor_test.go</files>
  <read_first>
    - internal/config/config_test.go (现有测试模式)
    - internal/config/instance_test.go (Duration 验证示例)
    - .planning/phases/11-configuration-extension/11-RESEARCH.md (MonitorConfig 规范)
  </read_first>
  <behavior>
    - Test 1: Valid MonitorConfig (Interval: 15m, Timeout: 10s)
    - Test 2: Interval too short (< 1m)
    - Test 3: Timeout too short (< 1s)
    - Test 4: Duration parsing from string format
  </behavior>
  <action>
    Create internal/config/monitor_test.go with test stubs for MonitorConfig validation:

    ```go
    package config

    import (
        "strings"
        "testing"
        "time"
    )

    func TestMonitorConfigValidate(t *testing.T) {
        t.Run("valid config", func(t *testing.T) {
            // TODO: Implement after monitor.go created
            t.Skip("Waiting for monitor.go implementation")
        })

        t.Run("interval too short", func(t *testing.T) {
            // TODO: Interval minimum validation (1 minute)
            t.Skip("Waiting for monitor.go implementation")
        })

        t.Run("timeout too short", func(t *testing.T) {
            // TODO: Timeout minimum validation (1 second)
            t.Skip("Waiting for monitor.go implementation")
        })
    }
    ```

    Follow same pattern as Task 1 for test structure.
  </action>
  <verify>
    <automated>go test -v ./internal/config/ -run TestMonitorConfigValidate</automated>
  </verify>
  <done>
    - File internal/config/monitor_test.go exists
    - Contains 3 test stubs (valid, interval too short, timeout too short)
    - All tests currently skip (waiting for implementation)
    - Tests can run: `go test -v ./internal/config/ -run TestMonitorConfigValidate`
  </done>
</task>

</tasks>

<verification>
Wave 0a 完成标准:
- 单元测试脚手架文件已创建
- 测试存根可以运行（即使跳过）
- 遵循现有测试模式
</verification>

<success_criteria>
1. internal/config/api_test.go 存在，包含 5 个测试存根
2. internal/config/monitor_test.go 存在，包含 3 个测试存根
3. 所有测试可以运行: `go test -v ./internal/config/`
4. 无测试失败（全部跳过是正常的）
</success_criteria>

<output>
After completion, create `.planning/phases/11-configuration-extension/11-01a-SUMMARY.md`
</output>
