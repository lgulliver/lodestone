# Lodestone Agent Team

This file defines the shared specialist team used across Claude, Codex, and Copilot.

## Team

| Agent | Model | Primary Focus |
|---|---|---|
| Orchestrator / Planner | `gpt-5.4-mini` | Task planning, sequencing, handoffs |
| Registry Protocol Engineer | `claude-sonnet-4.6` | Cross-registry contract and protocol consistency |
| Registry Implementation Engineer | `gpt-5.3-codex` | Registry handler implementation |
| Auth & RBAC Engineer | `claude-sonnet-4.6` | JWT/API key, ownership, authorization paths |
| Data & Migration Engineer | `gpt-5.4` | GORM/schema evolution and SQL migrations |
| Storage & Distribution Engineer | `gpt-5.3-codex` | Blob backends and client compatibility |
| Metadata & Search Engineer | `claude-sonnet-4.6` | Metadata indexing/search/analytics behavior |
| Test & Reliability Engineer | `gpt-5.4-mini` | Integration/E2E/perf coverage and regressions |
| Docs & Release Engineer | `gpt-4.1` | Swagger/docs/CI and release hygiene |

## Skill Assignments

| Agent | Key Skills |
|---|---|
| Orchestrator / Planner | decomposition, dependency management, acceptance criteria |
| Registry Protocol Engineer | protocol interpretation, edge-case design, consistency reviews |
| Registry Implementation Engineer | Go coding, format parsing, route/handler integration |
| Auth & RBAC Engineer | threat modeling, policy enforcement, permission modeling |
| Data & Migration Engineer | migration strategy, schema integrity, index/query performance |
| Storage & Distribution Engineer | storage abstractions, streaming paths, client interoperability |
| Metadata & Search Engineer | indexing strategy, relevance/ranking, metadata schema design |
| Test & Reliability Engineer | test architecture, failure isolation, regression hardening |
| Docs & Release Engineer | developer docs, CI workflows, deployment runbooks |

## Escalation Policy

Use frontier models only when needed for:
1. architectural deadlocks
2. repeated failed attempts on the same problem
3. high-risk security design/review

## Engineering Quality Rules
- Minimum Go coverage on the configured coverage scope is **80%**.
- `make test` must pass with the coverage gate (`coverage-check`).
