---
status: investigating
trigger: "config-folder-clearing-on-copy-edit"
created: 2026-04-14T00:00:00.000Z
updated: 2026-04-14T01:00:00.000Z
---

## Current Focus

hypothesis: CloneConfig reads and writes to the SAME config path when source and copy share the same start_command, corrupting the original instance's config file (port + workspace overwritten). The "config folder cleared" symptom is caused by the workspace being redirected to a different directory, making skills appear lost.
test: Trace CloneConfig code path with same start_command for source and target
expecting: sourceConfigPath == targetConfigPath, confirming the overwrite
next_action: Form final diagnosis with all evidence

## Symptoms

expected: 停止、复制、编辑实例配置后，原有实例的config文件夹内容完整保留
actual: 某些情况下，原有实例的config文件夹内容被清空，skills文件丢失
errors: 无错误信息
reproduction: 停止某个nanobot实例 → 复制该实例 → 编辑复制的实例的配置和启动参数 → 原有实例的config文件夹内容被清空
timeline: 近期发现

## Eliminated

- hypothesis: HandlePut (nanobot config save) deletes config directory
  evidence: WriteConfig only calls os.MkdirAll + os.WriteFile. No RemoveAll or Remove calls. Safe.
  timestamp: 2026-04-14T00:30:00Z

- hypothesis: HandleUpdate (instance config edit) triggers config deletion
  evidence: HandleUpdate only modifies config.yaml via UpdateConfig. No callbacks to nanobot config manager. No file deletion.
  timestamp: 2026-04-14T00:30:00Z

- hypothesis: StopForUpdate or HandleStop deletes config files
  evidence: StopForUpdate only kills the process by PID. No file operations. HandleStop delegates to StopForUpdate.
  timestamp: 2026-04-14T00:30:00Z

- hypothesis: HandleGet (nanobot config GET) lazy creation deletes files
  evidence: CreateDefaultConfig only calls WriteConfig which creates directories and writes files. No deletion.
  timestamp: 2026-04-14T00:30:00Z

- hypothesis: Hot reload OnInstancesChange deletes config directories
  evidence: OnInstancesChange calls StopAllNanobots (process kill) and recreates InstanceManager. No os.RemoveAll on config directories. skipReload mechanism should suppress reload during API-initiated config changes.
  timestamp: 2026-04-14T00:45:00Z

- hypothesis: shouldSkipConfigCleanup has a bug allowing cleanup when it shouldn't
  evidence: shouldSkipConfigCleanup correctly iterates remaining instances after deletion and checks if any share the same config path. The deleted instance is already removed from config.yaml before this check runs, so the iteration is correct.
  timestamp: 2026-04-14T00:45:00Z

## Evidence

- timestamp: 2026-04-14T00:10:00Z
  checked: internal/nanobot/config_manager.go ParseConfigPath (lines 50-78)
  found: ParseConfigPath extracts --config value from start_command using regex. The instanceName parameter is completely UNUSED in path resolution. This means two instances with the same --config flag in their start_command will resolve to the SAME config path.
  implication: When copying an instance and keeping the same start_command, both source and target resolve to the same config file.

- timestamp: 2026-04-14T00:15:00Z
  checked: internal/nanobot/config_manager.go CloneConfig (lines 223-269)
  found: CloneConfig resolves sourceConfigPath and targetConfigPath independently. When both start_commands are identical (default copy behavior), sourceConfigPath == targetConfigPath. The function reads the source config, modifies gateway.port and agents.defaults.workspace, then writes to the target path. If they're the same path, this OVERWRITES the original instance's config with the copy's port and workspace values.
  implication: This is the ROOT CAUSE. The original instance's config.json is corrupted during the copy operation.

- timestamp: 2026-04-14T00:15:00Z
  checked: internal/nanobot/config_manager.go CloneConfig (lines 248-257)
  found: When source == target path, the config modifications are:
    - gateway.port: changed to the copy's port (e.g., original port + 1)
    - agents.defaults.workspace: changed from "~/.nanobot-{sourceName}" to "~/.nanobot-{targetName}"
  implication: After the copy, the original instance's config has wrong port AND wrong workspace. The nanobot process will look for skills in the copy's workspace directory, making them appear "lost".

