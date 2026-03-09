# Feature Research

**Domain:** Multi-instance process management and orchestration
**Researched:** 2026-03-09
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| **Instance Configuration** | Users need to define what instances exist | LOW | YAML array structure with name, port, command fields |
| **Unique Instance Names** | Required for identification and logging | LOW | Validate no duplicates in config, fail fast on startup |
| **Stop All Instances** | Basic orchestration operation | LOW | Iterate through instances, stop each (reuse existing stop logic) |
| **Start All Instances** | Basic orchestration operation | LOW | Iterate through instances, start each with configured command |
| **Failure Notification** | Existing v0.1 feature, users expect it to continue | MEDIUM | Report which instances failed, include instance name in message |
| **Graceful Degradation** | Some instances succeed, some fail - don't abort all | MEDIUM | Continue starting other instances when one fails |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| **Instance Health Status** | Know which instances are running vs failed | MEDIUM | Track state per instance, report in logs and notifications |
| **Configurable Retry Policy** | Auto-retry failed instances | HIGH | Defer to v0.3 - adds significant complexity, needs backoff logic |
| **Individual Instance Control** | Stop/start specific instance by name | MEDIUM | Future consideration for v0.3 - CLI flags for targeting specific instances |
| **Parallel Instance Startup** | Start instances concurrently for faster recovery | HIGH | Defer to v0.3 - requires goroutine management, error aggregation |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| **Rolling Updates (sequential)** | Maintain availability during update | Not applicable - nanobot instances are stateless agents, no traffic to route | All-at-once is simpler and appropriate for this use case |
| **Dependency Ordering** | Start instances in specific order | Adds complexity for minimal benefit in this single-tool context | Parallel start with graceful degradation handles failures adequately |
| **Auto-restart on Crash** | Keep instances running | Conflicts with update orchestration, creates race conditions | Cron-based health checks or external monitoring is better fit |

## Feature Dependencies

```
[Multi-Instance Configuration]
    └──requires──> [Instance Name Validation]
    └──requires──> [Port Detection per Instance]

[Stop All Instances]
    └──requires──> [Instance Configuration]
    └──requires──> [Process Detection by Port] (exists from v0.1)

[Update Operation]
    └──requires──> [Stop All Instances]
    └──requires──> [Update Binary] (exists from v0.1)

[Start All Instances]
    └──requires──> [Update Operation]
    └──requires──> [Graceful Degradation]
    └──requires──> [Failure Notification per Instance]

[Graceful Degradation]
    └──requires──> [Failure Notification per Instance]

[Failure Notification per Instance]
    └──requires──> [Instance Configuration]
    └──enhances──> [Pushover Integration] (exists from v0.1)
```

### Dependency Notes

- **Multi-Instance Configuration requires Instance Name Validation:** Duplicate names would cause ambiguity in logs and notifications. Validate on startup, fail fast with clear error message.

- **Multi-Instance Configuration requires Port Detection per Instance:** Each instance runs on a unique port. Config must specify port per instance for process detection (existing v0.1 logic adapted).

- **Start All Instances requires Graceful Degradation:** If one instance fails to start, continue starting others. Don't abort the entire operation because of a single instance failure.

- **Graceful Degradation requires Failure Notification per Instance:** Users need to know which specific instances failed. Aggregate failures and send single notification with all failed instance names.

- **Failure Notification per Instance enhances Pushover Integration:** Reuse existing Pushover setup from v0.1, extend message format to include instance names.

## MVP Definition

### Launch With (v0.2)

Minimum viable product - what's needed to validate multi-instance management.

- [x] **Instance Configuration (YAML)** - Essential: Define instances as array with name, port, start_command fields
- [x] **Instance Name Validation** - Essential: Detect duplicates on startup, fail fast
- [x] **Stop All Instances** - Essential: Iterate config, stop each by port (reuse v0.1 logic)
- [x] **Start All Instances** - Essential: Iterate config, start each with command, capture errors per instance
- [x] **Graceful Degradation** - Essential: Continue starting other instances when one fails
- [x] **Per-Instance Failure Notification** - Essential: Report which instances failed in Pushover message

### Add After Validation (v0.3)

Features to add once multi-instance is working.

- [ ] **Instance Health Status Tracking** - Track running/stopped/failed state per instance for better logging
- [ ] **Individual Instance Control** - CLI flags like `-instance <name>` to target specific instance
- [ ] **Parallel Instance Startup** - Start instances concurrently for faster recovery after update

### Future Consideration (v2+)

Features to defer until product has more users and feedback.

- [ ] **Configurable Retry Policy** - Auto-retry failed starts with exponential backoff
- [ ] **Instance Groups** - Define groups of instances for partial updates (e.g., "frontend" vs "backend" groups)
- [ ] **Health Check Endpoint** - Expose instance status via HTTP endpoint for external monitoring

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Instance Configuration (YAML) | HIGH | LOW | P1 |
| Instance Name Validation | HIGH | LOW | P1 |
| Stop All Instances | HIGH | LOW | P1 |
| Start All Instances | HIGH | MEDIUM | P1 |
| Graceful Degradation | HIGH | MEDIUM | P1 |
| Per-Instance Failure Notification | HIGH | LOW | P1 |
| Instance Health Status Tracking | MEDIUM | MEDIUM | P2 |
| Individual Instance Control | MEDIUM | MEDIUM | P2 |
| Parallel Instance Startup | LOW | HIGH | P3 |
| Configurable Retry Policy | MEDIUM | HIGH | P3 |

