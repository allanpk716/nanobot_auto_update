# Feature Research

**Domain:** HTTP API Service + Monitoring Service
**Researched:** 2026-03-16
**Confidence:** MEDIUM

## Feature Landscape

### Table Stakes (Users Expect These)

Features users assume exist. Missing these = product feels incomplete.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Bearer Token Authentication | Standard auth for HTTP APIs. Users expect secure, token-based access control. | LOW | Single static token in Authorization header. Simple comparison, no JWT complexity needed. |
| JSON Response Format | APIs must return structured data. JSON is the de facto standard. | LOW | Use standard Go encoding/json. Include status field + message/data structure. |
| HTTP Status Codes (2xx, 4xx, 5xx) | Proper status codes communicate success/failure clearly to clients. | LOW | 200 for success, 401 for auth failure, 500 for server errors. Use standard net/http codes. |
| Monitoring Service Runs Continuously | Always-on background service is core expectation for monitoring. | LOW | Use Go goroutine + time.Ticker. 15-minute intervals, runs forever until stopped. |
| HTTP GET Health Checks | Standard pattern for connectivity monitoring. Simple, reliable, universally understood. | LOW | http.Client with timeout. Check response status code only (no body parsing needed). |
| Failure Notifications | Users expect to be notified when monitoring detects problems. | LOW | Reuse existing Pushover integration. Send alert on connectivity failure. |
| Service Logging | All services must log operations for debugging and audit. | LOW | Already using WQGroup/logger. Add component-level logging (monitor/api). |
| Configuration from YAML | Existing pattern. Users expect to configure in config.yaml. | LOW | Extend existing config structure with new sections (api, monitoring). |

### Differentiators (Competitive Advantage)

Features that set the product apart. Not required, but valuable.

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Recovery Notifications | Alert when connectivity is restored. Provides complete incident lifecycle visibility. | LOW | Track previous state. Send notification on failure→success transition. |
| Structured JSON Error Responses | Include error code + human message. Easier for clients to parse and display errors. | MEDIUM | Define error codes enum. Include `error_code`, `message`, `details` fields. |
| Request/Response Logging | Log API requests and responses for audit trail. Helps debugging integration issues. | MEDIUM | Log method, path, status code, duration. Be careful with log volume. |
| Configurable Monitoring Interval | Allow users to customize check frequency (default 15 min). Flexibility for different use cases. | LOW | Add `monitoring.interval` to config. Use time.Duration parsing. |
| Configurable Monitoring Target | Allow users to change what URL to monitor. Some may prefer different endpoints. | LOW | Add `monitoring.target_url` to config. Default to https://www.google.com. |
| Graceful Service Shutdown | Handle Ctrl+C cleanly. Stop monitoring, finish in-flight API requests. | MEDIUM | Use context.Context + signal.Notify. Implement shutdown timeout. |
| Health Check Endpoint | Expose /health for the API service itself. Allows external monitoring. | LOW | GET /health returns 200 + JSON status. No auth required. |

### Anti-Features (Commonly Requested, Often Problematic)

Features that seem good but create problems.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| JWT Token Authentication | "More secure" than static token. Industry standard for user auth. | Overkill for single-user internal service. Adds complexity (token rotation, expiration, signing keys). | Static Bearer token in config. Simple, sufficient for internal tool. |
| Multiple Monitoring Targets | Monitor multiple URLs simultaneously. Redundancy improves reliability. | Increases complexity exponentially (state management per target, notification grouping, error aggregation). Already covered by existing multi-instance pattern. | Single target per service instance. Use separate instances for different targets if needed. |
| Monitoring Response Body Content | "Verify the response contains expected data." More thorough than status code check. | Creates fragility. Content changes, formatting differences, false positives. Google homepage content varies by region/user agent. | Check HTTP status code only (200 OK). Simple, reliable, sufficient for connectivity test. |
| Complex Alert Thresholds | Alert after N consecutive failures. Avoid false positives from transient issues. | Adds state machine complexity. Delays real alerts. Transient failures are rare for Google connectivity. | Alert on first failure. Immediate notification. Fast response time is priority. |
| Rate Limiting on API | Prevent abuse. Protect service from overload. | Unnecessary for internal single-user service. Adds complexity (token bucket, client tracking). | Trust internal network. If needed, add simple request timeout. |
| Database for Monitoring History | Store historical data for analysis, trending, reporting. | Massive scope creep. Requires DB, schema, migrations, retention policy. Out of project scope. | Logs provide recent history. Keep service stateless. No persistent data. |
| API Versioning (/api/v2/) | Future-proof for API changes. Standard practice for public APIs. | Over-engineering for internal tool. Single endpoint, unlikely to change significantly. | Simple /api/v1/ prefix. Sufficient for current scope. Can version later if truly needed. |
| Retry Logic for HTTP Requests | "Make monitoring more resilient to transient failures." | Hides real connectivity issues. Delays failure detection. Defeats purpose of monitoring. | Single request with timeout. Fail fast, alert immediately. |
| Circuit Breaker Pattern | "Prevent cascading failures when Google is down." | Unnecessary for single-purpose monitoring. Adds complexity (state machine, cooldown period). | Simple check + alert pattern. Let monitoring fail and notify. No downstream dependencies to protect. |
| Real-time Web Dashboard | "Visual interface to see monitoring status." | Massive scope creep. Requires frontend, WebSocket, state management. | Logs + JSON API responses. Keep it simple. Use existing log files. |

