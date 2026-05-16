# Claude Team Configuration

This repository uses specialist agent definitions under `.claude/agents/`.

## Default Model Policy
- Use non-frontier models by default.
- Use frontier models only for explicit escalation:
  - architectural deadlock
  - repeated failed implementation attempts
  - high-risk security review

## Agent Roster

1. **orchestrator-planner** (`gpt-5.4-mini`)
2. **registry-protocol-engineer** (`claude-sonnet-4.6`)
3. **registry-implementation-engineer** (`gpt-5.3-codex`)
4. **auth-rbac-engineer** (`claude-sonnet-4.6`)
5. **data-migration-engineer** (`gpt-5.4`)
6. **storage-distribution-engineer** (`gpt-5.3-codex`)
7. **metadata-search-engineer** (`claude-sonnet-4.6`)
8. **test-reliability-engineer** (`gpt-5.4-mini`)
9. **docs-release-engineer** (`gpt-4.1`)

## Recommended Skills by Agent

- **orchestrator-planner**: task decomposition, dependency mapping, risk triage.
- **registry-protocol-engineer**: protocol semantics, compatibility analysis, API contract design.
- **registry-implementation-engineer**: Go implementation, handler wiring, registry format parsing.
- **auth-rbac-engineer**: auth threat modeling, authorization policy, ownership rules.
- **data-migration-engineer**: schema design, migration safety, query/index tuning.
- **storage-distribution-engineer**: blob storage design, distribution workflows, artifact lifecycle.
- **metadata-search-engineer**: metadata normalization, search relevance, analytics semantics.
- **test-reliability-engineer**: integration/e2e strategy, regression prevention, failure reproduction.
- **docs-release-engineer**: API docs quality, CI/release workflows, operational runbooks.

## Quality Gate
- Maintain a minimum of **80% coverage on the configured coverage scope**.
- Use `make test` (which runs `coverage-check`) to enforce the threshold.
