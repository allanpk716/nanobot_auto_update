---
status: awaiting_human_verify
trigger: "instance-copy-config-corruption: Two bugs: (1) Deleting a copied instance stops all running instances and corrupts original config. (2) Saving config creates directory based on instance description name instead of preserving original path."
created: 2026-04-13T00:00:00Z
updated: 2026-04-13T00:50:00Z
---

## Current Focus
hypothesis: Fix implemented and tests passing. Verify completeness.
test: go build and go test pass for all affected packages
expecting: All tests pass, no regressions
next_action: Verify fix completeness, then request human verification

## Symptoms
expected: Bug 1: Copy+delete should not affect other instances. Bug 2: Config save should preserve original directory
actual: Bug 1: Delete copied instance stops all instances and corrupts original config. Bug 2: Config save creates new directory based on instance description name
errors: No error messages, logic bugs
reproduction: Bug 1: Copy instance -> start multiple -> delete copy -> all stop, original config corrupted. Bug 2: Create default gateway instance -> name it -> edit -> save -> new folder created
started: After recent code changes

## Eliminated
- hypothesis: Bug 2 is caused by showEditDialog (instance config edit) rather than showNanobotConfigDialog (nanobot config edit)
  evidence: showEditDialog edits instance metadata (name/port/command/timeout), not nanobot config.json. The nanobot config editing is done by showNanobotConfigDialog which calls PUT /api/v1/instances/{name}/nanobot-config. The user likely meant the "config" button or confused the buttons.
  timestamp: 2026-04-13T00:15:00Z

## Evidence
- timestamp: 2026-04-13T00:05:00Z
  checked: internal/api/instance_config_handler.go HandleDelete (lines 359-409)
  found: HandleDelete calls lifecycle.StopAllNanobots() which kills ALL nanobot.exe processes system-wide after deleting instance config. The code even has a comment acknowledging this: "Known limitation: StopAllNanobots stops ALL nanobot.exe processes system-wide, not just the deleted instance."
  implication: This explains Bug 1 symptom where all instances stop when deleting any instance.

- timestamp: 2026-04-13T00:06:00Z
  checked: internal/nanobot/config_manager.go CleanupConfig (lines 275-300)
  found: CleanupConfig resolves configPath via ParseConfigPath(startCommand, instanceName). For default gateway instances (no --config flag), this ALWAYS returns ~/.nanobot/config.json regardless of instanceName. When the copied instance uses the same start_command (cloned), both resolve to the same path. CleanupConfig checks if path equals default path and only removes config.json file. BUT this means deleting the copy deletes the ORIGINAL's config file too!
  implication: This is the second part of Bug 1 - the original instance's config.json gets deleted because both source and copy share the same default path.

- timestamp: 2026-04-13T00:07:00Z
  checked: internal/nanobot/config_manager.go ParseConfigPath (lines 50-78)
  found: ParseConfigPath has a regex to extract --config from startCommand. If no --config is found, it falls back to ~/.nanobot/config.json. The instanceName parameter is NEVER used in the fallback path. This means ALL default gateway instances share the same config path regardless of name.
  implication: For Bug 2, when editing and saving nanobot config, the HandlePut in nanobot_config_handler.go calls ParseConfigPath(ic.StartCommand, ic.Name) which for default gateway always returns ~/.nanobot/config.json. Bug 2 about creating a new folder with instance description name seems to be related to a different scenario -- need to verify.

- timestamp: 2026-04-13T00:08:00Z
  checked: internal/nanobot/config_manager.go WriteConfig (lines 166-187)
  found: WriteConfig calls os.MkdirAll(filepath.Dir(configPath), 0755) before writing. If configPath somehow resolves to a new path based on instance name, the directory would be created. But for the default gateway case, the path is always ~/.nanobot/config.json.
  implication: Bug 2 might be triggered when a default gateway instance is COPIED and then edited. The copy operation in CloneConfig updates agents.defaults.workspace to ~/.nanobot-{targetName} in the config JSON content. But the file is still written to the correct path.

- timestamp: 2026-04-13T00:15:00Z
  checked: git log and git diff 118aff8..7b93dd0
  found: Commit 7b93dd0 changed ParseConfigPath fallback from ~/.nanobot-{instanceName}/config.json to ~/.nanobot/config.json. This was the Bug 2 fix. BEFORE this fix, the fallback path included instanceName, causing the wrong directory to be created. The fix is already committed to master.
  implication: Bug 2 path resolution is now correct in code. The user may not have deployed the fix yet, or there are residual orphaned directories from before the fix.

- timestamp: 2026-04-13T00:18:00Z
  checked: HandleDelete flow for default gateway instances sharing config
  found: When instance A (default gateway) and its copy A-copy (same start_command) both exist: (1) Delete A-copy -> CleanupConfig("nanobot gateway", "A-copy") -> ParseConfigPath returns ~/.nanobot/config.json -> os.Remove removes the file (2) This destroys A's config too (3) Then StopAllNanobots kills all running instances including A
  implication: Bug 1 is definitively caused by two issues: shared config path destruction + StopAllNanobots

## Resolution
root_cause:
  Bug 1a: HandleDelete unconditionally called lifecycle.StopAllNanobots(), killing ALL nanobot processes on every instance deletion
  Bug 1b: CleanupConfig removed the shared default config file (~/.nanobot/config.json) when deleting ANY default gateway instance, because all default gateway instances resolve to the same config path
  Bug 2: FIXED in commit 7b93dd0 (ParseConfigPath fallback now correctly returns ~/.nanobot/config.json)
fix:
  Bug 1a: Replaced StopAllNanobots with targeted instance stop via onStopInstance callback.
    - Added onStopInstance callback field to InstanceConfigHandler
    - Added SetOnStopInstance setter method
    - In HandleDelete, calls onStopInstance to stop only the deleted instance by PID
    - In server.go, wired up the callback to use InstanceManager.GetLifecycle + StopForUpdate
    - Removed lifecycle.StopAllNanobots call and lifecycle import
  Bug 1b: Added shouldSkipConfigCleanup method that checks if remaining instances share the same config path.
    - In HandleDelete, calls shouldSkipConfigCleanup before onDeleteInstance
    - If another instance resolves to the same path, cleanup is skipped entirely
verification: go build ./... passes. go test ./internal/api/... ./internal/nanobot/... ./internal/instance/... all pass.
files_changed: [internal/api/instance_config_handler.go, internal/api/server.go]
