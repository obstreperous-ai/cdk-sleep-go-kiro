# Meta-Prompting Patterns for AI-Driven Infrastructure Development

This document captures reusable meta-prompting patterns extracted from the [Event-Driven Sleep Audio Pipeline](./README.md) project. These patterns guided AI agents through the entire development lifecycle, from initial bootstrap to production-ready infrastructure, using strict TDD and issue-driven development.

## What Are Meta-Prompting Patterns?

Meta-prompting patterns are structured instructions that shape how AI agents approach software development tasks. Rather than prompting for a single output, these patterns establish persistent behavioral constraints, quality gates, and workflow rules that apply across an entire project lifecycle.

They matter for agentic IaC development because:

- **Consistency** - Agents produce uniform code quality across dozens of issues without drift
- **Safety** - Constraints prevent common IaC mistakes (deploying untested code, breaking existing resources)
- **Reproducibility** - The same patterns can bootstrap new projects with the same quality bar
- **Auditability** - Every decision traces back to a documented rule, making reviews straightforward

---

## Table of Contents

- [Pattern 1: Agent Persona](#pattern-1-agent-persona)
- [Pattern 2: TDD-First](#pattern-2-tdd-first)
- [Pattern 3: Architecture-as-Source-of-Truth](#pattern-3-architecture-as-source-of-truth)
- [Pattern 4: Issue-Driven Development](#pattern-4-issue-driven-development)
- [Pattern 5: Conventional Commits](#pattern-5-conventional-commits)
- [Pattern 6: Snapshot Stability](#pattern-6-snapshot-stability)
- [Pattern 7: Defense-in-Depth Validation](#pattern-7-defense-in-depth-validation)
- [Combining Patterns: Quick-Start Template](#combining-patterns-quick-start-template)
- [Adapting for Your Project](#adapting-for-your-project)

---

## Pattern 1: Agent Persona

### Description

Define a specialist identity with explicit technical constraints. The persona establishes domain expertise, preferred idioms, and non-negotiable rules that apply to every response.

### Extracted From

[`.github/AGENT_GUIDELINES.md`](./.github/AGENT_GUIDELINES.md):

> You are a Senior AWS CDK Go TDD Specialist. Use clean Go idioms. Write tests first, then minimal code. Always follow strict TDD: write failing test(s) first, then the minimal code to make them pass. Keep ARCHITECTURE.md and its Mermaid diagram perfectly in sync after every change. Prefer L2/L3 constructs. Follow AWS Well-Architected principles. Never deploy until tests + synth succeed locally.

### Why It Works

- Anchors the agent to a specific expertise level and domain
- "Senior" implies knowledge of edge cases, best practices, and trade-offs
- Explicit tool preferences (L2/L3 constructs) prevent low-level over-engineering
- Hard constraints ("never deploy until...") create safety gates

### Template

```markdown
You are a Senior {DOMAIN} {LANGUAGE} {METHODOLOGY} Specialist.
Use clean {LANGUAGE} idioms.
{METHODOLOGY_RULES}
Keep {DOCUMENTATION_FILE} perfectly in sync after every change.
Prefer {PREFERRED_ABSTRACTIONS}.
Follow {FRAMEWORK_PRINCIPLES}.
Never {HARD_CONSTRAINT}.
```

### Adaptation Examples

**Terraform Python project:**
```
You are a Senior Terraform Python TDD Specialist. Use clean HCL idioms for infrastructure and Pythonic patterns for tooling. Write tests (Terratest) first, then minimal HCL. Keep DESIGN.md in sync after every change. Prefer modules over raw resources. Follow HashiCorp best practices. Never apply until plan + tests succeed locally.
```

**Kubernetes TypeScript project:**
```
You are a Senior Kubernetes TypeScript Specialist. Use clean TypeScript idioms. Write unit tests (Jest) first, then minimal implementation. Keep docs/architecture.md in sync after every change. Prefer Helm charts over raw manifests. Follow the Twelve-Factor App methodology. Never deploy until lint + test + build succeed locally.
```

---

## Pattern 2: TDD-First

### Description

Enforce the red-green-refactor cycle at every layer of the application. The agent must write a failing test before any implementation code, then write only enough code to make the test pass.

### Extracted From

The project's development workflow (visible across all 11 completed tasks):

1. Write a failing test that asserts the desired behavior
2. Run it to confirm it fails for the right reason
3. Implement the minimal code to make it pass
4. Refactor while keeping tests green
5. Repeat for each new behavior

### Why It Works

- Prevents over-engineering (only code that satisfies a test gets written)
- Creates an executable specification of the system
- Catches regressions immediately
- Forces the agent to think about interfaces before implementations

### Template

```markdown
Always follow strict TDD:
1. Write failing test(s) first that define the expected behavior
2. Run tests to confirm they fail for the expected reason
3. Write the minimal code to make them pass
4. Refactor while keeping all tests green
5. Never skip ahead to implementation without a covering test

TDD applies at every layer:
- {INFRA_LAYER}: {INFRA_TEST_APPROACH}
- {APP_LAYER}: {APP_TEST_APPROACH}
- {INTEGRATION_LAYER}: {INTEGRATION_TEST_APPROACH}
- {REGRESSION_LAYER}: {REGRESSION_TEST_APPROACH}
```

### Adaptation Example

```markdown
Always follow strict TDD:
1. Write failing test(s) first that define the expected behavior
2. Run tests to confirm they fail for the expected reason
3. Write the minimal code to make them pass
4. Refactor while keeping all tests green
5. Never skip ahead to implementation without a covering test

TDD applies at every layer:
- Infrastructure (CDK assertions): Assert CloudFormation resources and properties exist
- Lambda handlers (unit tests): Define behavior with mock clients before implementation
- Integration (E2E tests): Validate cross-resource wiring before connections are made
- Regression (snapshot tests): Lock infrastructure state with golden file comparison
```

---

## Pattern 3: Architecture-as-Source-of-Truth

### Description

Designate a single documentation file as the authoritative design reference. The agent must update this file alongside (or before) any infrastructure change, ensuring documentation never drifts from reality.

### Extracted From

[`CONTRIBUTING.md`](./CONTRIBUTING.md):

> `ARCHITECTURE.md` is the single source of truth for the Event-Driven Sleep Audio Pipeline design. Any change that adds, removes, or modifies AWS resources, data flow steps, security controls, or observability components must include a corresponding update to `ARCHITECTURE.md` and its Mermaid diagram in the same commit or pull request.

### Why It Works

- Eliminates stale documentation (the rule makes updates non-optional)
- Mermaid diagrams render in GitHub, providing always-current visual architecture
- Reviewers can verify architecture consistency without reading all the code
- Forces the agent to reason about design impact before implementing

### Template

```markdown
{ARCHITECTURE_FILE} is the single source of truth for the {PROJECT} design.

Before making any change that affects {CHANGE_SCOPE}, read {ARCHITECTURE_FILE} to understand the current design.

Any change that adds, removes, or modifies {CHANGE_CATEGORIES} must include a corresponding update to {ARCHITECTURE_FILE} and its Mermaid diagram in the same commit.

If you are unsure whether your change affects the architecture, err on the side of updating the document.
```

### Key Elements

- The file must contain a **Mermaid diagram** that is machine-renderable
- The rule must specify **what triggers an update** (resource changes, data flow changes, security changes)
- The update must happen **in the same commit** (not as a follow-up)

---

## Pattern 4: Issue-Driven Development

### Description

Every change must originate from a tracked issue with defined scope. No work happens without a corresponding issue, and each issue maps to exactly one focused pull request.

### Extracted From

The project's task structure (`.agents/tasks/` directory with 11 completed task cycles):

- Each task has a defined scope (`task.json`)
- Features within tasks have explicit acceptance criteria (`features/FEAT-*.json`)
- Work products are reviewed against criteria before completion
- No scope creep: one concern per issue

### Why It Works

- Creates an audit trail of every design decision
- Prevents scope creep (the agent cannot expand beyond the issue)
- Makes code review focused and manageable
- Enables parallel work streams without conflicts

### Template

```markdown
Every change must follow issue-driven development:
1. Create an issue describing the desired outcome and acceptance criteria
2. Scope the issue to a single concern (do not combine unrelated changes)
3. Implement only what the issue describes (no scope creep)
4. Reference the issue in the commit/PR
5. Close the issue only when all acceptance criteria are met

Issue structure:
- Title: {TYPE}: {DESCRIPTION}
- Body: Background, acceptance criteria, out of scope
- Labels: {LABEL_TAXONOMY}
```

---

## Pattern 5: Conventional Commits

### Description

All commit messages follow a structured format that enables automated changelog generation, semantic versioning, and clear project history.

### Extracted From

[`CONTRIBUTING.md`](./CONTRIBUTING.md):

> All commit messages must follow the Conventional Commits format:
> - `feat:` - new feature
> - `fix:` - bug fix
> - `chore:` - maintenance tasks
> - `docs:` - documentation changes
> - `refactor:` - code restructuring without behavior change

### Why It Works

- Agents produce consistent, parseable commit history
- Enables automated tooling (changelogs, version bumps)
- Commit type signals review priority (feat needs more scrutiny than docs)
- Scope forces the agent to categorize its work accurately

### Template

```markdown
All commit messages must follow Conventional Commits:

Format: {TYPE}({OPTIONAL_SCOPE}): {DESCRIPTION}

Types:
- feat: New feature or capability
- fix: Bug fix
- chore: Maintenance (dependencies, config, tooling)
- docs: Documentation only
- refactor: Code restructuring without behavior change
- test: Adding or updating tests

Rules:
- Subject line under 72 characters
- Use imperative mood ("add" not "added")
- Reference the issue number when applicable
```

---

## Pattern 6: Snapshot Stability

### Description

Use golden file comparison (snapshot testing) to detect unintended infrastructure drift. A canonical CloudFormation/Terraform output is stored in version control and compared against the current synthesis output on every test run.

### Extracted From

The project's `TestStackSnapshotStability` test and snapshot workflow:

- `testdata/snapshot.json` stores the canonical CloudFormation template
- The test synthesizes the current stack and compares byte-for-byte against the golden file
- Any unintended change causes a test failure
- Intentional changes require explicit snapshot regeneration and review

### Why It Works

- Catches unintended side effects from CDK version upgrades, feature flag changes, or refactoring
- Forces explicit acknowledgment of infrastructure changes
- Works as a regression gate in CI (no unreviewed changes pass)
- The golden file serves as a readable infrastructure specification

### Template

```markdown
Snapshot stability rules:
1. The canonical output is stored at {SNAPSHOT_PATH}
2. On every test run, synthesize the current state and compare against the snapshot
3. If the comparison fails, the test fails (no auto-update in CI)
4. To update the snapshot intentionally:
   a. Delete {SNAPSHOT_PATH}
   b. Run {REGENERATION_COMMAND}
   c. Review the regenerated output for correctness
   d. Commit the updated snapshot alongside the code change
5. Never commit a snapshot update without reviewing the diff
```

---

## Pattern 7: Defense-in-Depth Validation

### Description

Validate inputs and constraints at multiple layers so that failures are caught at the cheapest possible point while still maintaining comprehensive coverage deeper in the stack.

### Extracted From

The project's dual-validation architecture:

- **Layer 1 (Step Functions Choice state):** Validates file extension before invoking any compute. Cost: near-zero (no Lambda invocation).
- **Layer 2 (Lambda handler):** Validates required fields, file extension (again), and performs business logic checks. Cost: Lambda invocation duration.

### Why It Works

- Cheap checks run first, saving compute costs on obviously invalid inputs
- Deeper checks catch edge cases that simpler rules miss
- No single layer is a single point of failure for validation
- Each layer can evolve independently

### Template

```markdown
Validate at multiple layers (defense-in-depth):

Layer 1 - {CHEAPEST_LAYER} (cost: {COST}):
  Validates: {BASIC_CHECKS}
  On failure: {FAST_FAIL_ACTION}

Layer 2 - {COMPUTE_LAYER} (cost: {COST}):
  Validates: {COMPREHENSIVE_CHECKS}
  On failure: {DETAILED_ERROR_ACTION}

Layer 3 - {STORAGE_LAYER} (cost: {COST}):
  Validates: {INTEGRITY_CHECKS}
  On failure: {RECOVERY_ACTION}

Principles:
- Cheapest validation runs first
- Each layer validates independently (do not assume upstream validated)
- Error messages include which layer caught the issue
- All validation failures are logged for observability
```

---

## Combining Patterns: Quick-Start Template

The following template combines all seven patterns into a single reusable `.github/AGENT_GUIDELINES.md` that you can adapt for any new IaC project:

```markdown
# Agent Guidelines

## Persona

You are a Senior {DOMAIN} {LANGUAGE} {METHODOLOGY} Specialist.
Use clean {LANGUAGE} idioms. Write tests first, then minimal code.

## TDD Rules

Always follow strict TDD:
1. Write failing test(s) first, then the minimal code to make them pass.
2. Apply TDD at every layer: infrastructure assertions, unit tests, integration tests, and snapshot tests.
3. Never skip ahead to implementation without a covering test.

## Architecture

{ARCHITECTURE_FILE} is the single source of truth for the project design.
Keep {ARCHITECTURE_FILE} and its Mermaid diagram perfectly in sync after every change.
If unsure whether a change affects architecture, update the document.

## Development Workflow

- Every change starts as a tracked issue with defined scope and acceptance criteria.
- One concern per issue. No scope creep.
- Use Conventional Commits: feat/fix/chore/docs/refactor/test.
- Reference the issue in commit messages.

## Quality Gates

- Prefer {PREFERRED_ABSTRACTIONS} over low-level primitives.
- Follow {FRAMEWORK_PRINCIPLES}.
- Never deploy until tests + synth/plan succeed locally.
- Snapshot test must pass (or be explicitly regenerated and reviewed).
- Validate at multiple layers: cheapest checks first, comprehensive checks deeper.

## Hard Constraints

- Never deploy untested infrastructure.
- Never commit a snapshot update without reviewing the diff.
- Never expand scope beyond the current issue.
- Never modify {ARCHITECTURE_FILE} without updating the Mermaid diagram.
```

---

## Adapting for Your Project

### Step 1: Define Your Persona

Identify the specialist role, language, and methodology for your project:

| Project Type | Example Persona |
|---|---|
| AWS CDK (Go) | Senior AWS CDK Go TDD Specialist |
| Terraform (HCL) | Senior Terraform Infrastructure TDD Specialist |
| Pulumi (TypeScript) | Senior Pulumi TypeScript Specialist |
| Kubernetes (Helm) | Senior Kubernetes Platform Engineer |
| Serverless Framework | Senior Serverless Node.js Specialist |

### Step 2: Choose Your Test Layers

Map test types to your IaC framework:

| Framework | Assertion Tests | Snapshot Tests | Integration Tests |
|---|---|---|---|
| AWS CDK | `assertions.Match` | `template.ToJSON()` comparison | Deploy + validate |
| Terraform | Terratest | `terraform plan -out` comparison | `terraform apply` + validate |
| Pulumi | Unit tests with mocks | `pulumi preview` comparison | `pulumi up` + validate |
| CloudFormation | cfn-lint + taskcat | Template diff | Stack deploy + validate |

### Step 3: Establish Your Architecture Document

Create your architecture-as-source-of-truth file:

1. Choose a location (e.g., `ARCHITECTURE.md`, `docs/design.md`)
2. Include a Mermaid diagram of the current system
3. Add the "must update in same commit" rule to your contributing guide
4. Define what constitutes an architectural change for your project

### Step 4: Set Up Snapshot Testing

1. Choose your snapshot format (CloudFormation JSON, Terraform plan, Pulumi state)
2. Store the golden file in version control
3. Write a test that compares current output against the golden file
4. Document the regeneration procedure in your contributing guide

### Step 5: Combine and Deploy

1. Copy the [Quick-Start Template](#combining-patterns-quick-start-template) above
2. Fill in your project-specific values
3. Save to `.github/AGENT_GUIDELINES.md` (or your preferred location)
4. Reference it in your project README and contributing guide
5. Iterate as you discover project-specific patterns

---

## References

- [README.md](./README.md) - Project overview and getting started
- [ARCHITECTURE.md](./ARCHITECTURE.md) - Full architecture documentation
- [CONTRIBUTING.md](./CONTRIBUTING.md) - Development setup and guidelines
- [SUMMARY.md](./SUMMARY.md) - Experiment notes and lessons learned
- [`.github/AGENT_GUIDELINES.md`](./.github/AGENT_GUIDELINES.md) - The actual agent prompt used in this project