- timestamp: 2026-04-14T00:20:00Z
  checked: internal/api/instance_config_handler.go HandleCopy (lines 462-593)
  found: At line 494, clonedInstance = *sourceIC deep-copies the source config INCLUDING StartCommand. At line 537-539, clonedInstance.StartCommand is only overridden if req.StartCommand is non-empty. At line 582, onCopyInstance is called with (sourceName, sourceStartCommand, clonedInstance.Name, clonedInstance.Port, clonedInstance.StartCommand). When the user doesn't modify start_command in the copy dialog, clonedInstance.StartCommand == sourceStartCommand, causing the same-path issue in CloneConfig.
  implication: The default copy behavior (user doesn't change start_command) triggers the config overwrite bug.

- timestamp: 2026-04-14T00:20:00Z
  checked: internal/web/static/home.js showCopyDialog (lines 340-440)
  found: At line 369, the copy dialog form is pre-populated with cfg.start_command (source's start_command). The user must manually change it. If they don't, the same start_command is sent to HandleCopy.
  implication: The UI design makes this bug easy to trigger -- most users won't change the start_command when copying.

- timestamp: 2026-04-14T00:25:00Z
  checked: internal/nanobot/config_manager.go resolveWorkspace (lines 38-44)
  found: resolveWorkspace uses instanceName to construct workspace path ("~/.nanobot-" + instanceName), while ParseConfigPath uses the --config flag value. These are INCONSISTENT: the workspace is always based on instance name, not the actual config path.
  implication: This is a design issue that makes the CloneConfig bug worse -- even if --config points to a custom path, the workspace is hardcoded to ~/.nanobot-{instanceName}.

- timestamp: 2026-04-14T00:25:00Z
  checked: internal/nanobot/config_manager_test.go
  found: No test covers the case where source and target start_commands are identical (same --config path). All tests use different source and target paths.
  implication: The bug is untested.

- timestamp: 2026-04-14T00:35:00Z
  checked: All os.RemoveAll and os.Remove calls in the codebase
  found: The ONLY os.RemoveAll on nanobot config directories is in CleanupConfig (line 294), called only from HandleDelete with shouldSkipConfigCleanup guard. The only os.Remove on nanobot config files is in CleanupConfig (line 287) for default path. No other code path deletes nanobot config files or directories.
  implication: No code path actively "clears" the config folder. The user's symptom of "config folder cleared" is most likely caused by the config file being overwritten with a different workspace value, causing the nanobot to look for files in a different directory.

- timestamp: 2026-04-14T00:50:00Z
  checked: cmd/nanobot-auto-updater/main.go OnInstancesChange callback (lines 382-399)
  found: OnInstancesChange calls lifecycle.StopAllNanobots() which kills ALL nanobot processes. This is called when instances config changes via hot reload. However, UpdateConfig sets skipReload=true during writes, and the debounce timer should fire after skipReload is reset to false. Even if the reload fires, it compares old and new configs -- if they're the same, OnInstancesChange is not triggered.
  implication: This is a secondary concern (kills all processes) but doesn't cause config file deletion.

## Resolution

root_cause:
  CloneConfig (config_manager.go:223-269) reads from and writes to the SAME config file when the source instance and the copied instance have the same start_command (which is the default behavior when the user doesn't modify the start_command in the copy dialog). This overwrites the original instance's config.json with:
  1. The copy's port (different from original)
  2. The copy's workspace path (e.g., ~/.nanobot-bot-copy instead of ~/.nanobot-bot)

  After this corruption, when the original nanobot instance restarts (manually or via hot reload), it reads the corrupted config and:
  - Binds to the wrong port
  - Looks for skills in the copy's workspace directory (~/.nanobot-bot-copy/) instead of the original's (~/.nanobot-bot/)
  - Skills appear "lost" because they're still in the original directory but the process looks elsewhere

  The user's symptom of "config文件夹内容被清空，skills文件丢失" is caused by the workspace being silently redirected to a different (empty or non-existent) directory.

  Secondary issue: ParseConfigPath ignores the instanceName parameter, while resolveWorkspace uses it. This asymmetry means two instances with the same --config flag but different names still share the same config FILE, but have different workspace paths in the config content.

fix:
verification:
files_changed: []