## Feature Dependencies

```
HTTP API Service
    └──requires──> Bearer Token Authentication
    └──requires──> JSON Response Format
    └──requires──> Configuration (YAML)
    └──requires──> Logging

Monitoring Service
    └──requires──> HTTP GET Health Checks
    └──requires──> Configuration (YAML)
    └──requires──> Logging
    └──requires──> Pushover Integration (for notifications)

Recovery Notifications
    └──requires──> Monitoring Service (must track state)
    └──requires──> Pushover Integration

Graceful Service Shutdown
    └──requires──> HTTP API Service (must stop accepting requests)
    └──requires──> Monitoring Service (must stop goroutine)

Health Check Endpoint (/health)
    └──enhances──> HTTP API Service (allows external monitoring)

Request/Response Logging
    └──enhances──> HTTP API Service (improves debugging)
```

### Dependency Notes

- **HTTP API requires Bearer Token Authentication:** Security baseline. API cannot be exposed without access control.
- **HTTP API requires JSON Response Format:** Structured responses are expected by all API clients. Alternative (plain text) would break integration.
- **Monitoring Service requires Pushover Integration:** Notifications are core value. Without them, monitoring is just silent logging.
- **Recovery Notifications requires Monitoring Service:** Must track previous state (failed vs healthy) to detect recovery transition.
- **Health Check Endpoint enhances HTTP API Service:** Nice-to-have for operational visibility, but API works without it.
- **Request/Response Logging enhances HTTP API Service:** Improves debugging but increases log volume. Optional optimization.

## MVP Definition

### Launch With (v0.3)

Minimum viable product — what's needed to validate the architecture change from cron to HTTP API + monitoring.

- [x] Bearer Token Authentication — Essential for API security. Non-negotiable.
- [x] JSON Response Format — Standard API expectation. Required for client integration.
- [x] HTTP Status Codes — Basic HTTP compliance. Cannot skip.
- [x] Monitoring Service Runs Continuously — Core architecture change. Replaces cron.
- [x] HTTP GET Health Checks (Google) — Primary monitoring feature. Cannot skip.
- [x] Failure Notifications — Core value proposition. Must alert on connectivity issues.
- [x] Service Logging — Required for debugging and operations.
- [x] Configuration from YAML — Consistency with existing pattern.
- [x] POST /api/v1/trigger-update Endpoint — Primary API feature. Triggers nanobot update.

### Add After Validation (v0.3.x)

Features to add once core is working in production.

- [ ] Recovery Notifications — Trigger: Users ask "how do I know when it's fixed?" HIGH value, LOW complexity.
- [ ] Graceful Service Shutdown — Trigger: Need to restart service cleanly. Improves ops experience.
- [ ] Configurable Monitoring Interval — Trigger: Users want faster/slower checks. Common customization request.
- [ ] Health Check Endpoint (/health) — Trigger: External monitoring systems need to check service health.

### Future Consideration (v0.4+)

Features to defer until architecture is proven and stable.

- [ ] Structured JSON Error Responses — Better DX, but requires error code taxonomy.
- [ ] Request/Response Logging — Audit trail, but increases log volume significantly.
- [ ] Configurable Monitoring Target — Flexibility, but most users will use default.

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Bearer Token Authentication | HIGH | LOW | P1 |
| JSON Response Format | HIGH | LOW | P1 |
| HTTP Status Codes | HIGH | LOW | P1 |
| Monitoring Service (continuous) | HIGH | LOW | P1 |
| HTTP GET Health Checks | HIGH | LOW | P1 |
| Failure Notifications | HIGH | LOW | P1 |
| POST /api/v1/trigger-update | HIGH | LOW | P1 |
| Service Logging | HIGH | LOW | P1 |
| Configuration from YAML | HIGH | LOW | P1 |
| Recovery Notifications | HIGH | LOW | P2 |
| Graceful Service Shutdown | MEDIUM | MEDIUM | P2 |
| Health Check Endpoint | MEDIUM | LOW | P2 |
| Configurable Monitoring Interval | MEDIUM | LOW | P2 |
| Structured JSON Error Responses | MEDIUM | MEDIUM | P3 |
| Request/Response Logging | LOW | MEDIUM | P3 |
| Configurable Monitoring Target | LOW | LOW | P3 |

