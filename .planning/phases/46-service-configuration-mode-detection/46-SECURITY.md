---
phase: 46
slug: service-configuration-mode-detection
status: verified
threats_open: 0
asvs_level: 1
created: 2026-04-10
---

# Phase 46 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| config.yaml -> Config struct | User-provided YAML file parsed by viper, values flow into application | ServiceConfig fields (AutoStart, ServiceName, DisplayName) |
| Process environment -> Service detection | OS session type determines service mode via svc.IsWindowsService() | Boolean flag (read-only, not user-controllable) |
| Config auto_start -> Service registration | auto_start=true triggers service registration intent which requires admin privileges | Boolean flag + service name strings |

---

## Threat Register

| Threat ID | Category | Component | Disposition | Mitigation | Status |
|-----------|----------|-----------|-------------|------------|--------|
| T-46-01 | Tampering | config.yaml service_name field | mitigate | ServiceConfig.Validate() enforces alphanumeric-only regex `^[a-zA-Z0-9]+$` (D-10) and max 256 chars (defense-in-depth for SCM limit) | closed |
| T-46-02 | Tampering | config.yaml service_name injection | mitigate | Compiled regex `regexp.MustCompile("^[a-zA-Z0-9]+$")` prevents special character injection into SCM service name; max length 256 prevents buffer issues | closed |
| T-46-03 | Tampering | config.yaml auto_start field | accept | Config file is local, requires file system access. File permissions managed by OS. No remote attack surface. | closed |
| T-46-04 | Spoofing | Service registration (D-08) | mitigate | Phase 48 will check admin privileges before registration. Phase 46 only logs intent and exits with code 2 — no actual SCM operations performed | closed |

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-46-01 | T-46-03 | config.yaml auto_start field tampering requires local file system access; OS file permissions provide adequate protection for a single-user Windows service application | GSD security audit | 2026-04-10 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-04-10 | 4 | 4 | 0 | GSD security audit (sonnet) |

### Verification Evidence

- **T-46-01 CLOSED**: `internal/config/service.go:9` — `serviceNameRegex = regexp.MustCompile("^[a-zA-Z0-9]+$")` verified; Validate() enforces regex match at line 28 and max 256 chars at line 33
- **T-46-02 CLOSED**: Same regex compilation at package level; all special characters (spaces, hyphens, symbols) rejected; 12/12 test cases pass confirming enforcement
- **T-46-03 CLOSED**: Accepted — config.yaml is a local file, no remote attack surface. `internal/config/service.go` validates all values before use
- **T-46-04 CLOSED**: `cmd/nanobot-auto-updater/main.go:134-139` — Phase 46 only logs intent and calls `os.Exit(2)`; no SCM API calls (`svc/mgr`) exist in any Phase 46 file

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-04-10
