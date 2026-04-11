---
status: partial
phase: 48-service-manager
source: [48-VERIFICATION.md]
started: 2026-04-11T00:00:00+08:00
updated: 2026-04-11T00:00:00+08:00
---

## Current Test

[awaiting human testing]

## Tests

### 1. Service Registration (MGR-02)
expected: Run as admin with auto_start: true, program registers as Windows service via SCM CreateService. Verify with `sc query <service_name>` shows SERVICE_AUTO_START.
result: [pending]

### 2. Idempotent Registration
expected: Run again as admin with auto_start: true, program logs "service already registered, skipping" and exits without error.
result: [pending]

### 3. Service Unregistration (MGR-03)
expected: Set auto_start: false, run from console. Program calls UnregisterService, logs "switched to console mode", continues running. Verify with `sc query <service_name>` shows service removed.
result: [pending]

### 4. Non-Admin Error (MGR-04)
expected: Run without admin privileges with auto_start: true. Program outputs error containing "Run as administrator" and exits with code 1.
result: [pending]

### 5. Recovery Policy
expected: After registration, `sc qfailure <service_name>` shows 3x ServiceRestart at 60 second intervals with 24h reset period.
result: [pending]

### 6. Console Mode Non-Regression
expected: With auto_start: false and no service registered, program starts normally in console mode. Full app startup/shutdown unaffected.
result: [pending]

## Summary

total: 6
passed: 0
issues: 0
pending: 6
skipped: 0
blocked: 0

## Gaps