**Priority key:**
- P1: Must have for launch (v0.3)
- P2: Should have, add when possible (v0.3.x)
- P3: Nice to have, future consideration (v0.4+)

## Competitor Feature Analysis

| Feature | Datadog Synthetics | New Relic Synthetics | SigNoz | Our Approach |
|---------|-------------------|----------------------|--------|--------------|
| Monitoring Type | Multi-location synthetic checks | Synthetic API tests | Full-stack APM with metrics | Single-location HTTP GET |
| Authentication | OAuth, API keys, mTLS | API keys, OAuth | OpenTelemetry tokens | Static Bearer token |
| Alerting | Multi-channel, AI-powered | Threshold-based, integrations | Alert rules, silences | Pushover notifications only |
| Response Analysis | Body, headers, timing metrics | Body validation, assertions | Metrics + traces + logs | Status code only |
| Complexity | HIGH (enterprise SaaS) | HIGH (enterprise SaaS) | HIGH (full observability) | LOW (single-purpose tool) |
| Pricing | Usage-based, expensive | Usage-based, expensive | Free tier + paid | N/A (self-hosted, free) |

**Our Differentiation:**
- Extreme simplicity: Single endpoint, single monitoring target, single notification channel
- No external dependencies: No database, no SaaS subscription, no vendor lock-in
- Tight scope: Does one thing well (connectivity monitoring + update triggering)
- Low resource usage: Designed for lightweight Windows background service

## Sources

**API Monitoring Best Practices:**
- [SigNoz: The Ultimate Guide to API Monitoring in 2026](https://signoz.io/blog/api-monitoring-complete-guide/) — MEDIUM confidence (verified multiple sources agree on JSON, status codes, auth patterns)
- [Dotcom-Monitor: API Monitoring Metrics, Best Practices](https://www.dotcom-monitor.com/blog/api-monitoring/) — MEDIUM confidence (industry standard practices)

**Monitoring Anti-Patterns:**
- [Netdata: Monitor Everything is an Anti-Pattern!](https://www.netdata.cloud/blog/monitor-everything-is-an-anti-pattern/) — HIGH confidence (authoritative source, detailed architectural analysis)
- [AWS DevOps Guidance: Anti-patterns for Continuous Monitoring](https://docs.aws.amazon.com/wellarchitected/latest/devops-guidance/anti-patterns-for-continuous-monitoring.html) — HIGH confidence (official AWS documentation)

**Alert Fatigue Research:**
- [OneUptime: Alert Fatigue Is Killing Your On-Call Team](https://oneuptime.com/blog/post/2026-03-05-alert-fatigue-ai-on-call/view) — MEDIUM confidence (industry trend analysis)
- [LogicMonitor: 2026 Observability & AI Trends](https://www.logicmonitor.com/blog/observability-ai-trends-2026) — MEDIUM confidence (survey data, 36% alert fatigue statistic)

**HTTP Status Codes:**
- [Dev.to: The Ultimate Guide to HTTP Status Codes in REST APIs](https://dev.to/gianfcop98/the-ultimate-guide-to-http-status-codes-in-rest-apis-40cp) — MEDIUM confidence (verified against official HTTP spec)
- [Postman: What are HTTP status codes?](https://blog.postman.com/what-are-http-status-codes/) — HIGH confidence (authoritative API tool vendor)

**Golang HTTP API Patterns:**
- [Encore.dev: How to Build a REST API with Go in 2026](https://encore.dev/articles/build-rest-api-go-2026) — MEDIUM confidence (current best practices)
- [JetBrains Guide: Authentication for Go Applications](https://www.jetbrains.com/guide/go/tutorials/authentication-for-go-apps/auth/) — HIGH confidence (official JetBrains documentation)

**Confidence Assessment:**
- Stack research: MEDIUM (relied on web search + industry sources, verified key claims across multiple sources)
- Features landscape: MEDIUM (based on general API monitoring domain knowledge, specific to our narrow use case)
- Anti-patterns: HIGH (strong consensus across multiple authoritative sources)

---

*Feature research for: HTTP API Service + Monitoring Service*
*Researched: 2026-03-16*