**Priority key:**
- P1: Must have for launch (v0.2)
- P2: Should have, add when possible (v0.3)
- P3: Nice to have, future consideration (v2+)

## Competitor Feature Analysis

| Feature | Docker Compose | Supervisord | Systemd | Our Approach (Nanobot Updater) |
|---------|----------------|-------------|---------|--------------------------------|
| **Configuration Format** | YAML services array | INI [program:x] sections | INI unit files with @ templates | YAML instances array (simple, familiar) |
| **Instance Identification** | Service name (key) | Program name after [program:] | Unit name with @instance | Instance name field in config |
| **Startup Order** | depends_on + healthcheck | priority field | After=, Wants= | Parallel start (no dependencies needed for stateless agents) |
| **Failure Handling** | restart: on-failure | autorestart=true | Restart=on-failure | Continue other instances, notify failures |
| **Status Reporting** | docker-compose ps | supervisorctl status | systemctl status | Logs + Pushover notification |
| **Duplicate Detection** | YAML parser error | Last definition wins | Multiple units allowed | Explicit validation, fail fast |

### Key Insights from Competitor Analysis

1. **Docker Compose Pattern** (HIGH confidence):
   - Services defined as YAML map with service names as keys
   - Healthchecks for startup ordering (`depends_on: condition: service_healthy`)
   - [Source: Docker Docs](https://docs.docker.com/compose/how-tos/startup-order/)

2. **Supervisord Pattern** (MEDIUM confidence):
   - Programs defined as `[program:name]` sections
   - Group multiple programs with `[group:name] programs=prog1,prog2`
   - Last duplicate definition wins (potential silent failure)
   - [Source: Supervisord Docs](https://supervisord.org/configuration.html)

3. **Systemd Template Pattern** (HIGH confidence):
   - Template unit files with `@` symbol (e.g., `service@.service`)
   - Instantiate multiple: `service@1.service`, `service@2.service`
   - Use `%I` specifier in unit file to reference instance identifier
   - [Source: Icinga Blog](https://icinga.com/blog/managing-multiple-service-instances-with-a-systemd-generator/)

4. **Our Approach Rationale**:
   - Use YAML array (not map) for instances - simpler iteration, explicit ordering
   - Validate duplicates explicitly (unlike supervisord's silent override)
   - Skip dependency ordering - nanobot instances are stateless, no interdependencies
   - Graceful degradation with notification - appropriate for background tool management

## Implementation Notes

### Configuration Structure (YAML)

```yaml
# v0.1 single instance (backward compatible)
cron: "0 3 * * *"
pushover_token: ""
pushover_user: ""

# v0.2 multi-instance addition
instances:
  - name: "bot-alpha"
    port: 8080
    start_command: "nanobot serve --port 8080"
  - name: "bot-beta"
    port: 8081
    start_command: "nanobot serve --port 8081 --config beta.yaml"
```

**Rationale:**
- Array (not map) preserves explicit ordering
- Name field required for identification in logs/notifications
- Port required for process detection (existing v0.1 logic)
- Start command per instance allows different configurations

### Graceful Degradation Strategy

**Pattern:** Continue on failure, aggregate errors, report at end

```go
// Pseudocode
failedInstances := []string{}
for _, instance := range instances {
    if err := startInstance(instance); err != nil {
        log.Errorf("Failed to start %s: %v", instance.Name, err)
        failedInstances = append(failedInstances, instance.Name)
    }
}

if len(failedInstances) > 0 {
    notifyFailedInstances(failedInstances)
}
```

**Notification Format:**
```
Nanobot Update Complete - Partial Failure

Failed to start: bot-alpha, bot-gamma
Successfully started: bot-beta

Check logs for details.
```

### Instance Name Validation Rules

1. **Required field** - Empty name rejected
2. **No duplicates** - Case-sensitive comparison
3. **Valid characters** - Alphanumeric, hyphens, underscores (no spaces or special chars)
4. **Length limit** - 1-64 characters (reasonable for logging and display)

## Sources

- [Docker Compose Startup Order](https://docs.docker.com/compose/how-tos/startup-order/) - Healthcheck and dependency patterns (HIGH confidence)
- [Supervisord Configuration](https://supervisord.org/configuration.html) - Multi-program configuration format (MEDIUM confidence - network error during fetch, relied on search snippets)
- [Systemd Template Services](https://icinga.com/blog/managing-multiple-service-instances-with-a-systemd-generator/) - Multi-instance management patterns (HIGH confidence)
- [AWS Well-Architected - Graceful Degradation](https://docs.aws.amazon.com/wellarchitected/latest/reliability-pillar/rel_mitigate_interaction_failure_graceful_degradation.html) - Partial failure handling philosophy (HIGH confidence)
- [Rolling vs All-at-Once Deployments](https://www.harness.io/blog/difference-between-rolling-and-blue-green-deployments) - Deployment strategy comparison (HIGH confidence)
- [YAML Duplicate Key Detection](https://stackoverflow.com/questions/47668308/duplicate-key-in-yaml-configuaration-file) - Validation best practices (HIGH confidence)
- [Server Naming Conventions](https://blog.invgate.com/server-naming-conventions) - Instance identification best practices (MEDIUM confidence)

---
*Feature research for: Multi-instance nanobot management*
*Researched: 2026-03-09*
